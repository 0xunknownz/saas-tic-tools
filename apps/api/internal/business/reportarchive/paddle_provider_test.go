package reportarchive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func paddleProtocolServer(t *testing.T, pages int) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ocr/jobs":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "success", "data": map[string]string{"jobId": "job-1"}})
		case r.URL.Path == "/api/v2/ocr/jobs/job-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "success", "data": map[string]any{
				"jobId": "job-1", "state": "done",
				"resultUrl": map[string]string{"jsonUrl": srv.URL + "/result.jsonl"},
			}})
		case r.URL.Path == "/result.jsonl":
			w.Header().Set("Content-Type", "application/octet-stream")
			for i := 0; i < pages; i++ {
				item := map[string]any{"result": map[string]any{
					"dataInfo":   map[string]any{},
					"ocrResults": []any{map[string]any{"prunedResult": map[string]any{"res": map[string]any{"rec_texts": []string{"page line " + string(rune('A'+i))}}}}},
				}}
				raw, _ := json.Marshal(item)
				_, _ = w.Write(append(raw, '\n'))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv = httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestPaddleProviderE2EAgainstOfficialProtocol(t *testing.T) {
	srv := paddleProtocolServer(t, 2)
	p := &paddleProvider{baseURL: srv.URL, token: "test-token", model: "PP-OCRv6", timeout: 10 * time.Second}
	result, err := p.Extract(context.Background(), "two.pdf", minimalPDF(2), 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "paddleocr" || result.Model != "PP-OCRv6" || len(result.Pages) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Raw) == 0 || result.Pages[0].Text == "" {
		t.Fatalf("raw/pages missing: %+v", result)
	}
	if _, err := p.Extract(context.Background(), "three.pdf", minimalPDF(3), 3); err == nil {
		t.Fatal("page mismatch accepted")
	} else {
		var failure *ocrFailure
		if !asOCRFailure(err, &failure) || failure.Code != "OCR_PAGE_COUNT_MISMATCH" {
			t.Fatalf("mismatch err = %v", err)
		}
	}
}

func asOCRFailure(err error, target **ocrFailure) bool {
	f, ok := err.(*ocrFailure)
	if ok {
		*target = f
	}
	return ok
}
