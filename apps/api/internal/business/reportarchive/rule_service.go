package reportarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/jackc/pgx/v5"
)

type RuleInput struct {
	TemplateKey string          `json:"templateKey"`
	Name        string          `json:"name"`
	Config      json.RawMessage `json:"config,omitempty"`
	CopyFrom    string          `json:"copyFrom,omitempty"`
}

type TrialInput struct {
	Text     string `json:"text"`
	ReportID string `json:"reportId"`
}

type TrialResult struct {
	RuleID      string            `json:"ruleId"`
	RuleVersion int               `json:"ruleVersion"`
	Fields      map[string]string `json:"fields"`
	Details     []Detail          `json:"details"`
	Issues      []Issue           `json:"issues"`
}

func (s *Service) listRules(ctx context.Context, tenantID string) ([]Rule, error) {
	return s.store.listRules(ctx, tenantID)
}

func (s *Service) createRuleVersion(ctx context.Context, tenantID, actor string, in RuleInput) (Rule, error) {
	var cfg RuleConfig
	if len(in.Config) > 0 {
		decoded, err := DecodeRuleConfig(in.Config)
		if err != nil {
			return Rule{}, err
		}
		cfg = decoded
	} else if in.CopyFrom != "" {
		source, err := s.store.getRule(ctx, tenantID, in.CopyFrom)
		if err != nil {
			return Rule{}, err
		}
		cfg = source.Config
	} else {
		return Rule{}, fmt.Errorf("%w: config or copyFrom is required", ErrInvalidInput)
	}
	if strings.TrimSpace(in.TemplateKey) == "" {
		in.TemplateKey = cfg.TemplateKey
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = cfg.Name
	}
	cfg.TemplateKey, cfg.Name = trim(in.TemplateKey), trim(in.Name)
	if err := validateRuleConfig(cfg); err != nil {
		return Rule{}, err
	}
	ctx = scope(ctx, tenantID)
	var out Rule
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		out, e = s.store.insertRuleVersionTx(ctx, tx, tenantID, cfg.TemplateKey, cfg.Name, actor, cfg)
		if e != nil {
			return e
		}
		return s.store.audit.WriteTx(ctx, tx, audit.Entry{Realm: "tenant", Actor: actor, Action: "reportarchive.rule_draft",
			Target: "rule:" + out.ID, Outcome: "success", Metadata: map[string]any{"tenant": tenantID, "version": out.Version}})
	})
	return out, err
}

func (s *Service) activateRule(ctx context.Context, tenantID, actor, id string) (Rule, error) {
	ctx = scope(ctx, tenantID)
	var out Rule
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		r, err := s.store.getRuleTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if r.Builtin || r.TenantID != tenantID {
			return fail(403, "RULE_READ_ONLY", "内置模板只能由平台管理员维护")
		}
		if r.Status == "active" {
			out = r
			return nil
		}
		var e error
		out, e = s.store.activateRuleTx(ctx, tx, tenantID, id)
		if e != nil {
			return e
		}
		return s.store.audit.WriteTx(ctx, tx, audit.Entry{Realm: "tenant", Actor: actor, Action: "reportarchive.rule_activated",
			Target: "rule:" + id, Outcome: "success", Metadata: map[string]any{"tenant": tenantID, "version": r.Version, "template": r.TemplateKey}})
	})
	return out, err
}

