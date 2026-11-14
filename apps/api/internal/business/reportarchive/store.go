package reportarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
)

type store struct {
	db    db.Transactor
	audit audit.Writer
	now   func() time.Time
}

func newStore(deps Deps) *store { return &store{db: deps.Db, audit: deps.Audit, now: time.Now} }

func scope(ctx context.Context, tenantID string) context.Context {
	return db.WithScope(ctx, db.Scope{Realm: "tenant", TenantID: tenantID})
}

const reportColumns = `r.id::text, r.report_number, r.report_name, r.product_name, r.institution, r.client_unit,
	r.inspected_unit, r.manufacturer, r.model, r.category, r.basis, COALESCE(r.report_date::text, ''),
	r.conclusion, r.detection_result, r.archive_status, r.review_stage, r.review_reason, r.archive_reason,
	r.report_version, r.revision, COALESCE(r.revision_of::text, ''), COALESCE(r.duplicate_of::text, ''), r.is_current,
	COALESCE(r.rule_id::text, ''), COALESCE(r.rule_version, 0), r.field_sources, r.raw_fields,
	r.created_by, r.created_at, r.updated_at, r.archived_at, r.voided_at, r.void_reason,
	f.id::text, f.file_name, f.content_type, f.size, f.pages, f.sha256, f.created_at`

func scanReport(row pgx.Row) (Report, error) {
	var r Report
	var sources, raw []byte
	var archived, voided *time.Time
	r.File = &FileInfo{}
	err := row.Scan(&r.ID, &r.ReportNumber, &r.ReportName, &r.ProductName, &r.Institution, &r.ClientUnit,
		&r.InspectedUnit, &r.Manufacturer, &r.Model, &r.Category, &r.Basis, &r.ReportDate,
		&r.Conclusion, &r.DetectionResult, &r.ArchiveStatus, &r.ReviewStage, &r.ReviewReason, &r.ArchiveReason,
		&r.ReportVersion, &r.Revision, &r.RevisionOf, &r.DuplicateOf, &r.IsCurrent,
		&r.RuleID, &r.RuleVersion, &sources, &raw,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &archived, &voided, &r.VoidReason,
		&r.File.ID, &r.File.FileName, &r.File.ContentType, &r.File.Size, &r.File.Pages, &r.File.SHA256, &r.File.CreatedAt)
	if err != nil {
		return r, err
	}
	r.ArchivedAt, r.VoidedAt = archived, voided
	if len(sources) > 0 {
		_ = json.Unmarshal(sources, &r.FieldSources)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r.RawFields)
	}
	if r.FieldSources == nil {
		r.FieldSources = map[string]FieldSource{}
	}
	if r.RawFields == nil {
		r.RawFields = map[string]any{}
	}
	return r, nil
}

