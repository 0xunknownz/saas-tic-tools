package reportarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/0xunknownz/saas.starter/foundation/plugin"
	"github.com/0xunknownz/saas.starter/foundation/tenant"
)

type apiHandler struct {
	svc      *Service
	identity identity.PlatformGuard
	tenant   tenant.Membership
}

type session struct {
	TenantID  string
	AccountID string
	Role      string
	Actor     string
}

const platformTemplatesPermission = "platform.product.reportarchive.templates"

func RegisterHTTP(r plugin.Router, svc *Service, deps HTTPDeps) error {
	a := &apiHandler{svc: svc, identity: deps.Identity, tenant: deps.Tenant}
	tenantRoute := func(pattern string, h func(http.ResponseWriter, *http.Request, session)) {
		r.Handle(pattern, deps.Identity.RequireTenant(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s, ok := a.session(w, req)
			if !ok {
				return
			}
			h(w, req, s)
		})))
	}
	adminRoute := func(pattern string, h func(http.ResponseWriter, *http.Request, session)) {
		tenantRoute(pattern, func(w http.ResponseWriter, req *http.Request, s session) {
			if s.Role != tenant.RoleTenantAdmin {
				writeError(w, ErrForbidden)
				return
			}
			h(w, req, s)
		})
	}
	platformRoute := func(pattern string, h func(http.ResponseWriter, *http.Request)) {
		r.Handle(pattern, deps.Identity.RequirePlatform(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !deps.Identity.PlatformCan(req.Context(), platformTemplatesPermission) {
				writeError(w, ErrForbidden)
				return
			}
			h(w, req)
		})))
	}

	tenantRoute("GET /api/tenant/v1/report-archive/reports", a.listReports)
	tenantRoute("GET /api/tenant/v1/report-archive/reports/{id}", a.getReport)
	tenantRoute("GET /api/tenant/v1/report-archive/reports/{id}/ocr", a.getOCR)
	tenantRoute("GET /api/tenant/v1/report-archive/reports/{id}/candidate", a.getCandidate)
	adminRoute("GET /api/tenant/v1/report-archive/reports/{id}/validation", a.getValidation)
	tenantRoute("GET /api/tenant/v1/report-archive/reports/{id}/file", a.fileInline)
	tenantRoute("GET /api/tenant/v1/report-archive/reports/{id}/download", a.fileDownload)
	tenantRoute("POST /api/tenant/v1/report-archive/uploads", a.upload)
	tenantRoute("GET /api/tenant/v1/report-archive/uploads/{id}", a.getBatch)
	tenantRoute("POST /api/tenant/v1/report-archive/uploads/{id}/items", a.retryUpload)
	adminRoute("PATCH /api/tenant/v1/report-archive/reports/{id}", a.updateReport)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/archive", a.archive)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/void", a.void)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/restore", a.restore)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/revision", a.revision)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/reparse", a.reparse)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/retry-ocr", a.retryOCR)
	adminRoute("POST /api/tenant/v1/report-archive/reports/{id}/candidate/apply", a.applyCandidate)
	adminRoute("GET /api/tenant/v1/report-archive/rules", a.listRules)
	adminRoute("POST /api/tenant/v1/report-archive/rules", a.createRule)
	adminRoute("POST /api/tenant/v1/report-archive/rules/{id}/activate", a.activateRule)
	adminRoute("POST /api/tenant/v1/report-archive/rules/{id}/trial", a.trialRule)
	platformRoute("GET /api/product-admin/v1/report-templates", a.listTemplates)
	platformRoute("POST /api/product-admin/v1/report-templates", a.createTemplate)
	platformRoute("POST /api/product-admin/v1/report-templates/{id}/activate", a.activateTemplate)
	return nil
}

func (a *apiHandler) session(w http.ResponseWriter, req *http.Request) (session, bool) {
	p := identity.TenantPrincipalFrom(req.Context())
	if p == nil {
		writeError(w, ErrUnauthenticated)
		return session{}, false
	}
	if p.TenantID == "" {
		writeError(w, ErrNoTenant)
		return session{}, false
	}
	role, err := a.tenant.Role(req.Context(), p.TenantID, p.AccountID)
	if errors.Is(err, tenant.ErrMemberNotFound) {
		writeError(w, ErrNotMember)
		return session{}, false
	}
	if err != nil {
		writeError(w, fail(http.StatusInternalServerError, "MEMBERSHIP_CHECK", "无法确认租户成员资格"))
		return session{}, false
	}
	if role != tenant.RoleTenantAdmin && role != tenant.RoleMember {
		writeError(w, ErrForbidden)
		return session{}, false
	}
	return session{TenantID: p.TenantID, AccountID: p.AccountID, Role: role, Actor: "account:" + p.AccountID}, true
}

