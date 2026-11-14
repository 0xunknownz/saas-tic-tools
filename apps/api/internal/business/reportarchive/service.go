package reportarchive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/jackc/pgx/v5"
)

// Service owns report archive use cases and the durable processing loop. The loop runs in both the
// API and the worker composition; PostgreSQL SKIP LOCKED makes that safe.
type Service struct {
	cfg    Config
	store  *store
	ocr    ocrProvider
	worker string

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	sem    chan struct{}
	seq    atomic.Uint64
}

func NewService(cfg Config, deps Deps) (*Service, error) {
	cfg = cfg.withDefaults()
	if deps.Db == nil {
		return nil, errors.New("reportarchive: database is required")
	}
	host, _ := os.Hostname()
	return &Service{cfg: cfg, store: newStore(deps), ocr: newOCRProvider(cfg), worker: fmt.Sprintf("%s-%d", host, os.Getpid()), sem: make(chan struct{}, cfg.OCRMaxConcurrency)}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.done != nil {
		s.mu.Unlock()
		return nil
	}
	loop, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	s.mu.Unlock()

	if n, err := s.store.resetStaleJobs(loop); err != nil {
		slog.WarnContext(loop, "report archive: reset stale jobs", "error", err)
	} else if n > 0 {
		slog.InfoContext(loop, "report archive: recovered stale jobs", "count", n)
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Duration(s.cfg.PollMilliseconds) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-loop.Done():
				return
			case <-ticker.C:
				s.drain(loop)
			}
		}
	}()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) drain(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		j, ok, err := s.store.claimJob(ctx, s.worker)
		if err != nil {
			slog.WarnContext(ctx, "report archive: claim job", "error", err)
			break
		}
		if !ok {
			break
		}
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			s.runJob(ctx, j)
		}(j)
	}
	wg.Wait()
}

func (s *Service) runJob(ctx context.Context, j job) {
	defer func() {
		if r := recover(); r != nil {
			_ = s.store.failJob(ctx, j, "internal", fmt.Sprintf("处理进程异常: %v", r))
			s.setReview(ctx, j, "internal", "处理进程异常，需管理员重试")
		}
	}()
	var err error
	switch j.Kind {
	case "process_report":
		err = s.processReport(ctx, j)
	case "reparse_report":
		err = s.reparseReport(ctx, j)
	default:
		err = fmt.Errorf("unknown job kind %q", j.Kind)
	}
	if err != nil {
		s.handleJobError(ctx, j, err)
	}
}

type jobFailure struct {
	stage     string
	code      string
	message   string
	retryable bool
}

func (e *jobFailure) Error() string { return e.code + ": " + e.message }

func (s *Service) handleJobError(ctx context.Context, j job, err error) {
	var f *jobFailure
	if !errors.As(err, &f) {
		f = &jobFailure{stage: "internal", code: "INTERNAL", message: err.Error()}
	}
	if f.retryable && j.Attempts < j.MaxAttempts {
		delay := time.Duration(s.cfg.RetryBaseSeconds) * time.Second * time.Duration(1<<(j.Attempts-1))
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		_ = s.store.retryJob(ctx, j, f.stage, f.message, delay)
		s.setReview(ctx, j, f.stage, f.message)
		return
	}
	_ = s.store.failJob(ctx, j, f.stage, f.message)
	s.setReview(ctx, j, f.stage, f.message)
}

