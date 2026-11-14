package reportarchive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func minimalPDF(pages int) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := []int{}
	add := func(s string) int {
		offsets = append(offsets, buf.Len())
		buf.WriteString(s)
		return len(offsets)
	}
	add("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	kids := make([]string, 0, pages)
	for i := 1; i <= pages; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 2+i))
	}
	add(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", join(kids, " "), pages))
	for i := 1; i <= pages; i++ {
		add(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n", 2+i))
	}
	xref := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", pages+3))
	for _, off := range offsets {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", pages+3, xref))
	return buf.Bytes()
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func TestInspectPDFAcceptsReadableAndRejectsLimits(t *testing.T) {
	one := minimalPDF(1)
	if pages, err := inspectPDF(one, 50_000_000, 100); err != nil || pages != 1 {
		t.Fatalf("one page: pages=%d err=%v", pages, err)
	}
	many := minimalPDF(100)
	if pages, err := inspectPDF(many, 50_000_000, 100); err != nil || pages != 100 {
		t.Fatalf("100 pages: pages=%d err=%v", pages, err)
	}
	if _, err := inspectPDF(minimalPDF(101), 50_000_000, 100); err == nil || err.Code != "PDF_TOO_MANY_PAGES" {
		t.Fatalf("101 pages: err=%v", err)
	}
	if _, err := inspectPDF([]byte("%PDF-1.4\nnot really"), 50_000_000, 100); err == nil || err.Code != "PDF_CORRUPT" {
		t.Fatalf("corrupt: err=%v", err)
	}
	if _, err := inspectPDF([]byte("hello"), 50_000_000, 100); err == nil || err.Code != "PDF_TYPE_INVALID" {
		t.Fatalf("non-pdf: err=%v", err)
	}
	if _, err := inspectPDF(nil, 50_000_000, 100); err == nil || err.Code != "PDF_EMPTY" {
		t.Fatalf("empty: err=%v", err)
	}
	if _, err := inspectPDF(minimalPDF(1), 10, 100); err == nil || err.Code != "PDF_TOO_LARGE" {
		t.Fatalf("too large: err=%v", err)
	}
	// Exact byte boundary: the file length itself is accepted; one byte more is rejected.
	exact := minimalPDF(1)
	if pages, err := inspectPDF(exact, int64(len(exact)), 100); err != nil || pages != 1 {
		t.Fatalf("exact size: pages=%d err=%v", pages, err)
	}
	if _, err := inspectPDF(exact, int64(len(exact))-1, 100); err == nil || err.Code != "PDF_TOO_LARGE" {
		t.Fatalf("one over size: err=%v", err)
	}
}

func TestInspectPDFPermissionEncryptedWithEmptyPassword(t *testing.T) {
	pdfcpuapi.DisableConfigDir()
	dir := t.TempDir()
	in := filepath.Join(dir, "plain.pdf")
	out := filepath.Join(dir, "encrypted.pdf")
	if err := os.WriteFile(in, minimalPDF(1), 0o600); err != nil {
		t.Fatal(err)
	}
	conf := model.NewDefaultConfiguration()
	conf.UserPW = ""
	conf.OwnerPW = "owner-only"
	conf.Permissions = model.PermissionsPrint
	if err := pdfcpuapi.EncryptFile(in, out, conf); err != nil {
		t.Skipf("pdfcpu cannot create empty-user-password fixture: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if pages, err := inspectPDF(data, 50_000_000, 100); err != nil || pages != 1 {
		t.Fatalf("encrypted with empty password: pages=%d err=%v", pages, err)
	}
}
