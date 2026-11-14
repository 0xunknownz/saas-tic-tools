package reportarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/0xunknownz/saas.starter/foundation/db"
)

type pendingUpload struct {
	Name     string
	Path     string
	Size     int64
	Oversize bool
}

// createUploadBatch creates the durable batch before any file is processed, so closing the page
// never loses a successfully received file or its task.
func (s *Service) createUploadBatch(ctx context.Context, tenantID, actor string) (string, error) {
	return s.store.createBatch(ctx, tenantID, actor)
}

func (s *Service) processUpload(ctx context.Context, tenantID, actor, batchID string, p pendingUpload) (UploadItem, error) {
	item := UploadItem{FileName: safeFileName(p.Name)}
	if p.Oversize {
		item.Status, item.ErrorCode, item.ErrorMessage = "rejected", "PDF_TOO_LARGE", fmt.Sprintf("文件超过 %d 字节上限", s.cfg.MaxUploadBytes)
		_ = s.store.addUploadItem(ctx, tenantID, batchID, item)
		return item, nil
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		item.Status, item.ErrorCode, item.ErrorMessage = "rejected", "FILE_READ", "无法读取上传文件"
		_ = s.store.addUploadItem(ctx, tenantID, batchID, item)
		return item, nil
	}
	item.Size = int64(len(data))
	sum := sha256.Sum256(data)
	item.SHA256 = hex.EncodeToString(sum[:])
	pages, apiErr := inspectPDF(data, s.cfg.MaxUploadBytes, s.cfg.MaxPDFPages)
	if apiErr != nil {
		item.Status, item.ErrorCode, item.ErrorMessage = "rejected", apiErr.Code, apiErr.Message
		_ = s.store.addUploadItem(ctx, tenantID, batchID, item)
		return item, nil
	}
	item.Pages = pages
	item.FileID = ""
	file := FileInfo{FileName: item.FileName, ContentType: "application/pdf", Size: item.Size, Pages: pages, SHA256: item.SHA256}
	report := Report{CreatedBy: actor, ReportVersion: 1, ArchiveStatus: "pending_ocr", DetectionResult: "unknown", FieldSources: map[string]FieldSource{}, RawFields: map[string]any{}}

	ctx = scope(ctx, tenantID)
	err = s.store.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		stored, existed, err := s.store.insertFileTx(ctx, tx, tenantID, file, data, actor)
		if err != nil {
			return err
		}
		report.File = &stored
		item.FileID = stored.ID
		var previous Report
		previousErr := ErrNotFound
		if existed {
			previous, previousErr = s.store.findReportByFileTx(ctx, tx, tenantID, stored.ID)
		}
		if existed && previousErr != nil && previousErr != ErrNotFound {
			return previousErr
		}
		if existed && previousErr == nil && previous.VoidedAt == nil {
			// A live duplicate links the existing report and creates no report or task.
			item.Status, item.ReportID, item.ArchiveStatus = "duplicate", previous.ID, previous.ArchiveStatus
			return s.store.addUploadItemTx(ctx, tx, tenantID, batchID, item)
		}
		report.ArchiveStatus = "pending_ocr"
		if previousErr == nil {
			report.ArchiveStatus = "needs_review"
			report.DuplicateOf = previous.ID
			report.ReportVersion = previous.ReportVersion + 1
			report.ReviewStage = "duplicate"
			report.ReviewReason = "相同文件的原档案已作废，需管理员确认恢复或作为新档案处理"
		}
		created, err := s.store.insertReportTx(ctx, tx, reportInsert{Report: report, TenantID: tenantID, FileID: stored.ID})
		if err != nil {
			return err
		}
		item.Status, item.ReportID, item.ArchiveStatus = "accepted", created.ID, created.ArchiveStatus
		if report.ArchiveStatus == "pending_ocr" {
			key := "process_report:" + created.ID
			if err := s.store.enqueueJobTx(ctx, tx, tenantID, created.ID, "process_report", key); err != nil {
				return err
			}
		}
		if err := s.store.insertEventTx(ctx, tx, tenantID, created.ID, "uploaded", actor, "", nil, map[string]any{"fileName": item.FileName, "sha256": item.SHA256, "pages": pages}); err != nil {
			return err
		}
		return s.store.addUploadItemTx(ctx, tx, tenantID, batchID, item)
	})
	if err != nil {
		item.Status, item.ErrorCode, item.ErrorMessage = "rejected", "UPLOAD_WRITE", "文件保存或档案写入失败，请重试"
		_ = s.store.addUploadItem(ctx, tenantID, batchID, item)
		return item, fmt.Errorf("upload %s: %w", p.Name, err)
	}
	return item, nil
}

// uploadFiles processes files one by one: one failure never stops the rest and every result is
// recorded in the batch.
func (s *Service) uploadFiles(ctx context.Context, tenantID, actor, batchID string, files []pendingUpload) (UploadBatch, error) {
	for _, f := range files {
		if _, err := s.processUpload(ctx, tenantID, actor, batchID, f); err != nil {
			// The item is already recorded as rejected; keep processing the rest.
			continue
		}
	}
	return s.store.getBatch(ctx, tenantID, batchID)
}

func (s *Service) batch(ctx context.Context, tenantID, batchID string) (UploadBatch, error) {
	return s.store.getBatch(ctx, tenantID, batchID)
}

// retryOneUpload appends a retried file to the member's existing batch. Ownership is checked by
// the caller from the authenticated account, so a member cannot retry somebody else's batch.
func (s *Service) retryOneUpload(ctx context.Context, tenantID, actor, batchID string, p pendingUpload) (UploadBatch, error) {
	b, err := s.store.getBatch(ctx, tenantID, batchID)
	if err != nil {
		return b, err
	}
	if b.CreatedBy != "account:"+actor {
		return b, fail(403, "UPLOAD_RETRY_FORBIDDEN", "只能重试自己批次中的失败文件")
	}
	if _, err := s.processUpload(ctx, tenantID, actor, batchID, p); err != nil {
		return b, err
	}
	return s.store.getBatch(ctx, tenantID, batchID)
}