func (s *Service) setReview(ctx context.Context, j job, stage, reason string) {
	ctx = scope(ctx, j.TenantID)
	_ = s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		tag, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='needs_review', review_stage=$3, review_reason=$4,
			revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2 AND voided_at IS NULL AND archive_status <> 'archived'`,
			j.TenantID, j.ReportID, stage, reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			return s.store.insertEventTx(ctx, tx, j.TenantID, j.ReportID, "processing_failed", "system", reason, nil, map[string]string{"stage": stage})
		}
		return nil
	})
}

func (s *Service) processReport(ctx context.Context, j job) error {
	report, err := s.store.getReport(ctx, j.TenantID, j.ReportID)
	if err != nil {
		return fmt.Errorf("load report: %w", err)
	}
	if report.VoidedAt != nil || report.ArchiveStatus == "archived" {
		return s.store.finishJob(ctx, j)
	}
	ocr, _ := s.store.latestOCRRun(ctx, j.TenantID, j.ReportID)
	if ocr == nil || ocr.Status != "succeeded" {
		file, content, err := s.store.getFile(ctx, j.TenantID, report.File.ID)
		if err != nil {
			return &jobFailure{stage: "ocr", code: "FILE_MISSING", message: "原件读取失败：" + err.Error(), retryable: false}
		}
		ocrRunID, err := s.createOCRRun(ctx, j, report)
		if err != nil {
			return err
		}
		ocrCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.OCRTimeoutSeconds)*time.Second)
		result, oerr := s.ocr.Extract(ocrCtx, file.FileName, content, file.Pages)
		cancel()
		if oerr != nil {
			var f *ocrFailure
			if errors.As(oerr, &f) {
				_ = s.finishOCRRunFailed(ctx, j, ocrRunID, f.Code, f.Message)
				if !f.Retryable {
					return &jobFailure{stage: "ocr", code: f.Code, message: f.Message, retryable: false}
				}
				return &jobFailure{stage: "ocr", code: f.Code, message: f.Message, retryable: true}
			}
			_ = s.finishOCRRunFailed(ctx, j, ocrRunID, "OCR_ERROR", oerr.Error())
			return &jobFailure{stage: "ocr", code: "OCR_ERROR", message: oerr.Error(), retryable: true}
		}
		if err := s.finishOCRRun(ctx, j, ocrRunID, result); err != nil {
			return err
		}
		ocr = &OCRRun{ID: ocrRunID, Status: "succeeded", Provider: result.Provider, Model: result.Model, Pages: result.Pages, Text: ocrPageText(result.Pages), Raw: result.Raw}
	}

	rules, err := s.store.activeRules(ctx, j.TenantID)
	if err != nil {
		return &jobFailure{stage: "parse", code: "RULES_LOAD", message: "模板规则读取失败", retryable: true}
	}
	rule, issues := chooseRule(rules, ocr.Text)
	if rule == nil {
		return s.finishReview(ctx, j, report, nil, issues, ocr.ID, "")
	}
	result := parseDocument(rule, ocr.Pages)
	result.Issues = append(result.Issues, issues...)
	result.Issues = append(result.Issues, validateParsed(rule.Config, result)...)
	if result.Fields["reportName"] == "" && result.Fields["productName"] != "" {
		result.Fields["reportName"] = result.Fields["productName"] + "检测报告"
	}

	// Duplicate report number/institution with a different file is a revision candidate, never an
	// automatic replacement.
	if len(result.Issues) == 0 && result.Fields["reportNumber"] != "" && result.Fields["institution"] != "" {
		dup, _ := s.findCurrentConflict(ctx, j.TenantID, result.Fields["institution"], result.Fields["reportNumber"], report.File.SHA256)
		if dup != nil {
			result.Issues = append(result.Issues, Issue{Code: "REPORT_NUMBER_DUPLICATE", Stage: "duplicate", Message: "同机构同报告编号已有有效版本，需管理员确认修订关系"})
		}
	}
	return s.finishParse(ctx, j, report, rule, result, ocr.ID)
}

func validateParsed(cfg RuleConfig, res ParseResult) []Issue {
	var issues []Issue
	required := append([]string{}, cfg.Validation.RequiredFields...)
	for name, f := range cfg.Fields {
		if f.Required && !contains(required, name) {
			required = append(required, name)
		}
	}
	for _, name := range required {
		value := strings.TrimSpace(res.Fields[name])
		if value == "" || value == "/" || value == "—" || value == "-" {
			issues = append(issues, Issue{Code: "FIELD_REQUIRED", Stage: "validation", Field: name, Message: "必填字段为空：" + name})
			continue
		}
		if fr, ok := cfg.Fields[name]; ok && fr.Format == "date" {
			if _, err := normalizeDate(value); err != nil {
				issues = append(issues, Issue{Code: "FIELD_FORMAT_INVALID", Stage: "validation", Field: name, Message: err.Error()})
			}
		}
	}
	if cfg.Validation.RequireDetails && len(res.Details) == 0 {
		issues = append(issues, Issue{Code: "DETAILS_MISSING", Stage: "validation", Message: "检测明细为空"})
	}
	return issues
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Service) findCurrentConflict(ctx context.Context, tenantID, institution, number, sha string) (*ReportSummary, error) {
	ctx = scope(ctx, tenantID)
	var out *ReportSummary
	err := s.store.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		var v ReportSummary
		err := tx.QueryRow(ctx, `SELECT r.id::text, r.report_number, r.report_name, r.archive_status, r.report_version, r.is_current, COALESCE(r.revision_of::text,''), r.created_at
			FROM report_reports r JOIN report_files f ON f.id=r.file_id
			WHERE r.tenant_id=$1 AND lower(r.institution)=lower($2) AND lower(r.report_number)=lower($3)
			AND r.is_current AND r.voided_at IS NULL AND f.sha256 <> $4 LIMIT 1`, tenantID, institution, number, sha).
			Scan(&v.ID, &v.ReportNumber, &v.ReportName, &v.ArchiveStatus, &v.ReportVersion, &v.IsCurrent, &v.RevisionOf, &v.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = &v
		return nil
	})
	return out, err
}

func (s *Service) createOCRRun(ctx context.Context, j job, report Report) (string, error) {
	ctx = scope(ctx, j.TenantID)
	var id string
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		id, e = s.store.insertOCRRun(ctx, tx, j.TenantID, j.ReportID, "running", s.cfg.OCRProvider, s.cfg.OCRModel)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE report_reports SET archive_status='ocr_running', ocr_run_id=$3, updated_at=now() WHERE tenant_id=$1 AND id=$2`, j.TenantID, j.ReportID, id)
		return e
	})
	return id, err
}

