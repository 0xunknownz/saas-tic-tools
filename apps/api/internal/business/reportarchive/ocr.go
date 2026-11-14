package reportarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	paddleocr "github.com/PaddlePaddle/PaddleOCR/api_sdk/go"
)

type ocrResult struct {
	Provider string
	Model    string
	Pages    []OCRPage
	Raw      json.RawMessage
}

type ocrFailure struct {
	Code      string
	Message   string
	Retryable bool
	Err       error
}

func (e *ocrFailure) Error() string { return e.Code + ": " + e.Message }

type ocrProvider interface {
	Extract(ctx context.Context, fileName string, data []byte, pageHint int) (ocrResult, error)
}

type disabledOCR struct{}

func (disabledOCR) Extract(context.Context, string, []byte, int) (ocrResult, error) {
	return ocrResult{}, &ocrFailure{Code: "OCR_NOT_CONFIGURED", Message: "云端 OCR 尚未配置，请联系管理员", Retryable: false}
}

func newOCRProvider(cfg Config) ocrProvider {
	switch cfg.OCRProvider {
	case "ocrspace":
		return &ocrSpaceProvider{endpoint: cfg.OCREndpoint, apiKey: cfg.OCRAPIKey, model: cfg.OCRModel, client: &http.Client{Timeout: time.Duration(cfg.OCRTimeoutSeconds) * time.Second}}
	case "paddleocr":
		return &paddleProvider{baseURL: cfg.OCREndpoint, token: cfg.OCRAPIKey, model: cfg.OCRModel, timeout: time.Duration(cfg.OCRTimeoutSeconds) * time.Second}
	default:
		return disabledOCR{}
	}
}

type ocrSpaceProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

type ocrSpaceResponse struct {
	ParsedResults []struct {
		ParsedText        string `json:"ParsedText"`
		FileParseExitCode int    `json:"FileParseExitCode"`
		ErrorMessage      string `json:"ErrorMessage"`
		ErrorDetails      string `json:"ErrorDetails"`
	} `json:"ParsedResults"`
	OCRExitCode            int             `json:"OCRExitCode"`
	IsErroredOnProcessing  bool            `json:"IsErroredOnProcessing"`
	ErrorMessage           json.RawMessage `json:"ErrorMessage"`
	ErrorDetails           string          `json:"ErrorDetails"`
	ProcessingTimeInMillis string          `json:"ProcessingTimeInMilliseconds"`
}

func (p *ocrSpaceProvider) Extract(ctx context.Context, fileName string, data []byte, pageHint int) (ocrResult, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return ocrResult{}, &ocrFailure{Code: "OCR_CREDENTIAL_MISSING", Message: "OCR 服务凭据未配置", Retryable: false}
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", safeFileName(fileName))
	if err != nil {
		return ocrResult{}, err
	}
	if _, err := part.Write(data); err != nil {
		return ocrResult{}, err
	}
	for k, v := range map[string]string{
		"apikey": p.apiKey, "filetype": "PDF", "OCREngine": "2", "isTable": "true",
		"scale": "true", "detectOrientation": "true", "language": "chs",
	} {
		if err := mw.WriteField(k, v); err != nil {
			return ocrResult{}, err
		}
	}
	if err := mw.Close(); err != nil {
		return ocrResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, &body)
	if err != nil {
		return ocrResult{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_TIMEOUT", Message: "OCR 请求失败或超时：" + shortError(err), Retryable: true, Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20+1))
	if err != nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_RESPONSE_READ", Message: "读取 OCR 响应失败", Retryable: true, Err: err}
	}
	if len(raw) > 20<<20 {
		return ocrResult{}, &ocrFailure{Code: "OCR_RESPONSE_TOO_LARGE", Message: "OCR 响应超过 20 MB", Retryable: false}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return ocrResult{}, &ocrFailure{Code: "OCR_RATE_LIMITED", Message: "OCR 服务限流", Retryable: true}
	}
	if resp.StatusCode >= 500 {
		return ocrResult{}, &ocrFailure{Code: "OCR_UPSTREAM", Message: fmt.Sprintf("OCR 服务返回 %d", resp.StatusCode), Retryable: true}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ocrResult{}, &ocrFailure{Code: "OCR_UNAUTHORIZED", Message: "OCR 凭据无效或无权限", Retryable: false}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ocrResult{}, &ocrFailure{Code: "OCR_HTTP", Message: fmt.Sprintf("OCR 服务返回 %d", resp.StatusCode), Retryable: false}
	}
	var out ocrSpaceResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_RESPONSE_INVALID", Message: "OCR 响应不是有效 JSON", Retryable: false, Err: err}
	}
	if out.IsErroredOnProcessing {
		msg := rawErrorMessage(out.ErrorMessage)
		if msg == "" {
			msg = out.ErrorDetails
		}
		retry := strings.Contains(strings.ToLower(msg), "timed out") || strings.Contains(strings.ToLower(msg), "rate")
		return ocrResult{}, &ocrFailure{Code: "OCR_PROVIDER_ERROR", Message: "OCR 处理失败：" + truncate(msg, 200), Retryable: retry}
	}
	var pages []OCRPage
	for _, result := range out.ParsedResults {
		if result.ErrorMessage != "" {
			return ocrResult{}, &ocrFailure{Code: "OCR_PAGE_ERROR", Message: "OCR 单页失败：" + truncate(result.ErrorMessage+" "+result.ErrorDetails, 200), Retryable: false}
		}
		chunks := strings.Split(result.ParsedText, "\f")
		for _, chunk := range chunks {
			if strings.TrimSpace(chunk) != "" {
				pages = append(pages, OCRPage{Page: len(pages) + 1, Text: chunk})
			}
		}
	}
	if len(pages) == 0 {
		return ocrResult{}, &ocrFailure{Code: "OCR_NO_TEXT", Message: "OCR 未返回可读取文本", Retryable: false}
	}
	if pageHint > 0 && len(pages) != pageHint {
		// Never silently drop or merge pages: the physical page mapping is part of the archive.
		return ocrResult{}, &ocrFailure{Code: "OCR_PAGE_COUNT_MISMATCH", Message: fmt.Sprintf("OCR 返回 %d 页，PDF 有 %d 页，需管理员核对", len(pages), pageHint), Retryable: false}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		compact.Write(raw)
	}
	return ocrResult{Provider: "ocrspace", Model: p.model, Pages: pages, Raw: compact.Bytes()}, nil
}

func rawErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, "；")
	}
	return string(raw)
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "report.pdf"
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// ocrPageText joins pages with form feeds for display; page boundaries are kept in OCRPage.
func ocrPageText(pages []OCRPage) string {
	parts := make([]string, 0, len(pages))
	for _, p := range pages {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\f")
}

// paddleProvider calls the official PaddleOCR hosted API (AI Studio access token).
// The SDK submits a PDF job, polls it, and returns per-page pruned results; this adapter keeps the
// raw result as data, extracts page text in reading order, and classifies failures for retry.
type paddleProvider struct {
	baseURL string
	token   string
	model   string
	timeout time.Duration
}

func (p *paddleProvider) Extract(ctx context.Context, fileName string, data []byte, pageHint int) (ocrResult, error) {
	if strings.TrimSpace(p.token) == "" {
		return ocrResult{}, &ocrFailure{Code: "OCR_CREDENTIAL_MISSING", Message: "PaddleOCR 访问令牌未配置", Retryable: false}
	}
	tmp, err := os.CreateTemp("", "paddleocr-*.pdf")
	if err != nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_INPUT_WRITE", Message: "无法准备 OCR 输入文件", Retryable: true, Err: err}
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return ocrResult{}, &ocrFailure{Code: "OCR_INPUT_WRITE", Message: "无法写入 OCR 输入文件", Retryable: true, Err: err}
	}
	_ = tmp.Close()

	opts := []paddleocr.ClientOption{paddleocr.WithToken(p.token)}
	if p.baseURL != "" {
		opts = append(opts, paddleocr.WithBaseURL(p.baseURL))
	}
	if p.timeout > 0 {
		opts = append(opts, paddleocr.WithRequestTimeout(p.timeout), paddleocr.WithPollTimeout(p.timeout))
	}
	client, err := paddleocr.NewClient(opts...)
	if err != nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_CREDENTIAL_MISSING", Message: "PaddleOCR 客户端初始化失败：" + shortError(err), Retryable: false, Err: err}
	}
	model := p.model
	if model == "" {
		model = paddleocr.PPOCRv6
	}
	result, err := client.OCR(ctx, &paddleocr.OCRRequest{Model: model, FilePath: tmp.Name()})
	if err != nil {
		return ocrResult{}, classifyPaddleError(err)
	}
	if result == nil {
		return ocrResult{}, &ocrFailure{Code: "OCR_RESPONSE_INVALID", Message: "PaddleOCR 返回空结果", Retryable: false}
	}
	pages := make([]OCRPage, 0, len(result.Pages))
	for _, page := range result.Pages {
		text := paddlePageText(page.PrunedResult)
		if strings.TrimSpace(text) == "" {
			// Empty pages are possible for blank pages, but a page must still exist so the physical
			// page mapping stays complete.
			text = ""
		}
		pages = append(pages, OCRPage{Page: len(pages) + 1, Text: text, Boxes: paddlePageBoxes(page.PrunedResult)})
	}
	if len(pages) == 0 {
		return ocrResult{}, &ocrFailure{Code: "OCR_NO_TEXT", Message: "PaddleOCR 未返回页面结果", Retryable: false}
	}
	if pageHint > 0 && len(pages) != pageHint {
		return ocrResult{}, &ocrFailure{Code: "OCR_PAGE_COUNT_MISMATCH", Message: fmt.Sprintf("PaddleOCR 返回 %d 页，PDF 有 %d 页，需管理员核对", len(pages), pageHint), Retryable: false}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		raw = []byte("{}")
	}
	if len(raw) > 20<<20 {
		// Keep the response bounded while recording that the raw payload was truncated; page text
		// remains complete and is stored separately.
		raw, _ = json.Marshal(map[string]any{"jobId": result.JobID, "pageCount": len(result.Pages), "truncated": true})
	}
	return ocrResult{Provider: "paddleocr", Model: model, Pages: pages, Raw: raw}, nil
}