func (s *store) getReport(ctx context.Context, tenantID, id string) (Report, error) {
	if !validUUID(id) {
		return Report{}, ErrNotFound
	}
	ctx = scope(ctx, tenantID)
	var r Report
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		r, e = s.getReportTx(ctx, tx, tenantID, id)
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *store) getReportTx(ctx context.Context, tx db.DBTX, tenantID, id string) (Report, error) {
	r, err := scanReport(tx.QueryRow(ctx, `SELECT `+reportColumns+` FROM report_reports r JOIN report_files f ON f.id = r.file_id AND f.tenant_id = r.tenant_id
		WHERE r.tenant_id = $1 AND r.id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *store) reportForUpdate(ctx context.Context, tx db.DBTX, tenantID, id string) (Report, error) {
	if !validUUID(id) {
		return Report{}, ErrNotFound
	}
	return scanReport(tx.QueryRow(ctx, `SELECT `+reportColumns+` FROM report_reports r JOIN report_files f ON f.id = r.file_id AND f.tenant_id = r.tenant_id
		WHERE r.tenant_id = $1 AND r.id = $2 FOR UPDATE OF r`, tenantID, id))
}

func (s *store) findReportByFile(ctx context.Context, tenantID, fileID string) (Report, error) {
	ctx = scope(ctx, tenantID)
	var r Report
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		r, e = scanReport(tx.QueryRow(ctx, `SELECT `+reportColumns+` FROM report_reports r JOIN report_files f ON f.id = r.file_id AND f.tenant_id = r.tenant_id
			WHERE r.tenant_id = $1 AND r.file_id = $2 ORDER BY r.created_at DESC LIMIT 1`, tenantID, fileID))
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *store) findFileBySHA(ctx context.Context, tenantID, sha string) (FileInfo, error) {
	ctx = scope(ctx, tenantID)
	var f FileInfo
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT id::text, file_name, content_type, size, pages, sha256, created_at
			FROM report_files WHERE tenant_id = $1 AND sha256 = $2`, tenantID, sha).
			Scan(&f.ID, &f.FileName, &f.ContentType, &f.Size, &f.Pages, &f.SHA256, &f.CreatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func (s *store) insertFileTx(ctx context.Context, tx db.DBTX, tenantID string, f FileInfo, content []byte, actor string) (FileInfo, bool, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO report_files (tenant_id, sha256, file_name, content_type, size, pages, content, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (tenant_id, sha256) DO NOTHING RETURNING id::text`,
		tenantID, f.SHA256, f.FileName, f.ContentType, f.Size, f.Pages, content, actor).Scan(&id)
	if err == nil {
		f.ID = id
		return f, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return f, false, err
	}
	var existing FileInfo
	err = tx.QueryRow(ctx, `SELECT id::text, file_name, content_type, size, pages, sha256, created_at
		FROM report_files WHERE tenant_id = $1 AND sha256 = $2`, tenantID, f.SHA256).
		Scan(&existing.ID, &existing.FileName, &existing.ContentType, &existing.Size, &existing.Pages, &existing.SHA256, &existing.CreatedAt)
	if err != nil {
		return f, false, err
	}
	return existing, true, nil
}

func (s *store) getFile(ctx context.Context, tenantID, id string) (FileInfo, []byte, error) {
	if !validUUID(id) {
		return FileInfo{}, nil, ErrNotFound
	}
	ctx = scope(ctx, tenantID)
	var f FileInfo
	var content []byte
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT id::text, file_name, content_type, size, pages, sha256, created_at, content
			FROM report_files WHERE tenant_id = $1 AND id = $2`, tenantID, id).
			Scan(&f.ID, &f.FileName, &f.ContentType, &f.Size, &f.Pages, &f.SHA256, &f.CreatedAt, &content)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil, ErrNotFound
	}
	return f, content, err
}

type reportInsert struct {
	Report
	TenantID string
	FileID   string
}

func (s *store) insertReportTx(ctx context.Context, tx db.DBTX, in reportInsert) (Report, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO report_reports
		(tenant_id, file_id, report_number, report_name, product_name, institution, client_unit, inspected_unit,
		 manufacturer, model, category, basis, report_date, conclusion, detection_result, archive_status,
		 review_stage, review_reason, archive_reason, report_version, revision_of, duplicate_of, is_current,
		 rule_id, rule_version, field_sources, raw_fields, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,'')::date,$14,$15,$16,$17,$18,$19,$20,NULLIF($21,'')::uuid,NULLIF($22,'')::uuid,$23,NULLIF($24,'')::uuid,NULLIF($25,0),$26,$27,$28)
		RETURNING id::text`,
		in.TenantID, in.FileID, in.ReportNumber, in.ReportName, in.ProductName, in.Institution, in.ClientUnit, in.InspectedUnit,
		in.Manufacturer, in.Model, in.Category, in.Basis, in.ReportDate, in.Conclusion, in.DetectionResult, in.ArchiveStatus,
		in.ReviewStage, in.ReviewReason, in.ArchiveReason, in.ReportVersion, in.RevisionOf, in.DuplicateOf, in.IsCurrent,
		in.RuleID, in.RuleVersion, jsonOrEmpty(in.FieldSources), jsonOrEmpty(in.RawFields), in.CreatedBy).Scan(&id)
	if err != nil {
		return Report{}, err
	}
	return s.getReportTx(ctx, tx, in.TenantID, id)
}

func jsonOrEmpty(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(v)
	if len(b) == 0 || string(b) == "null" {
		return []byte("{}")
	}
	return b
}

func (s *store) updateReportTx(ctx context.Context, tx db.DBTX, tenantID string, r Report, expectedRevision int) error {
	sources, _ := json.Marshal(r.FieldSources)
	raw, _ := json.Marshal(r.RawFields)
	tag, err := tx.Exec(ctx, `UPDATE report_reports SET report_number=$3, report_name=$4, product_name=$5, institution=$6,
		client_unit=$7, inspected_unit=$8, manufacturer=$9, model=$10, category=$11, basis=$12,
		report_date=NULLIF($13,'')::date, conclusion=$14, detection_result=$15, field_sources=$16, raw_fields=$17,
		rule_id=NULLIF($19,'')::uuid, rule_version=NULLIF($20,0),
		revision=revision+1, updated_at=now()
		WHERE tenant_id=$1 AND id=$2 AND revision=$18 AND voided_at IS NULL`,
		tenantID, r.ID, r.ReportNumber, r.ReportName, r.ProductName, r.Institution, r.ClientUnit, r.InspectedUnit,
		r.Manufacturer, r.Model, r.Category, r.Basis, r.ReportDate, r.Conclusion, r.DetectionResult, sources, raw, expectedRevision, r.RuleID, r.RuleVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (s *store) setReportStateTx(ctx context.Context, tx db.DBTX, tenantID, id string, status, stage, reason string, current bool, archived, voided bool, voidReason string) error {
	tag, err := tx.Exec(ctx, `UPDATE report_reports SET archive_status=$3, review_stage=$4, review_reason=$5,
		archive_reason=CASE WHEN $6<>'' THEN $6 ELSE archive_reason END,
		is_current=$7, archived_at=CASE WHEN $8 THEN COALESCE(archived_at, now()) ELSE archived_at END,
		voided_at=CASE WHEN $9 THEN COALESCE(voided_at, now()) ELSE voided_at END,
		void_reason=CASE WHEN $9 THEN $10 ELSE void_reason END,
		revision=revision+1, updated_at=now()
		WHERE tenant_id=$1 AND id=$2`, tenantID, id, status, stage, reason, reason, current, archived, voided, voidReason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) replaceDetailsTx(ctx context.Context, tx db.DBTX, tenantID, reportID string, details []Detail) error {
	if _, err := tx.Exec(ctx, `DELETE FROM report_details WHERE tenant_id=$1 AND report_id=$2`, tenantID, reportID); err != nil {
		return err
	}
	for i := range details {
		d := details[i]
		position := d.Position
		if position < 0 {
			position = i
		}
		if d.SourcePage < 1 {
			d.SourcePage = 1
		}
		if _, err := tx.Exec(ctx, `INSERT INTO report_details
			(report_id, tenant_id, section, item_name, requirement, unit, method, result_raw, judgement, position, source_page, source_text)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			reportID, tenantID, d.Section, d.ItemName, d.Requirement, d.Unit, d.Method, d.ResultRaw, d.Judgement, position, d.SourcePage, d.SourceText); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) listDetails(ctx context.Context, tenantID, reportID string) ([]Detail, error) {
	ctx = scope(ctx, tenantID)
	out := []Detail{}
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		rows, err := tx.Query(ctx, `SELECT id::text, section, item_name, requirement, unit, method, result_raw, judgement,
			position, source_page, source_text FROM report_details WHERE tenant_id=$1 AND report_id=$2 ORDER BY position`, tenantID, reportID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d Detail
			if err := rows.Scan(&d.ID, &d.Section, &d.ItemName, &d.Requirement, &d.Unit, &d.Method, &d.ResultRaw, &d.Judgement, &d.Position, &d.SourcePage, &d.SourceText); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (s *store) insertEventTx(ctx context.Context, tx db.DBTX, tenantID, reportID, action, actor, reason string, before, after any) error {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	if len(b) == 0 || string(b) == "null" {
		b = nil
	}
	if len(a) == 0 || string(a) == "null" {
		a = nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO report_events (report_id, tenant_id, action, actor, reason, before, after)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, reportID, tenantID, action, actor, reason, b, a)
	if err != nil {
		return err
	}
	return s.audit.WriteTx(ctx, tx, audit.Entry{Realm: "tenant", Actor: actor, Action: "reportarchive." + action,
		Target: "report:" + reportID, Outcome: "success", Metadata: map[string]any{"tenant": tenantID, "reason": reason}})
}

func (s *store) listEvents(ctx context.Context, tenantID, reportID string) ([]Event, error) {
	ctx = scope(ctx, tenantID)
	out := []Event{}
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		rows, err := tx.Query(ctx, `SELECT id::text, action, actor, reason, before, after, created_at
			FROM report_events WHERE tenant_id=$1 AND report_id=$2 ORDER BY created_at DESC LIMIT 100`, tenantID, reportID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e Event
			if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Reason, &e.Before, &e.After, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

func (s *store) listVersions(ctx context.Context, tenantID, reportID string) ([]ReportSummary, error) {
	ctx = scope(ctx, tenantID)
	var institution, number string
	if err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT institution, report_number FROM report_reports WHERE tenant_id=$1 AND id=$2`, tenantID, reportID).Scan(&institution, &number)
	}); err != nil {
		return nil, err
	}
	if institution == "" || number == "" {
		return []ReportSummary{}, nil
	}
	out := []ReportSummary{}
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		rows, err := tx.Query(ctx, `SELECT id::text, report_number, report_name, archive_status, report_version, is_current, COALESCE(revision_of::text, ''), created_at
			FROM report_reports WHERE tenant_id=$1 AND lower(institution)=lower($2) AND lower(report_number)=lower($3)
			ORDER BY report_version DESC, created_at DESC`, tenantID, institution, number)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v ReportSummary
			if err := rows.Scan(&v.ID, &v.ReportNumber, &v.ReportName, &v.ArchiveStatus, &v.ReportVersion, &v.IsCurrent, &v.RevisionOf, &v.CreatedAt); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

func (s *store) createBatch(ctx context.Context, tenantID, actor string) (string, error) {
	ctx = scope(ctx, tenantID)
	var id string
	err := s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `INSERT INTO report_upload_batches (tenant_id, created_by) VALUES ($1,$2) RETURNING id::text`, tenantID, actor).Scan(&id)
	})
	return id, err
}

func (s *store) addUploadItem(ctx context.Context, tenantID, batchID string, item UploadItem) error {
	ctx = scope(ctx, tenantID)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.Exec(ctx, `INSERT INTO report_upload_items
			(batch_id, tenant_id, file_name, status, report_id, file_id, sha256, size, pages, error_code, error_message)
			VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11)`,
			batchID, tenantID, item.FileName, item.Status, item.ReportID, item.FileID, item.SHA256, item.Size, item.Pages, item.ErrorCode, item.ErrorMessage)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE report_upload_batches SET
			total = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1),
			accepted = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='accepted'),
			failed = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='rejected'),
			duplicate = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='duplicate')
			WHERE tenant_id=$2 AND id=$1`, batchID, tenantID)
		return err
	})
}