func (s *Service) finishOCRRun(ctx context.Context, j job, id string, result ocrResult) error {
	ctx = scope(ctx, j.TenantID)
	return s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		if err := s.store.finishOCRRunTx(ctx, tx, j.TenantID, id, result); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE report_reports SET ocr_run_id=$3, archive_status='parsing', updated_at=now() WHERE tenant_id=$1 AND id=$2`, j.TenantID, j.ReportID, id)
		return err
	})
}

func (s *Service) finishOCRRunFailed(ctx context.Context, j job, id, code, message string) error {
	ctx = scope(ctx, j.TenantID)
	return s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		return s.store.failOCRRunTx(ctx, tx, j.TenantID, id, code, message)
	})
}

func (s *Service) finishReview(ctx context.Context, j job, report Report, rule *Rule, issues []Issue, ocrRunID, parseRunID string) error {
	return s.finishParse(ctx, j, report, rule, ParseResult{Issues: issues, Fields: map[string]string{}, Details: []Detail{}}, ocrRunID)
}

func (s *Service) finishParse(ctx context.Context, j job, report Report, rule *Rule, result ParseResult, ocrRunID string) error {
	ctx = scope(ctx, j.TenantID)
	fields := result.Fields
	status := "archived"
	stage, reason := "", ""
	if len(result.Issues) > 0 {
		status = "needs_review"
		stage = result.Issues[0].Stage
		reason = result.Issues[0].Message
	}
	report.ReportNumber = fields["reportNumber"]
	report.ReportName = fields["reportName"]
	report.ProductName = fields["productName"]
	report.Institution = fields["institution"]
	report.ClientUnit = fields["clientUnit"]
	report.InspectedUnit = fields["inspectedUnit"]
	report.Manufacturer = fields["manufacturer"]
	report.Model = fields["model"]
	report.Category = fields["category"]
	report.Basis = fields["basis"]
	report.ReportDate = fields["reportDate"]
	report.Conclusion = fields["conclusion"]
	report.DetectionResult = deriveDetection(fields["conclusion"])
	report.FieldSources = result.FieldSources
	report.RawFields = result.RawFields
	report.RuleID = result.RuleID
	report.RuleVersion = result.RuleVersion

	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		current, err := s.store.reportForUpdate(ctx, tx, j.TenantID, j.ReportID)
		if err != nil {
			return err
		}
		report.Revision = current.Revision
		parseRunID, err := s.store.insertParseRunTx(ctx, tx, j.TenantID, j.ReportID, ocrRunID, result, "succeeded")
		if err != nil {
			return err
		}
		report.ParseRunID = parseRunID
		if err := s.store.updateReportTx(ctx, tx, j.TenantID, report, current.Revision); err != nil {
			return err
		}
		if err := s.store.replaceDetailsTx(ctx, tx, j.TenantID, j.ReportID, result.Details); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE report_reports SET parse_run_id=$3, ocr_run_id=$4 WHERE tenant_id=$1 AND id=$2`, j.TenantID, j.ReportID, parseRunID, ocrRunID)
		if err != nil {
			return err
		}
		if status == "archived" {
			// Claim the current slot only after all validation passed. A concurrent claimant hits the
			// partial unique index; a savepoint lets the transaction continue after that expected error.
			if _, err := tx.Exec(ctx, "SAVEPOINT archive_claim"); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='archived', is_current=true, archived_at=now(),
				review_stage='', review_reason='', revision=revision+1, updated_at=now()
				WHERE tenant_id=$1 AND id=$2 AND voided_at IS NULL`, j.TenantID, j.ReportID)
			if err != nil {
				_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT archive_claim")
				if pgErrCode(err) == "23505" {
					_, _ = tx.Exec(ctx, `UPDATE report_reports SET archive_status='needs_review', is_current=false, review_stage='duplicate',
						review_reason='同机构同报告编号已有有效版本，需管理员确认修订关系', revision=revision+1, updated_at=now()
						WHERE tenant_id=$1 AND id=$2`, j.TenantID, j.ReportID)
					return s.store.insertEventTx(ctx, tx, j.TenantID, j.ReportID, "duplicate_conflict", "system", "并发归档冲突", nil, nil)
				}
				return err
			}
			if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT archive_claim"); err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrNotFound
			}
			return s.store.insertEventTx(ctx, tx, j.TenantID, j.ReportID, "auto_archived", "system", "自动归档", nil, map[string]any{"ruleId": result.RuleID, "ruleVersion": result.RuleVersion})
		}
		tag, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status='needs_review', is_current=false, review_stage=$3,
			review_reason=$4, revision=revision+1, updated_at=now() WHERE tenant_id=$1 AND id=$2 AND voided_at IS NULL`,
			j.TenantID, j.ReportID, stage, reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return s.store.insertEventTx(ctx, tx, j.TenantID, j.ReportID, "parse_needs_review", "system", reason, nil, map[string]any{"issues": result.Issues})
	})
	if err != nil {
		return err
	}
	return s.store.finishJob(ctx, j)
}