func (s *Service) trialRule(ctx context.Context, tenantID, actor, ruleID string, in TrialInput) (TrialResult, error) {
	rule, err := s.store.getRule(ctx, tenantID, ruleID)
	if err != nil {
		return TrialResult{}, err
	}
	var pages []OCRPage
	if in.ReportID != "" {
		if _, err := s.store.getReport(ctx, tenantID, in.ReportID); err != nil {
			return TrialResult{}, err
		}
		ocr, err := s.store.latestOCRRun(ctx, tenantID, in.ReportID)
		if err != nil {
			return TrialResult{}, err
		}
		if ocr == nil || ocr.Status != "succeeded" || len(ocr.Pages) == 0 {
			return TrialResult{}, fail(409, "OCR_MISSING", "该档案没有可用的 OCR 结果")
		}
		pages = ocr.Pages
	} else {
		text := in.Text
		if strings.TrimSpace(text) == "" {
			return TrialResult{}, fmt.Errorf("%w: text or reportId is required", ErrInvalidInput)
		}
		if len(text) > 2<<20 {
			return TrialResult{}, fmt.Errorf("%w: trial text exceeds 2 MB", ErrInvalidInput)
		}
		pages = []OCRPage{{Page: 1, Text: text}}
	}
	result := parseDocument(&rule, pages)
	result.Issues = append(result.Issues, validateParsed(rule.Config, result)...)
	out := TrialResult{RuleID: rule.ID, RuleVersion: rule.Version, Fields: result.Fields, Details: result.Details, Issues: result.Issues}
	if !rule.Builtin {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%v", pages)))
		_ = s.store.insertTrial(ctx, tenantID, rule.ID, actor, hex.EncodeToString(hash[:]), out)
	}
	return out, nil
}

func (s *Service) builtinRules(ctx context.Context) ([]Rule, error) {
	out := []Rule{}
	err := s.store.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		rows, err := tx.Query(ctx, `SELECT id::text, template_key, name, version, status, config, created_by, created_at, activated_at, retired_at
			FROM report_rules WHERE tenant_id IS NULL ORDER BY template_key, version DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r Rule
			var raw []byte
			if err := rows.Scan(&r.ID, &r.TemplateKey, &r.Name, &r.Version, &r.Status, &raw, &r.CreatedBy, &r.CreatedAt, &r.ActivatedAt, &r.RetiredAt); err != nil {
				return err
			}
			_ = json.Unmarshal(raw, &r.Config)
			r.Builtin = true
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) createBuiltinRule(ctx context.Context, actor string, in RuleInput) (Rule, error) {
	if len(in.Config) == 0 {
		return Rule{}, fmt.Errorf("%w: config is required", ErrInvalidInput)
	}
	cfg, err := DecodeRuleConfig(in.Config)
	if err != nil {
		return Rule{}, err
	}
	if in.TemplateKey != "" {
		cfg.TemplateKey = trim(in.TemplateKey)
	}
	if in.Name != "" {
		cfg.Name = trim(in.Name)
	}
	if err := validateRuleConfig(cfg); err != nil {
		return Rule{}, err
	}
	var out Rule
	err = s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		var version int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM report_rules WHERE tenant_id IS NULL AND template_key=$1`, cfg.TemplateKey).Scan(&version); err != nil {
			return err
		}
		raw, _ := json.Marshal(cfg)
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO report_rules (tenant_id, template_key, name, version, status, config, created_by)
			VALUES (NULL,$1,$2,$3,'draft',$4,$5) RETURNING id::text`, cfg.TemplateKey, cfg.Name, version, raw, actor).Scan(&id); err != nil {
			return err
		}
		out, err = s.store.getRuleTx(ctx, tx, "", id)
		return err
	})
	return out, err
}

func (s *Service) activateBuiltinRule(ctx context.Context, actor, id string) (Rule, error) {
	var out Rule
	err := s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		var template string
		var status string
		if err := tx.QueryRow(ctx, `SELECT template_key, status FROM report_rules WHERE id=$1 AND tenant_id IS NULL FOR UPDATE`, id).Scan(&template, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE report_rules SET status='retired', retired_at=now()
			WHERE tenant_id IS NULL AND template_key=$1 AND status='active' AND id<>$2`, template, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE report_rules SET status='active', activated_at=now(), retired_at=NULL WHERE id=$1 AND tenant_id IS NULL`, id); err != nil {
			return err
		}
		var e error
		out, e = s.store.getRuleTx(ctx, tx, "", id)
		return e
	})
	return out, err
}

func (s *Service) touchRule(_ context.Context, _ string) {}

var _ = time.Now