func (s *store) addUploadItemTx(ctx context.Context, tx db.DBTX, tenantID, batchID string, item UploadItem) error {
	_, err := tx.Exec(ctx, `INSERT INTO report_upload_items
		(batch_id, tenant_id, file_name, status, report_id, file_id, sha256, size, pages, error_code, error_message)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11)`,
		batchID, tenantID, item.FileName, item.Status, item.ReportID, item.FileID, item.SHA256, item.Size, item.Pages, item.ErrorCode, item.ErrorMessage)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE report_upload_batches SET
		total = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1),
		accepted = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='accepted'),
		failed = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='rejected'),
		duplicate = (SELECT count(*) FROM report_upload_items WHERE batch_id=$1 AND status='duplicate')
		WHERE tenant_id=$2 AND id=$1`, batchID, tenantID)
	return err
}

func (s *store) findReportByFileTx(ctx context.Context, tx db.DBTX, tenantID, fileID string) (Report, error) {
	r, err := scanReport(tx.QueryRow(ctx, `SELECT `+reportColumns+` FROM report_reports r JOIN report_files f ON f.id = r.file_id AND f.tenant_id = r.tenant_id
		WHERE r.tenant_id = $1 AND r.file_id = $2 ORDER BY r.created_at DESC LIMIT 1`, tenantID, fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *store) getBatch(ctx context.Context, tenantID, batchID string) (UploadBatch, error) {
	if !validUUID(batchID) {
		return UploadBatch{}, ErrNotFound
	}
	ctx = scope(ctx, tenantID)
	var b UploadBatch
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		err := tx.QueryRow(ctx, `SELECT id::text, created_by, total, accepted, failed, duplicate, created_at
			FROM report_upload_batches WHERE tenant_id=$1 AND id=$2`, tenantID, batchID).
			Scan(&b.ID, &b.CreatedBy, &b.Total, &b.Accepted, &b.Failed, &b.Duplicate, &b.CreatedAt)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT i.id::text, i.file_name, i.status, COALESCE(i.report_id::text,''), COALESCE(i.file_id::text,''),
			i.sha256, i.size, i.pages, i.error_code, i.error_message, COALESCE(r.archive_status,''), i.created_at
			FROM report_upload_items i LEFT JOIN report_reports r ON r.id=i.report_id AND r.tenant_id=i.tenant_id
			WHERE i.tenant_id=$1 AND i.batch_id=$2 ORDER BY i.created_at`, tenantID, batchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		b.Items = []UploadItem{}
		for rows.Next() {
			var item UploadItem
			if err := rows.Scan(&item.ID, &item.FileName, &item.Status, &item.ReportID, &item.FileID, &item.SHA256, &item.Size, &item.Pages,
				&item.ErrorCode, &item.ErrorMessage, &item.ArchiveStatus, &item.CreatedAt); err != nil {
				return err
			}
			b.Items = append(b.Items, item)
		}
		return rows.Err()
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