func deriveDetection(conclusion string) string {
	switch {
	case strings.Contains(conclusion, "不合格"):
		return "unqualified"
	case strings.Contains(conclusion, "合格"):
		return "qualified"
	default:
		return "unknown"
	}
}

func (s *Service) reparseReport(ctx context.Context, j job) error {
	report, err := s.store.getReport(ctx, j.TenantID, j.ReportID)
	if err != nil {
		return &jobFailure{stage: "parse", code: "REPORT_MISSING", message: "档案不存在", retryable: false}
	}
	if report.VoidedAt != nil {
		return &jobFailure{stage: "parse", code: "REPORT_VOIDED", message: "已作废档案不能重解析", retryable: false}
	}
	ocr, _ := s.store.latestOCRRun(ctx, j.TenantID, j.ReportID)
	if ocr == nil || ocr.Status != "succeeded" || len(ocr.Pages) == 0 {
		return &jobFailure{stage: "parse", code: "OCR_MISSING", message: "没有可用的 OCR 结果，请先重试 OCR", retryable: false}
	}
	rules, err := s.store.activeRules(ctx, j.TenantID)
	if err != nil {
		return &jobFailure{stage: "parse", code: "RULES_LOAD", message: "模板规则读取失败", retryable: true}
	}
	rule, issues := chooseRule(rules, ocr.Text)
	if rule == nil {
		return &jobFailure{stage: "parse", code: "RULE_NOT_MATCHED", message: issues[0].Message, retryable: false}
	}
	result := parseDocument(rule, ocr.Pages)
	result.Issues = append(result.Issues, validateParsed(rule.Config, result)...)
	ctx = scope(ctx, j.TenantID)
	err = s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := s.store.insertParseRunTx(ctx, tx, j.TenantID, j.ReportID, ocr.ID, result, "candidate")
		if err != nil {
			return err
		}
		return s.store.insertEventTx(ctx, tx, j.TenantID, j.ReportID, "reparse_candidate", "system", "规则重解析生成候选", nil, map[string]any{"issues": result.Issues})
	})
	if err != nil {
		return err
	}
	return s.store.finishJob(ctx, j)
}

func (s *Service) findReportJob(ctx context.Context, tenantID, reportID, kind string) error {
	ctx = scope(ctx, tenantID)
	return s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		key := fmt.Sprintf("%s:%s:%d", kind, reportID, time.Now().UnixNano())
		return s.store.enqueueJobTx(ctx, tx, tenantID, reportID, kind, key)
	})
}
