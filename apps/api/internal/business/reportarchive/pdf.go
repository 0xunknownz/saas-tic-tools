package reportarchive

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// inspectPDF verifies that data is a readable PDF and returns its physical page count. Permission
// encryption with an empty user password is readable and accepted; a PDF that needs a password is
// rejected with a specific reason. Limits apply to the real file, never to the file name or header.
func inspectPDF(data []byte, maxBytes int64, maxPages int) (int, *APIError) {
	if len(data) == 0 {
		return 0, fail(400, "PDF_EMPTY", "文件为空")
	}
	if int64(len(data)) > maxBytes {
		return 0, fail(400, "PDF_TOO_LARGE", fmt.Sprintf("文件超过 %d 字节上限", maxBytes))
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return 0, fail(400, "PDF_TYPE_INVALID", "文件内容不是 PDF")
	}

	api.DisableConfigDir()
	done := make(chan struct{})
	var pages int
	var err error
	go func() {
		defer close(done)
		conf := model.NewDefaultConfiguration()
		conf.ValidationMode = model.ValidationRelaxed
		conf.UserPW = ""
		pages, err = api.PageCount(bytes.NewReader(data), conf)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		return 0, fail(400, "PDF_UNREADABLE", "PDF 校验超时，无法确认可读性")
	}
	if err != nil {
		switch {
		case errors.Is(err, pdfcpu.ErrWrongPassword):
			return 0, fail(400, "PDF_PASSWORD_REQUIRED", "PDF 需要打开密码，无法接收")
		case strings.Contains(strings.ToLower(err.Error()), "password"):
			return 0, fail(400, "PDF_PASSWORD_REQUIRED", "PDF 需要打开密码，无法接收")
		default:
			return 0, fail(400, "PDF_CORRUPT", "PDF 损坏或无法解析："+shortError(err))
		}
	}
	if pages < 1 {
		return 0, fail(400, "PDF_EMPTY_PAGES", "PDF 没有可读取的物理页")
	}
	if pages > maxPages {
		return 0, fail(400, "PDF_TOO_MANY_PAGES", fmt.Sprintf("PDF 有 %d 个物理页，超过 %d 页上限", pages, maxPages))
	}
	return pages, nil
}

func shortError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