func (s *store) listRules(ctx context.Context, tenantID string) ([]Rule, error) {
	ctx = scope(ctx, tenantID)
	out := []Rule{}
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		rows, err := tx.Query(ctx, `SELECT id::text, COALESCE(tenant_id::text,''), template_key, name, version, status, config, created_by, created_at, activated_at, retired_at
			FROM report_rules WHERE tenant_id IS NULL OR tenant_id=$1
			ORDER BY (tenant_id IS NULL), template_key, version DESC`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r Rule
			var raw []byte
			if err := rows.Scan(&r.ID, &r.TenantID, &r.TemplateKey, &r.Name, &r.Version, &r.Status, &raw, &r.CreatedBy, &r.CreatedAt, &r.ActivatedAt, &r.RetiredAt); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &r.Config); err != nil {
				return err
			}
			r.Builtin = r.TenantID == ""
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *store) getRule(ctx context.Context, tenantID, id string) (Rule, error) {
	if !validUUID(id) {
		return Rule{}, ErrNotFound
	}
	ctx = scope(ctx, tenantID)
	var r Rule
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		r, e = s.getRuleTx(ctx, tx, tenantID, id)
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *store) getRuleTx(ctx context.Context, tx db.DBTX, tenantID, id string) (Rule, error) {
	var r Rule
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT id::text, COALESCE(tenant_id::text,''), template_key, name, version, status, config, created_by, created_at, activated_at, retired_at
		FROM report_rules WHERE id=$1 AND (tenant_id=$2 OR tenant_id IS NULL)`, id, tenantID).
		Scan(&r.ID, &r.TenantID, &r.TemplateKey, &r.Name, &r.Version, &r.Status, &raw, &r.CreatedBy, &r.CreatedAt, &r.ActivatedAt, &r.RetiredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw, &r.Config); err != nil {
		return r, err
	}
	r.Builtin = r.TenantID == ""
	return r, nil
}

func (s *store) activeRules(ctx context.Context, tenantID string) ([]Rule, error) {
	all, err := s.listRules(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Rule, 0, len(all))
	for _, r := range all {
		if r.Status == "active" {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *store) insertRuleVersionTx(ctx context.Context, tx db.DBTX, tenantID, templateKey, name, actor string, cfg RuleConfig) (Rule, error) {
	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM report_rules WHERE tenant_id=$1 AND template_key=$2`, tenantID, templateKey).Scan(&version); err != nil {
		return Rule{}, err
	}
	raw, _ := json.Marshal(cfg)
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO report_rules (tenant_id, template_key, name, version, status, config, created_by)
		VALUES ($1,$2,$3,$4,'draft',$5,$6) RETURNING id::text`, tenantID, templateKey, name, version, raw, actor).Scan(&id)
	if err != nil {
		return Rule{}, err
	}
	return s.getRuleTx(ctx, tx, tenantID, id)
}

func (s *store) activateRuleTx(ctx context.Context, tx db.DBTX, tenantID, id string) (Rule, error) {
	if _, err := tx.Exec(ctx, `UPDATE report_rules SET status='retired', retired_at=now()
		WHERE tenant_id=$1 AND id<>$2 AND status='active' AND template_key=(SELECT template_key FROM report_rules WHERE id=$2 AND tenant_id=$1) AND tenant_id=$1`, tenantID, id); err != nil {
		return Rule{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE report_rules SET status='active', activated_at=now(), retired_at=NULL
		WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	if err != nil {
		if pgErrCode(err) == "23505" {
			return Rule{}, ErrConflict
		}
		return Rule{}, err
	}
	if tag.RowsAffected() == 0 {
		return Rule{}, ErrNotFound
	}
	return s.getRuleTx(ctx, tx, tenantID, id)
}

func pgErrCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func (s *store) insertTrial(ctx context.Context, tenantID, ruleID, actor, textSHA string, result any) error {
	ctx = scope(ctx, tenantID)
	raw, _ := json.Marshal(result)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.Exec(ctx, `INSERT INTO report_rule_trials (tenant_id, rule_id, created_by, text_sha256, result)
			VALUES ($1,$2,$3,$4,$5)`, tenantID, ruleID, actor, textSHA, raw)
		return err
	})
}

func (s *store) enqueueJobTx(ctx context.Context, tx db.DBTX, tenantID, reportID, kind, key string) error {
	_, err := tx.Exec(ctx, `INSERT INTO report_jobs (tenant_id, report_id, kind, idempotency_key)
		VALUES ($1,$2,$3,$4) ON CONFLICT (idempotency_key) DO NOTHING`, tenantID, reportID, kind, key)
	return err
}

func (s *store) claimJob(ctx context.Context, workerID string) (job, bool, error) {
	var j job
	claimed := false
	err := s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		var id string
		err := tx.QueryRow(ctx, `WITH candidate AS (
				SELECT id FROM report_jobs WHERE status='queued' AND available_at <= now()
				ORDER BY available_at, created_at FOR UPDATE SKIP LOCKED LIMIT 1
			)
			UPDATE report_jobs j SET status='running', attempts=j.attempts+1, locked_at=now(), locked_by=$1, updated_at=now()
			FROM candidate WHERE j.id=candidate.id
			RETURNING j.id::text, j.tenant_id::text, j.report_id::text, j.kind, j.attempts, j.max_attempts`, workerID).
			Scan(&id, &j.TenantID, &j.ReportID, &j.Kind, &j.Attempts, &j.MaxAttempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		j.ID = id
		claimed = true
		return nil
	})
	return j, claimed, err
}

type job struct {
	ID          string
	TenantID    string
	ReportID    string
	Kind        string
	Attempts    int
	MaxAttempts int
}

func (s *store) finishJob(ctx context.Context, j job) error {
	ctx = scope(ctx, j.TenantID)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.Exec(ctx, `UPDATE report_jobs SET status='succeeded', locked_at=NULL, last_error='', error_stage='', updated_at=now() WHERE id=$1`, j.ID)
		return err
	})
}

func (s *store) retryJob(ctx context.Context, j job, stage, message string, delay time.Duration) error {
	ctx = scope(ctx, j.TenantID)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.Exec(ctx, `UPDATE report_jobs SET status='queued', available_at=now()+$2::interval, locked_at=NULL,
			last_error=$3, error_stage=$4, updated_at=now() WHERE id=$1`, j.ID, fmt.Sprintf("%d seconds", int(delay.Seconds())), message, stage)
		return err
	})
}

func (s *store) failJob(ctx context.Context, j job, stage, message string) error {
	ctx = scope(ctx, j.TenantID)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		_, err := tx.Exec(ctx, `UPDATE report_jobs SET status='failed', locked_at=NULL, last_error=$2, error_stage=$3, updated_at=now() WHERE id=$1`, j.ID, message, stage)
		return err
	})
}

func (s *store) resetStaleJobs(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		tag, err := tx.Exec(ctx, `UPDATE report_jobs SET status='queued', locked_at=NULL, updated_at=now()
			WHERE status='running' AND (locked_at IS NULL OR locked_at < now() - interval '10 minutes')`)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		return nil
	})
	return n, err
}

func (s *store) latestJob(ctx context.Context, tenantID, reportID string) (job, error) {
	ctx = scope(ctx, tenantID)
	var j job
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT id::text, tenant_id::text, report_id::text, kind, attempts, max_attempts
			FROM report_jobs WHERE tenant_id=$1 AND report_id=$2 ORDER BY created_at DESC LIMIT 1`, tenantID, reportID).
			Scan(&j.ID, &j.TenantID, &j.ReportID, &j.Kind, &j.Attempts, &j.MaxAttempts)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