func classifyPaddleError(err error) *ocrFailure {
	var auth *paddleocr.AuthError
	var invalid *paddleocr.InvalidRequestError
	var rate *paddleocr.RateLimitError
	var unavailable *paddleocr.ServiceUnavailableError
	var apiErr *paddleocr.APIError
	var failed *paddleocr.JobFailedError
	var reqTimeout *paddleocr.RequestTimeoutError
	var pollTimeout *paddleocr.PollTimeoutError
	var netErr *paddleocr.NetworkError
	var fileNotFound *paddleocr.FileNotFoundError
	var responseErr *paddleocr.ResponseFormatError
	var parseErr *paddleocr.ResultParseError
	msg := shortError(err)
	switch {
	case errors.As(err, &auth):
		return &ocrFailure{Code: "OCR_UNAUTHORIZED", Message: "PaddleOCR 访问令牌无效或过期", Retryable: false, Err: err}
	case errors.As(err, &invalid), errors.As(err, &fileNotFound), errors.As(err, &failed):
		// PaddleOCR answers a full submission queue as HTTP 400 with code 10010; that is a
		// provider-side capacity limit, not a bad request, so it must be retried.
		if strings.Contains(msg, "队列已满") || strings.Contains(msg, "10010") || strings.Contains(msg, "稍后重试") {
			return &ocrFailure{Code: "OCR_RATE_LIMITED", Message: "PaddleOCR 提交队列已满，稍后重试", Retryable: true, Err: err}
		}
		return &ocrFailure{Code: "OCR_REQUEST_INVALID", Message: "PaddleOCR 请求被拒绝：" + msg, Retryable: false, Err: err}
	case errors.As(err, &rate):
		return &ocrFailure{Code: "OCR_RATE_LIMITED", Message: "PaddleOCR 限流", Retryable: true, Err: err}
	case errors.As(err, &unavailable), errors.As(err, &netErr), errors.As(err, &reqTimeout), errors.As(err, &pollTimeout):
		return &ocrFailure{Code: "OCR_TRANSIENT", Message: "PaddleOCR 临时故障：" + msg, Retryable: true, Err: err}
	case errors.As(err, &apiErr):
		if apiErr.StatusCode >= 500 {
			return &ocrFailure{Code: "OCR_UPSTREAM", Message: fmt.Sprintf("PaddleOCR 服务返回 %d", apiErr.StatusCode), Retryable: true, Err: err}
		}
		return &ocrFailure{Code: "OCR_HTTP", Message: msg, Retryable: false, Err: err}
	case errors.As(err, &responseErr), errors.As(err, &parseErr):
		return &ocrFailure{Code: "OCR_RESPONSE_INVALID", Message: "PaddleOCR 响应无法解析：" + msg, Retryable: false, Err: err}
	default:
		return &ocrFailure{Code: "OCR_ERROR", Message: msg, Retryable: true, Err: err}
	}
}

// paddlePageText extracts reading-order text from a PaddleOCR pruned result. The result shape has
// changed across model versions (rec_texts, data rows, generic text fields), so the extractor
// accepts all known shapes and keeps array order.