func (a *apiHandler) listReports(w http.ResponseWriter, req *http.Request, s session) {
	q := req.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("pageSize"))
	f := ListFilter{Query: q.Get("query"), Institution: q.Get("institution"), DateFrom: q.Get("dateFrom"), DateTo: q.Get("dateTo"),
		Status: q.Get("status"), Item: q.Get("item"), Judgement: q.Get("judgement"), Scope: q.Get("scope"), Page: page, PageSize: size}
	out, err := a.svc.store.listReports(req.Context(), s.TenantID, f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) getReport(w http.ResponseWriter, req *http.Request, s session) {
	r, err := a.svc.reportDetail(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, r)
}

func (a *apiHandler) getOCR(w http.ResponseWriter, req *http.Request, s session) {
	if _, err := a.svc.store.getReport(req.Context(), s.TenantID, req.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	ocr, err := a.svc.store.latestOCRRun(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if ocr == nil {
		writeError(w, fail(404, "OCR_NOT_FOUND", "该档案尚无 OCR 结果"))
		return
	}
	writeJSON(w, http.StatusOK, ocr)
}

func (a *apiHandler) getCandidate(w http.ResponseWriter, req *http.Request, s session) {
	c, err := a.svc.candidate(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if c == nil {
		writeError(w, fail(404, "CANDIDATE_NOT_FOUND", "没有待确认的重解析候选"))
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *apiHandler) getValidation(w http.ResponseWriter, req *http.Request, s session) {
	issues, err := a.svc.validationPreview(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ValidationPreview{Issues: issues})
}

func (a *apiHandler) fileInline(w http.ResponseWriter, req *http.Request, s session) {
	a.serveFile(w, req, s, "inline")
}
func (a *apiHandler) fileDownload(w http.ResponseWriter, req *http.Request, s session) {
	a.serveFile(w, req, s, "attachment")
}

func (a *apiHandler) serveFile(w http.ResponseWriter, req *http.Request, s session, disposition string) {
	r, err := a.svc.store.getReport(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	_, content, err := a.svc.store.getFile(req.Context(), s.TenantID, r.File.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": r.File.FileName}))
	// ServeContent adds Accept-Ranges and serves single byte ranges, which the PDF viewer uses to
	// fetch only the pages it needs. Access was already checked above.
	http.ServeContent(w, req, r.File.FileName, r.File.CreatedAt, bytes.NewReader(content))
}

func (a *apiHandler) upload(w http.ResponseWriter, req *http.Request, s session) {
	files, cleanup, apiErr := readUploadParts(w, req, a.svc.cfg.MaxUploadFiles, a.svc.cfg.MaxUploadBytes)
	defer cleanup()
	if apiErr != nil {
		writeError(w, apiErr)
		return
	}
	batchID, err := a.svc.createUploadBatch(req.Context(), s.TenantID, s.Actor)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.svc.uploadFiles(req.Context(), s.TenantID, s.Actor, batchID, files)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, batch)
}

func (a *apiHandler) getBatch(w http.ResponseWriter, req *http.Request, s session) {
	b, err := a.svc.batch(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if s.Role != tenant.RoleTenantAdmin && b.CreatedBy != s.Actor {
		writeError(w, ErrForbidden)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *apiHandler) retryUpload(w http.ResponseWriter, req *http.Request, s session) {
	files, cleanup, apiErr := readUploadParts(w, req, 1, a.svc.cfg.MaxUploadBytes)
	defer cleanup()
	if apiErr != nil {
		writeError(w, apiErr)
		return
	}
	if len(files) != 1 {
		writeError(w, fail(400, "FILE_REQUIRED", "一次只能重试一个文件"))
		return
	}
	b, err := a.svc.retryOneUpload(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), files[0])
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *apiHandler) updateReport(w http.ResponseWriter, req *http.Request, s session) {
	var in ReportUpdate
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := a.svc.updateReport(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type reasonInput struct {
	Reason           string   `json:"reason"`
	ConfirmedIssues  []string `json:"confirmedIssues"`
	ExpectedRevision int      `json:"expectedRevision"`
}

func (a *apiHandler) archive(w http.ResponseWriter, req *http.Request, s session) {
	var in reasonInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := a.svc.manualArchive(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in.Reason, in.ConfirmedIssues)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) void(w http.ResponseWriter, req *http.Request, s session) {
	a.reasonAction(w, req, s, a.svc.voidReport)
}
func (a *apiHandler) restore(w http.ResponseWriter, req *http.Request, s session) {
	a.reasonAction(w, req, s, a.svc.restoreReport)
}
func (a *apiHandler) revision(w http.ResponseWriter, req *http.Request, s session) {
	a.reasonAction(w, req, s, a.svc.confirmRevision)
}

type reasonFunc func(context.Context, string, string, string, string) (Report, error)

func (a *apiHandler) reasonAction(w http.ResponseWriter, req *http.Request, s session, fn reasonFunc) {
	var in reasonInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := fn(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) reparse(w http.ResponseWriter, req *http.Request, s session) {
	var in reasonInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		writeError(w, fail(400, "REASON_REQUIRED", "重解析必须填写原因"))
		return
	}
	if err := a.svc.requestReparse(req.Context(), s.TenantID, s.Actor, req.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (a *apiHandler) retryOCR(w http.ResponseWriter, req *http.Request, s session) {
	var in reasonInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	if err := a.svc.retryOCR(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in.Reason); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (a *apiHandler) applyCandidate(w http.ResponseWriter, req *http.Request, s session) {
	var in reasonInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	if _, err := a.svc.applyCandidate(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in.ExpectedRevision); err != nil {
		writeError(w, err)
		return
	}
	out, err := a.svc.reportDetail(req.Context(), s.TenantID, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) listRules(w http.ResponseWriter, req *http.Request, s session) {
	rules, err := a.svc.listRules(req.Context(), s.TenantID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

func (a *apiHandler) createRule(w http.ResponseWriter, req *http.Request, s session) {
	var in RuleInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := a.svc.createRuleVersion(req.Context(), s.TenantID, s.Actor, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *apiHandler) activateRule(w http.ResponseWriter, req *http.Request, s session) {
	out, err := a.svc.activateRule(req.Context(), s.TenantID, s.Actor, req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) trialRule(w http.ResponseWriter, req *http.Request, s session) {
	var in TrialInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := a.svc.trialRule(req.Context(), s.TenantID, s.Actor, req.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiHandler) listTemplates(w http.ResponseWriter, req *http.Request) {
	rules, err := a.svc.builtinRules(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

func (a *apiHandler) createTemplate(w http.ResponseWriter, req *http.Request) {
	var in RuleInput
	if !a.identity.GuardJSON(w, req, &in) {
		return
	}
	out, err := a.svc.createBuiltinRule(req.Context(), "platform", in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *apiHandler) activateTemplate(w http.ResponseWriter, req *http.Request) {
	out, err := a.svc.activateBuiltinRule(req.Context(), "platform", req.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var ae *APIError
	switch {
	case errors.As(err, &ae):
		writeJSON(w, ae.Status, map[string]string{"code": ae.Code, "message": ae.Message})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "记录不存在"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "CONFLICT", "message": "数据已变化，请刷新后重试"})
	case errors.Is(err, ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "INVALID_INPUT", "message": strings.TrimPrefix(err.Error(), "report archive: invalid input: ")})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "INTERNAL", "message": "服务器处理失败，请稍后重试"})
	}
}

func readUploadParts(w http.ResponseWriter, req *http.Request, maxFiles int, maxBytes int64) ([]pendingUpload, func(), *APIError) {
	cleanup := func() {}
	if maxFiles < 1 {
		maxFiles = 50
	}
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes*int64(maxFiles)+20<<20)
	reader, err := req.MultipartReader()
	if err != nil {
		return nil, cleanup, fail(400, "MULTIPART_REQUIRED", "请使用 multipart/form-data 上传 PDF")
	}
	dir, err := os.MkdirTemp("", "tic-report-upload-")
	if err != nil {
		return nil, cleanup, fail(500, "TEMP_UNAVAILABLE", "无法创建上传临时目录")
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	var files []pendingUpload
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, cleanup, fail(400, "MULTIPART_INVALID", "上传数据读取失败")
		}
		if part.FileName() == "" {
			_ = part.Close()
			continue
		}
		if len(files) >= maxFiles {
			_ = part.Close()
			return nil, cleanup, fail(400, "BATCH_TOO_LARGE", fmt.Sprintf("一次最多上传 %d 份 PDF", maxFiles))
		}
		name := safeFileName(part.FileName())
		f, err := os.CreateTemp(dir, "upload-*.pdf")
		if err != nil {
			_ = part.Close()
			return nil, cleanup, fail(500, "TEMP_UNAVAILABLE", "无法创建上传临时文件")
		}
		n, copyErr := io.Copy(f, io.LimitReader(part, maxBytes+1))
		_ = f.Close()
		_ = part.Close()
		if copyErr != nil {
			return nil, cleanup, fail(400, "MULTIPART_INVALID", "上传文件读取失败")
		}
		files = append(files, pendingUpload{Name: name, Path: f.Name(), Size: n, Oversize: n > maxBytes})
	}
	if len(files) == 0 {
		return nil, cleanup, fail(400, "FILE_REQUIRED", "请选择至少一份 PDF")
	}
	return files, cleanup, nil
}