func (s *store) manualRetryJob(ctx context.Context, tenantID, reportID string) error {
	ctx = scope(ctx, tenantID)
	return s.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		return manualRetryJobTx(ctx, tx, tenantID, reportID)
	})
}

func manualRetryJobTx(ctx context.Context, tx db.DBTX, tenantID, reportID string) error {
	tag, err := tx.Exec(ctx, `UPDATE report_jobs SET status='queued', attempts=0, available_at=now(), locked_at=NULL,
		last_error='', error_stage='', updated_at=now()
		WHERE id=(SELECT id FROM report_jobs WHERE tenant_id=$1 AND report_id=$2 ORDER BY created_at DESC LIMIT 1)`, tenantID, reportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) insertOCRRun(ctx context.Context, tx db.DBTX, tenantID, reportID, status, provider, model string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO report_ocr_runs (report_id, tenant_id, status, provider, model)
		VALUES ($1,$2,$3,$4,$5) RETURNING id::text`, reportID, tenantID, status, provider, model).Scan(&id)
	return id, err
}

func (s *store) finishOCRRunTx(ctx context.Context, tx db.DBTX, tenantID, id string, result ocrResult) error {
	pages, _ := json.Marshal(result.Pages)
	_, err := tx.Exec(ctx, `UPDATE report_ocr_runs SET status='succeeded', provider=$3, model=$4, pages=$5, text=$6, raw=$7, finished_at=now()
		WHERE tenant_id=$1 AND id=$2`, tenantID, id, result.Provider, result.Model, pages, ocrPageText(result.Pages), result.Raw)
	return err
}

func (s *store) failOCRRunTx(ctx context.Context, tx db.DBTX, tenantID, id, code, message string) error {
	_, err := tx.Exec(ctx, `UPDATE report_ocr_runs SET status='failed', error_code=$3, error_message=$4, finished_at=now()
		WHERE tenant_id=$1 AND id=$2`, tenantID, id, code, message)
	return err
}

func (s *store) latestOCRRun(ctx context.Context, tenantID, reportID string) (*OCRRun, error) {
	ctx = scope(ctx, tenantID)
	var r OCRRun
	var pages, raw []byte
	var finished *time.Time
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT id::text, status, provider, model, COALESCE(text,''), pages, raw, error_code, error_message, started_at, finished_at
			FROM report_ocr_runs WHERE tenant_id=$1 AND report_id=$2 ORDER BY created_at DESC LIMIT 1`, tenantID, reportID).
			Scan(&r.ID, &r.Status, &r.Provider, &r.Model, &r.Text, &pages, &raw, &r.ErrorCode, &r.ErrorMessage, &r.StartedAt, &finished)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.FinishedAt = finished
	_ = json.Unmarshal(pages, &r.Pages)
	r.Raw = raw
	return &r, nil
}

