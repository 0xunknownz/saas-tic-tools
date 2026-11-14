package reportarchive

import (
	"context"
	"errors"
	"testing"
	"time"

	paddleocr "github.com/PaddlePaddle/PaddleOCR/api_sdk/go"
)

func TestPaddlePageTextExtractsKnownShapes(t *testing.T) {
	recTexts := map[string]any{"res": map[string]any{"rec_texts": []any{"报告编号：ZBJ-1", "产品名称：样品"}}}
	if got := paddlePageText(recTexts); got != "报告编号：ZBJ-1\n产品名称：样品" {
		t.Fatalf("rec_texts = %q", got)
	}
	dataRows := map[string]any{"data": []any{
		[]any{[]any{1, 2, 3, 4}, "检测项目", 0.99},
		[]any{[]any{1, 2, 3, 4}, "技术要求", 0.98},
	}}
	if got := paddlePageText(dataRows); got != "检测项目\n技术要求" {
		t.Fatalf("data rows = %q", got)
	}
	generic := map[string]any{"pages": []any{map[string]any{"text": "第一页"}}}
	if got := paddlePageText(generic); got != "第一页" {
		t.Fatalf("generic = %q", got)
	}
}

func TestPaddleProviderRequiresToken(t *testing.T) {
	p := &paddleProvider{model: "PP-OCRv6"}
	_, err := p.Extract(context.Background(), "x.pdf", minimalPDF(1), 1)
	var failure *ocrFailure
	if !errors.As(err, &failure) || failure.Code != "OCR_CREDENTIAL_MISSING" || failure.Retryable {
		t.Fatalf("err = %v", err)
	}
}

func TestClassifyPaddleErrors(t *testing.T) {
	auth := &paddleocr.AuthError{PaddleOCRAPIError: paddleocr.PaddleOCRAPIError{Message: "token"}}
	if failure := classifyPaddleError(auth); failure.Retryable || failure.Code != "OCR_UNAUTHORIZED" {
		t.Fatalf("auth = %+v", failure)
	}
	rate := &paddleocr.RateLimitError{APIError: paddleocr.APIError{StatusCode: 429, PaddleOCRAPIError: paddleocr.PaddleOCRAPIError{Message: "rate"}}}
	if failure := classifyPaddleError(rate); !failure.Retryable || failure.Code != "OCR_RATE_LIMITED" {
		t.Fatalf("rate = %+v", failure)
	}
	upstream := &paddleocr.APIError{StatusCode: 503, PaddleOCRAPIError: paddleocr.PaddleOCRAPIError{Message: "busy"}}
	if failure := classifyPaddleError(upstream); !failure.Retryable || failure.Code != "OCR_UPSTREAM" {
		t.Fatalf("upstream = %+v", failure)
	}
	queueFull := &paddleocr.InvalidRequestError{PaddleOCRAPIError: paddleocr.PaddleOCRAPIError{Message: "Bad request: {\"code\":10010,\"msg\":\"任务提交队列已满，请稍后重试\"}"}}
	if failure := classifyPaddleError(queueFull); !failure.Retryable || failure.Code != "OCR_RATE_LIMITED" {
		t.Fatalf("queue full = %+v", failure)
	}
	_ = time.Second
}