func paddlePageBoxes(value any) []OCRBox {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	texts, ok := m["rec_texts"].([]any)
	if !ok || len(texts) == 0 {
		return nil
	}
	var rawBoxes []any
	if boxes, ok := m["rec_boxes"].([]any); ok && len(boxes) == len(texts) {
		rawBoxes = boxes
	} else if polys, ok := m["dt_polys"].([]any); ok && len(polys) == len(texts) {
		rawBoxes = polys
	} else {
		return nil
	}
	out := make([]OCRBox, 0, len(texts))
	for i, rawText := range texts {
		text, ok := rawText.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		box, ok := parseOCRBox(rawBoxes[i])
		if !ok {
			continue
		}
		out = append(out, OCRBox{Text: strings.TrimSpace(text), X: box[0], Y: box[1], W: box[2], H: box[3]})
	}
	return out
}

func parseOCRBox(v any) ([4]float64, bool) {
	switch t := v.(type) {
	case []any:
		if len(t) != 4 {
			return [4]float64{}, false
		}
		nums := make([]float64, 4)
		for i, item := range t {
			n, ok := item.(float64)
			if !ok {
				// Some payloads encode numbers as strings.
				if s, ok := item.(string); ok {
					parsed, err := strconv.ParseFloat(s, 64)
					if err != nil {
						return [4]float64{}, false
					}
					n = parsed
				} else {
					return [4]float64{}, false
				}
			}
			nums[i] = n
		}
		// rec_boxes is [x, y, w, h]; dt_polys is [[x1,y1],[x2,y1],[x2,y2],[x1,y2]] flattened
		// into four numbers in some payloads.
		if nums[2] > nums[0] && nums[3] > nums[1] {
			return [4]float64{nums[0], nums[1], nums[2] - nums[0], nums[3] - nums[1]}, true
		}
		return [4]float64{nums[0], nums[1], nums[2], nums[3]}, true
	case []float64:
		if len(t) != 4 {
			return [4]float64{}, false
		}
		if t[2] > t[0] && t[3] > t[1] {
			return [4]float64{t[0], t[1], t[2] - t[0], t[3] - t[1]}, true
		}
		return [4]float64{t[0], t[1], t[2], t[3]}, true
	}
	return [4]float64{}, false
}

// rebuildPaddlePages fills OCRPage boxes from the stored PaddleOCR raw payload when an older run
// was recorded without positions. It is best-effort: missing positions keep the line parser.
func rebuildPaddlePages(raw json.RawMessage, pages []OCRPage) []OCRPage {
	if len(raw) == 0 || len(pages) == 0 {
		return pages
	}
	for _, p := range pages {
		if len(p.Boxes) > 0 {
			return pages
		}
	}
	var payload struct {
		Pages []struct {
			Raw map[string]any `json:"raw"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Pages) != len(pages) {
		return pages
	}
	for i, page := range payload.Pages {
		pruned, ok := page.Raw["prunedResult"]
		if !ok {
			continue
		}
		if boxes := paddlePageBoxes(pruned); len(boxes) > 0 {
			pages[i].Boxes = boxes
		}
	}
	return pages
}

func paddlePageText(value any) string {
	out := []string{}
	seen := map[string]bool{}
	var walk func(v any, depth int)
	walk = func(v any, depth int) {
		if depth > 24 || len(out) > 20000 {
			return
		}
		switch t := v.(type) {
		case string:
			text := strings.TrimSpace(t)
			if text != "" && !seen[text] {
				seen[text] = true
				out = append(out, text)
			}
		case []any:
			allStrings := true
			for _, item := range t {
				if _, ok := item.(string); !ok {
					allStrings = false
					break
				}
			}
			if allStrings {
				for _, item := range t {
					walk(item, depth+1)
				}
				return
			}
			// Common OCR shapes: ["text", score] or [[box...], "text", score].
			if len(t) >= 2 {
				if s, ok := t[1].(string); ok && len(t) <= 4 {
					text := strings.TrimSpace(s)
					if text != "" && !seen[text] {
						seen[text] = true
						out = append(out, text)
					}
					return
				}
			}
			for _, item := range t {
				walk(item, depth+1)
			}
		case map[string]any:
			// Prefer known text arrays before walking other keys so box metadata is not mistaken
			// for text.
			for _, key := range []string{"rec_texts", "texts", "strs", "labels"} {
				if arr, ok := t[key].([]any); ok {
					walk(arr, depth+1)
					return
				}
			}
			for _, key := range []string{"rec_text", "text", "str", "label", "content", "value"} {
				if s, ok := t[key].(string); ok {
					text := strings.TrimSpace(s)
					if text != "" && !seen[text] {
						seen[text] = true
						out = append(out, text)
					}
				}
			}
			keys := make([]string, 0, len(t))
			for key := range t {
				switch key {
				case "rec_texts", "texts", "strs", "labels", "rec_text", "text", "str", "label", "content", "value":
					continue
				default:
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(t[key], depth+1)
			}
		}
	}
	walk(value, 0)
	return strings.Join(out, "\n")
}