func (s *store) insertParseRunTx(ctx context.Context, tx db.DBTX, tenantID, reportID, ocrRunID string, result ParseResult, status string) (string, error) {
	fields, _ := json.Marshal(result.Fields)
	details, _ := json.Marshal(result.Details)
	issues, _ := json.Marshal(result.Issues)
	raw, _ := json.Marshal(result)
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO report_parse_runs (report_id, tenant_id, ocr_run_id, rule_id, rule_version, status, fields, details, issues, raw)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,0),$6,$7,$8,$9,$10) RETURNING id::text`,
		reportID, tenantID, ocrRunID, result.RuleID, result.RuleVersion, status, fields, details, issues, raw).Scan(&id)
	return id, err
}

func (s *store) latestParseRun(ctx context.Context, tenantID, reportID string) (*ParseRun, error) {
	ctx = scope(ctx, tenantID)
	var p ParseRun
	var issues, fields, details []byte
	var applied *time.Time
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		return tx.QueryRow(ctx, `SELECT id::text, status, COALESCE(rule_id::text,''), COALESCE(rule_version,0), fields, details, issues, created_at, applied_at
			FROM report_parse_runs WHERE tenant_id=$1 AND report_id=$2 ORDER BY created_at DESC LIMIT 1`, tenantID, reportID).
			Scan(&p.ID, &p.Status, &p.RuleID, &p.RuleVersion, &fields, &details, &issues, &p.CreatedAt, &applied)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.AppliedAt = applied
	p.Fields, p.Details = fields, details
	_ = json.Unmarshal(issues, &p.Issues)
	return &p, nil
}

type candidateParse struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"ruleId"`
	RuleVersion int               `json:"ruleVersion"`
	Fields      map[string]string `json:"fields"`
	Details     []Detail          `json:"details"`
	Issues      []Issue           `json:"issues"`
}

