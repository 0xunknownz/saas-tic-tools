package reportarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
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

func parsePositiveInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
