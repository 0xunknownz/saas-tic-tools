package reportarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/jackc/pgx/v5"
)

type ReportUpdate struct {
	ReportNumber     string   `json:"reportNumber"`
	ReportName       string   `json:"reportName"`
	ProductName      string   `json:"productName"`
	Institution      string   `json:"institution"`
	ClientUnit       string   `json:"clientUnit"`
	InspectedUnit    string   `json:"inspectedUnit"`
	Manufacturer     string   `json:"manufacturer"`
	Model            string   `json:"model"`
	Category         string   `json:"category"`
	Basis            string   `json:"basis"`
	ReportDate       string   `json:"reportDate"`
	Conclusion       string   `json:"conclusion"`
	Details          []Detail `json:"details"`
	ExpectedRevision int      `json:"expectedRevision"`
}

func (s *Service) reportDetail(ctx context.Context, tenantID, id string) (Report, error) {
	r, err := s.store.getReport(ctx, tenantID, id)
	if err != nil {
		return r, err
	}
	r.Details, _ = s.store.listDetails(ctx, tenantID, id)
	r.OCR, _ = s.store.latestOCRRun(ctx, tenantID, id)
	if r.OCR != nil {
		// The detail response keeps page text; raw provider JSON is available from the dedicated OCR
		// endpoint and would otherwise make every detail request unnecessarily large.
		r.OCR.Raw = nil
	}
	r.LatestParse, _ = s.store.latestParseRun(ctx, tenantID, id)
	r.Events, _ = s.store.listEvents(ctx, tenantID, id)
	r.Versions, _ = s.store.listVersions(ctx, tenantID, id)
	return r, nil
}