func (s *store) candidate(ctx context.Context, tenantID, reportID string) (*candidateParse, error) {
	if !validUUID(reportID) {
		return nil, nil
	}
	ctx = scope(ctx, tenantID)
	var c *candidateParse
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		var e error
		c, e = candidateTx(ctx, tx, tenantID, reportID)
		return e
	})
	return c, err
}

func candidateTx(ctx context.Context, tx db.DBTX, tenantID, reportID string) (*candidateParse, error) {
	var c candidateParse
	var fields, details, issues []byte
	err := tx.QueryRow(ctx, `SELECT id::text, COALESCE(rule_id::text,''), COALESCE(rule_version,0), fields, details, issues FROM report_parse_runs
		WHERE tenant_id=$1 AND report_id=$2 AND status='candidate' ORDER BY created_at DESC LIMIT 1`, tenantID, reportID).
		Scan(&c.ID, &c.RuleID, &c.RuleVersion, &fields, &details, &issues)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(fields, &c.Fields)
	_ = json.Unmarshal(details, &c.Details)
	_ = json.Unmarshal(issues, &c.Issues)
	return &c, nil
}

func (s *store) markCandidateAppliedTx(ctx context.Context, tx db.DBTX, tenantID, reportID, candidateID, actor string) error {
	// Retire every other candidate/accepted run, but never the one being applied.
	if _, err := tx.Exec(ctx, `UPDATE report_parse_runs SET status='superseded'
		WHERE tenant_id=$1 AND report_id=$2 AND id<>$3 AND status IN ('candidate','applied')`, tenantID, reportID, candidateID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE report_parse_runs SET status='applied', applied_by=$4, applied_at=now()
		WHERE tenant_id=$1 AND id=$2 AND report_id=$3 AND status='candidate'`, tenantID, candidateID, reportID, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// listReports applies tenant, scope and same-detail filtering. The item/judgement pair is one
// EXISTS over report_details so two different details cannot satisfy the two halves.
func (s *store) listReports(ctx context.Context, tenantID string, f ListFilter) (ListResult, error) {
	ctx = scope(ctx, tenantID)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	where := []string{"r.tenant_id = $1"}
	args := []any{tenantID}
	add := func(v any, clause string) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	switch f.Scope {
	case "", "current":
		where = append(where, "r.archive_status='archived' AND r.is_current AND r.voided_at IS NULL")
	case "review":
		where = append(where, "r.voided_at IS NULL AND (r.archive_status IN ('needs_review','pending_ocr','ocr_running','parsing') OR (r.archive_status='archived' AND NOT r.is_current))")
	case "history":
		where = append(where, "r.archive_status='archived' AND NOT r.is_current AND r.voided_at IS NULL")
	case "voided":
		where = append(where, "r.voided_at IS NOT NULL")
	case "all":
	default:
		return ListResult{}, fmt.Errorf("%w: unknown scope", ErrInvalidInput)
	}
	if q := trim(f.Query); q != "" {
		add("%"+q+"%", "(r.report_number ILIKE %[1]s OR r.report_name ILIKE %[1]s OR r.product_name ILIKE %[1]s OR r.institution ILIKE %[1]s OR r.client_unit ILIKE %[1]s OR r.inspected_unit ILIKE %[1]s)")
	}
	if v := trim(f.Institution); v != "" {
		add("%"+v+"%", "r.institution ILIKE %[1]s")
	}
	if v := trim(f.DateFrom); v != "" {
		add(v, "r.report_date >= %[1]s::date")
	}
	if v := trim(f.DateTo); v != "" {
		add(v, "r.report_date <= %[1]s::date")
	}
	if v := trim(f.Status); v != "" {
		add(v, "r.archive_status = %[1]s")
	}
	if trim(f.Item) != "" || trim(f.Judgement) != "" {
		args = append(args, "%"+trim(f.Item)+"%", trim(f.Judgement))
		n := len(args)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM report_details d WHERE d.tenant_id=r.tenant_id AND d.report_id=r.id AND d.item_name ILIKE $%d AND ($%d='' OR d.judgement=$%d))", n-1, n, n))
	}
	base := " FROM report_reports r JOIN report_files f ON f.id=r.file_id AND f.tenant_id=r.tenant_id WHERE " + strings.Join(where, " AND ")
	var total int
	out := []Report{}
	err := s.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		if err := tx.QueryRow(ctx, "SELECT count(*)"+base, args...).Scan(&total); err != nil {
			return err
		}
		limitArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
		rows, err := tx.Query(ctx, "SELECT "+reportColumns+base+" ORDER BY r.report_date DESC NULLS LAST, r.created_at DESC LIMIT $"+fmt.Sprint(len(args)+1)+" OFFSET $"+fmt.Sprint(len(args)+2), limitArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			r, err := scanReport(rows)
			if err != nil {
				return err
			}
			r.Details = nil
			r.Events = nil
			r.Versions = nil
			r.OCR = nil
			r.LatestParse = nil
			// The list response carries only summary fields.
			r.RawFields = nil
			r.FieldSources = nil
			out = append(out, r)
		}
		return rows.Err()
	})
	return ListResult{Items: out, Total: total, Page: f.Page, PageSize: f.PageSize}, err
}