func (s *Service) updateReport(ctx context.Context, tenantID, actor, id string, in ReportUpdate) (Report, error) {
	if in.ExpectedRevision < 1 {
		return Report{}, fmt.Errorf("%w: expectedRevision is required", ErrInvalidInput)
	}
	for name, value := range map[string]string{"reportNumber": in.ReportNumber, "reportDate": in.ReportDate} {
		if value == "" {
			continue
		}
		if name == "reportDate" {
			if _, err := normalizeDate(value); err != nil {
				return Report{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}
		}
	}
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt != nil {
			return fail(409, "REPORT_VOIDED", "已作废档案不能编辑")
		}
		before := current
		current.ReportNumber = strings.ToUpper(trim(in.ReportNumber))
		current.ReportName = trim(in.ReportName)
		current.ProductName = trim(in.ProductName)
		current.Institution = trim(in.Institution)
		current.ClientUnit = trim(in.ClientUnit)
		current.InspectedUnit = trim(in.InspectedUnit)
		current.Manufacturer = trim(in.Manufacturer)
		current.Model = trim(in.Model)
		current.Category = trim(in.Category)
		current.Basis = trim(in.Basis)
		current.Conclusion = trim(in.Conclusion)
		if in.ReportDate != "" {
			d, err := normalizeDate(in.ReportDate)
			if err != nil {
				return err
			}
			current.ReportDate = d
		} else {
			current.ReportDate = ""
		}
		current.DetectionResult = deriveDetection(current.Conclusion)
		if current.ReportName == "" && current.ProductName != "" {
			current.ReportName = current.ProductName + "检测报告"
		}
		if err := s.store.updateReportTx(ctx, tx, tenantID, current, in.ExpectedRevision); err != nil {
			return err
		}
		if in.Details != nil {
			if err := s.store.replaceDetailsTx(ctx, tx, tenantID, id, in.Details); err != nil {
				return err
			}
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "manual_edit", actor, "管理员修正资料", before, current); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) manualArchive(ctx context.Context, tenantID, actor, id, reason string, confirmed []string) (Report, error) {
	reason = trim(reason)
	if reason == "" {
		return Report{}, fail(400, "REASON_REQUIRED", "人工归档必须填写原因")
	}
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt != nil {
			return fail(409, "REPORT_VOIDED", "已作废档案不能归档")
		}
		details, err := listDetailsTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		cfg, hasRule := s.ruleConfigForTx(ctx, tx, tenantID, current.RuleID)
		issues := manualValidation(cfg, hasRule, current, details)
		var hard []Issue
		var soft []Issue
		for _, issue := range issues {
			if issue.Code == "FIELD_REQUIRED" || issue.Code == "FIELD_FORMAT_INVALID" {
				hard = append(hard, issue)
			} else {
				soft = append(soft, issue)
			}
		}
		if len(hard) > 0 {
			return fail(400, "VALIDATION_FAILED", "必填或格式校验未通过："+issueSummary(hard))
		}
		if len(soft) > 0 {
			missing := []string{}
			for _, issue := range soft {
				missing = append(missing, issue.Code)
			}
			for _, code := range confirmed {
				missing = removeString(missing, code)
			}
			if len(missing) > 0 {
				return fail(409, "ISSUES_NOT_ACKNOWLEDGED", "需逐项确认完整性异常："+strings.Join(missing, ", "))
			}
		}
		conflict, err := currentForKeyTx(ctx, tx, tenantID, current.Institution, current.ReportNumber, id)
		if err != nil {
			return err
		}
		if conflict != nil {
			return fail(409, "REVISION_CONFIRMATION_REQUIRED", "同机构同报告编号已有有效版本，请先确认修订关系")
		}
		before := current
		tag, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='archived', is_current=true, archived_at=now(),
			archive_reason=$3, review_stage='', review_reason='', revision=revision+1, updated_at=now()
			WHERE tenant_id=$1 AND id=$2 AND voided_at IS NULL`, tenantID, id, reason)
		if err != nil {
			if pgErrCode(err) == "23505" {
				return fail(409, "REVISION_CONFIRMATION_REQUIRED", "同机构同报告编号已有有效版本，请先确认修订关系")
			}
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "manual_archived", actor, reason, before, map[string]any{"confirmedIssues": confirmed}); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) ruleConfigForTx(ctx context.Context, tx db.DBTX, tenantID, ruleID string) (RuleConfig, bool) {
	if ruleID == "" {
		return RuleConfig{Validation: ValidationRule{RequiredFields: []string{"reportNumber", "productName", "institution", "clientUnit", "reportDate", "conclusion"}, RequireDetails: true}, Fields: map[string]FieldRule{}}, false
	}
	r, err := s.store.getRuleTx(ctx, tx, tenantID, ruleID)
	if err != nil {
		return RuleConfig{Validation: ValidationRule{RequiredFields: []string{"reportNumber", "productName", "institution", "clientUnit", "reportDate", "conclusion"}, RequireDetails: true}, Fields: map[string]FieldRule{}}, false
	}
	return r.Config, true
}

func manualValidation(cfg RuleConfig, hasRule bool, r Report, details []Detail) []Issue {
	fields := map[string]string{
		"reportNumber": r.ReportNumber, "productName": r.ProductName, "institution": r.Institution,
		"clientUnit": r.ClientUnit, "reportDate": r.ReportDate, "conclusion": r.Conclusion,
		"inspectedUnit": r.InspectedUnit, "manufacturer": r.Manufacturer, "model": r.Model,
		"category": r.Category, "basis": r.Basis, "reportName": r.ReportName,
	}
	var issues []Issue
	required := []string{"reportNumber", "productName", "institution", "clientUnit", "reportDate", "conclusion"}
	if hasRule {
		required = cfg.Validation.RequiredFields
	}
	for _, name := range required {
		if strings.TrimSpace(fields[name]) == "" || fields[name] == "/" {
			issues = append(issues, Issue{Code: "FIELD_REQUIRED", Stage: "validation", Field: name, Message: "必填字段为空：" + name})
		}
	}
	if r.ReportDate != "" {
		if _, err := normalizeDate(r.ReportDate); err != nil {
			issues = append(issues, Issue{Code: "FIELD_FORMAT_INVALID", Stage: "validation", Field: "reportDate", Message: err.Error()})
		}
	}
	if hasRule {
		if fr, ok := cfg.Fields["reportNumber"]; ok && fr.Format != "" && fr.Format != "date" {
			if re, err := compilePattern(fr.Format); err == nil && r.ReportNumber != "" && !re.MatchString(r.ReportNumber) {
				issues = append(issues, Issue{Code: "FIELD_FORMAT_INVALID", Stage: "validation", Field: "reportNumber", Message: "报告编号格式不符合规则"})
			}
		}
	}
	if len(details) == 0 {
		issues = append(issues, Issue{Code: "DETAILS_MISSING", Stage: "completeness", Message: "检测明细为空，需人工确认完整性"})
	}
	for i, d := range details {
		if strings.TrimSpace(d.ItemName) == "" || strings.TrimSpace(d.ResultRaw) == "" || strings.TrimSpace(d.Judgement) == "" {
			issues = append(issues, Issue{Code: "DETAIL_INCOMPLETE", Stage: "completeness", Message: fmt.Sprintf("第 %d 条明细缺少项目、结果或判定", i+1)})
		}
	}
	return issues
}

func issueSummary(issues []Issue) string {
	parts := make([]string, 0, len(issues))
	for _, i := range issues {
		parts = append(parts, i.Message)
	}
	return strings.Join(parts, "；")
}

func removeString(list []string, v string) []string {
	out := list[:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func listDetailsTx(ctx context.Context, tx db.DBTX, tenantID, reportID string) ([]Detail, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, section, item_name, requirement, unit, method, result_raw, judgement,
		position, source_page, source_text FROM report_details WHERE tenant_id=$1 AND report_id=$2 ORDER BY position`, tenantID, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Detail{}
	for rows.Next() {
		var d Detail
		if err := rows.Scan(&d.ID, &d.Section, &d.ItemName, &d.Requirement, &d.Unit, &d.Method, &d.ResultRaw, &d.Judgement, &d.Position, &d.SourcePage, &d.SourceText); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func currentForKeyTx(ctx context.Context, tx db.DBTX, tenantID, institution, number, exclude string) (*ReportSummary, error) {
	if institution == "" || number == "" {
		return nil, nil
	}
	var v ReportSummary
	err := tx.QueryRow(ctx, `SELECT id::text, report_number, report_name, archive_status, report_version, is_current, COALESCE(revision_of::text,''), created_at
		FROM report_reports WHERE tenant_id=$1 AND lower(institution)=lower($2) AND lower(report_number)=lower($3)
		AND is_current AND voided_at IS NULL AND id<>$4 LIMIT 1`, tenantID, institution, number, exclude).
		Scan(&v.ID, &v.ReportNumber, &v.ReportName, &v.ArchiveStatus, &v.ReportVersion, &v.IsCurrent, &v.RevisionOf, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Service) voidReport(ctx context.Context, tenantID, actor, id, reason string) (Report, error) {
	reason = trim(reason)
	if reason == "" {
		return Report{}, fail(400, "REASON_REQUIRED", "作废必须填写原因")
	}
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt != nil {
			return fail(409, "REPORT_ALREADY_VOIDED", "档案已作废")
		}
		before := current
		if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='voided', is_current=false, voided_at=now(),
			void_reason=$3, review_stage='', review_reason='', revision=revision+1, updated_at=now()
			WHERE tenant_id=$1 AND id=$2`, tenantID, id, reason); err != nil {
			return err
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "voided", actor, reason, before, map[string]string{"reason": reason}); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) restoreReport(ctx context.Context, tenantID, actor, id, reason string) (Report, error) {
	reason = trim(reason)
	if reason == "" {
		return Report{}, fail(400, "REASON_REQUIRED", "恢复必须填写原因")
	}
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt == nil {
			return fail(409, "REPORT_NOT_VOIDED", "档案未作废，无需恢复")
		}
		conflict, err := currentForKeyTx(ctx, tx, tenantID, current.Institution, current.ReportNumber, id)
		if err != nil {
			return err
		}
		if conflict != nil {
			return fail(409, "REVISION_CONFIRMATION_REQUIRED", "同机构同报告编号已有有效版本，不能直接恢复")
		}
		before := current
		if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='archived', is_current=true, voided_at=NULL,
			void_reason='', archived_at=now(), revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenantID, id); err != nil {
			if pgErrCode(err) == "23505" {
				return fail(409, "REVISION_CONFIRMATION_REQUIRED", "同机构同报告编号已有有效版本，不能直接恢复")
			}
			return err
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "restored", actor, reason, before, map[string]string{"reason": reason}); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) confirmRevision(ctx context.Context, tenantID, actor, id, reason string) (Report, error) {
	reason = trim(reason)
	if reason == "" {
		return Report{}, fail(400, "REASON_REQUIRED", "确认修订关系必须填写原因")
	}
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		candidate, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if candidate.VoidedAt != nil {
			return fail(409, "REPORT_VOIDED", "已作废档案不能确认修订")
		}
		if candidate.Institution == "" || candidate.ReportNumber == "" {
			return fail(400, "REVISION_KEY_MISSING", "确认修订前必须填写检测机构和报告编号")
		}
		conflict, err := currentForKeyTx(ctx, tx, tenantID, candidate.Institution, candidate.ReportNumber, id)
		if err != nil {
			return err
		}
		if conflict == nil {
			// No current version: this candidate simply becomes current.
			if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='archived', is_current=true, revision_of=NULL,
				report_version=1, archived_at=now(), revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenantID, id); err != nil {
				return err
			}
			if err := s.store.insertEventTx(ctx, tx, tenantID, id, "revision_confirmed", actor, reason, nil, map[string]any{"current": true}); err != nil {
				return err
			}
			out, err = s.store.getReportTx(ctx, tx, tenantID, id)
			return err
		}
		// Lock the current row so two administrators cannot both confirm the same replacement.
		var lockedID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM report_reports WHERE tenant_id=$1 AND id=$2 AND is_current FOR UPDATE`, tenantID, conflict.ID).Scan(&lockedID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(409, "REVISION_CONFLICT", "当前有效版本已被其他管理员更新，请刷新后重试")
			}
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE report_reports SET is_current=false, revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenantID, lockedID); err != nil {
			return err
		}
		version := conflict.ReportVersion + 1
		if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='archived', is_current=true, revision_of=$3,
			report_version=$4, archived_at=now(), review_stage='', review_reason='', revision=revision+1, updated_at=now()
			WHERE tenant_id=$1 AND id=$2`, tenantID, id, lockedID, version); err != nil {
			if pgErrCode(err) == "23505" {
				return fail(409, "REVISION_CONFLICT", "当前有效版本已被其他管理员更新，请刷新后重试")
			}
			return err
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, lockedID, "revision_replaced", actor, reason, nil, map[string]any{"newVersion": version}); err != nil {
			return err
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "revision_confirmed", actor, reason, nil, map[string]any{"previous": lockedID, "version": version}); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) requestReparse(ctx context.Context, tenantID, actor, id string) error {
	ctx = scope(ctx, tenantID)
	return s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt != nil {
			return fail(409, "REPORT_VOIDED", "已作废档案不能重解析")
		}
		key := fmt.Sprintf("reparse_report:%s:%d", id, time.Now().UnixNano())
		if err := s.store.enqueueJobTx(ctx, tx, tenantID, id, "reparse_report", key); err != nil {
			return err
		}
		return s.store.insertEventTx(ctx, tx, tenantID, id, "reparse_requested", actor, "管理员请求按当前规则重新解析", nil, nil)
	})
}

func (s *Service) retryOCR(ctx context.Context, tenantID, actor, id, reason string) error {
	reason = trim(reason)
	if reason == "" {
		return fail(400, "REASON_REQUIRED", "重试 OCR 必须填写处理原因")
	}
	ctx = scope(ctx, tenantID)
	return s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.VoidedAt != nil {
			return fail(409, "REPORT_VOIDED", "已作废档案不能重试 OCR")
		}
		if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='pending_ocr', review_stage='', review_reason='', revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenantID, id); err != nil {
			return err
		}
		if err := manualRetryJobTx(ctx, tx, tenantID, id); err != nil {
			key := fmt.Sprintf("process_report:%s:%d", id, time.Now().UnixNano())
			if e := s.store.enqueueJobTx(ctx, tx, tenantID, id, "process_report", key); e != nil {
				return e
			}
		}
		return s.store.insertEventTx(ctx, tx, tenantID, id, "ocr_retry", actor, reason, nil, nil)
	})
}

func (s *Service) applyCandidate(ctx context.Context, tenantID, actor, id string, expectedRevision int) (Report, error) {
	ctx = scope(ctx, tenantID)
	var out Report
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return fail(409, "REVISION_CONFLICT", "档案已被其他管理员修改，请刷新后重试")
		}
		cand, err := candidateTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if cand == nil {
			return fail(404, "CANDIDATE_NOT_FOUND", "没有待确认的重解析候选")
		}
		before, _ := json.Marshal(current)
		updated := current
		updated.ReportNumber = cand.Fields["reportNumber"]
		updated.ReportName = cand.Fields["reportName"]
		updated.ProductName = cand.Fields["productName"]
		updated.Institution = cand.Fields["institution"]
		updated.ClientUnit = cand.Fields["clientUnit"]
		updated.InspectedUnit = cand.Fields["inspectedUnit"]
		updated.Manufacturer = cand.Fields["manufacturer"]
		updated.Model = cand.Fields["model"]
		updated.Category = cand.Fields["category"]
		updated.Basis = cand.Fields["basis"]
		updated.ReportDate = cand.Fields["reportDate"]
		updated.Conclusion = cand.Fields["conclusion"]
		updated.DetectionResult = deriveDetection(updated.Conclusion)
		updated.RuleID = cand.RuleID
		updated.RuleVersion = cand.RuleVersion
		if err := s.store.updateReportTx(ctx, tx, tenantID, updated, expectedRevision); err != nil {
			return err
		}
		if err := s.store.replaceDetailsTx(ctx, tx, tenantID, id, cand.Details); err != nil {
			return err
		}
		if err := s.store.markCandidateAppliedTx(ctx, tx, tenantID, id, cand.ID, actor); err != nil {
			return err
		}
		if len(cand.Issues) > 0 {
			// A candidate with validation issues replaces the data but must go back to review; the
			// administrator cannot silently keep an archived state with unverified content.
			if _, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='needs_review', is_current=false,
				review_stage='parse', review_reason=$3, revision=revision+1, updated_at=now()
				WHERE tenant_id=$1 AND id=$2 AND voided_at IS NULL`, tenantID, id, cand.Issues[0].Message); err != nil {
				return err
			}
		}
		after, _ := json.Marshal(updated)
		if err := s.store.insertEventTx(ctx, tx, tenantID, id, "candidate_applied", actor, "管理员确认重解析候选", json.RawMessage(before), json.RawMessage(after)); err != nil {
			return err
		}
		out, err = s.store.getReportTx(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *Service) candidate(ctx context.Context, tenantID, reportID string) (*candidateParse, error) {
	return s.store.candidate(ctx, tenantID, reportID)
}

func (s *Service) validationPreview(ctx context.Context, tenantID, reportID string) ([]Issue, error) {
	r, err := s.store.getReport(ctx, tenantID, reportID)
	if err != nil {
		return nil, err
	}
	ctx = scope(ctx, tenantID)
	var issues []Issue
	err = s.store.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		cfg, hasRule := s.ruleConfigForTx(ctx, tx, tenantID, r.RuleID)
		details, e := listDetailsTx(ctx, tx, tenantID, reportID)
		if e != nil {
			return e
		}
		issues = manualValidation(cfg, hasRule, r, details)
		return nil
	})
	return issues, err
}
