package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

const nativeBMGSMaxReports = 50
const nativeBMGSMaxImages = 250
const nativeBMGSMaxRawBytes = 256 * 1024 * 1024
const nativeBMGSMaxProcessedBytes = 100 * 1024 * 1024
const nativeBMGSMaxWorkbookBytes = 150 * 1024 * 1024

type nativeBMGSRecord struct {
	ref BMGSSourceRef
	row Record
}

func (s *Service) nativeExportProject(ctx context.Context, projectID, actorID, permission string) (Record, error) {
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{permission})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xuất hồ sơ này")
	}
	return s.repo.QueryOne(ctx, `SELECT id::text,code,name,address FROM projects WHERE id=$1 AND status <> 'cancelled'`, projectID)
}

func (s *Service) nativeDailySource(ctx context.Context, sourceID, actorID string) (Record, error) {
	if !validActivityUUID(sourceID) {
		return nil, validationError("Báo cáo không hợp lệ")
	}
	identity, err := s.repo.QueryOne(ctx, `SELECT project_id::text,work_item_id::text FROM operation_daily_reports WHERE id=$1`, sourceID)
	if err != nil {
		return nil, err
	}
	if _, err = s.nativeExportProject(ctx, cleanID(identity["project_id"]), actorID, "operation.view"); err != nil {
		return nil, err
	}
	if id := cleanID(identity["work_item_id"]); id != "" {
		allowed, e := s.repo.CanViewWorkItem(ctx, id, actorID)
		if e != nil {
			return nil, e
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xem Đầu việc của báo cáo")
		}
		item, e := s.repo.GetWorkItemDetail(ctx, id)
		if e != nil {
			return nil, e
		}
		if cleanText(item["status"]) == "cancelled" {
			return nil, validationError("Đầu việc của báo cáo đã hủy")
		}
	}
	row, err := s.repo.WorkReportSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if err = enrichNativeDailySourceForActor(ctx, s.repo.querySource(), row, false, actorID); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) nativeWorkReportSource(ctx context.Context, reportID string, revision int, actorID string) (Record, error) {
	if !validActivityUUID(reportID) || revision < 1 {
		return nil, validationError("Báo cáo hoặc lần gửi không hợp lệ")
	}
	header, err := s.repo.WorkReportHeader(ctx, reportID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(header["project_id"])
	if _, err = s.nativeExportProject(ctx, projectID, actorID, "work_report.export"); err != nil {
		return nil, err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"work_report.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem bản gửi")
	}
	manage, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"work_report.manage"})
	if err != nil {
		return nil, err
	}
	supervise, err := s.repo.CanSuperviseWorkReport(ctx, actorID, header)
	if err != nil {
		return nil, err
	}
	if !manage && !supervise && actorID != cleanID(header["author_id"]) && actorID != cleanID(header["recipient_id"]) {
		return nil, forbiddenError("Bản gửi không thuộc phạm vi của bạn")
	}
	version, err := s.repo.WorkReportRevision(ctx, reportID, revision)
	if err != nil {
		return nil, err
	}
	snapshot := recordFromAny(version["snapshot_json"])
	if cleanID(snapshot["project_id"]) != projectID {
		return nil, validationError("Bản gửi thiếu định danh công trình")
	}
	return bmgsCopy(snapshot), nil
}

func (s *Service) ListNativeDailyReports(ctx context.Context, projectID, actorID string, filter NativeDailyReportFilter) ([]Record, error) {
	if err := bmgsValidateDates(filter.DateFrom, filter.DateTo); err != nil {
		return nil, err
	}
	if _, err := s.nativeExportProject(ctx, projectID, actorID, "operation.view"); err != nil {
		return nil, err
	}
	rows, err := s.repo.nativeDailyReports(ctx, projectID, actorID, filter, 0)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if cleanText(row["delivery_status"]) == "needs_changes" {
			if err = freezeNativeWorkReportEvidence(ctx, s.repo.querySource(), row, false); err != nil {
				return nil, err
			}
		}
		if err = annotateNativeWorkReportDeliverySourceChanged(ctx, s.repo.querySource(), row); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func (s *Service) ExportNativeDailyReport(ctx context.Context, sourceID, actorID, actorName string) (tabularExportResult, error) {
	row, err := s.nativeDailySource(ctx, sourceID, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	return s.exportNativeDailyRecords(ctx, []nativeBMGSRecord{{BMGSSourceRef{Kind: "daily_report", ID: sourceID}, row}}, actorID, actorName)
}

func (s *Service) ExportNativeWorkReport(ctx context.Context, reportID string, revision int, actorID, actorName string) (tabularExportResult, error) {
	row, err := s.nativeWorkReportSource(ctx, reportID, revision, actorID)
	if err != nil {
		return tabularExportResult{}, err
	}
	return s.exportNativeDailyRecords(ctx, []nativeBMGSRecord{{BMGSSourceRef{Kind: "work_report", ID: reportID, RevisionNo: revision}, row}}, actorID, actorName)
}

func (s *Service) ExportNativeDailyReports(ctx context.Context, projectID string, filter NativeDailyReportFilter, actorID, actorName string) (tabularExportResult, error) {
	if err := bmgsValidateDates(filter.DateFrom, filter.DateTo); err != nil {
		return tabularExportResult{}, err
	}
	if _, err := s.nativeExportProject(ctx, projectID, actorID, "operation.view"); err != nil {
		return tabularExportResult{}, err
	}
	rows, err := s.repo.nativeDailyReports(ctx, projectID, actorID, filter, nativeBMGSMaxReports+1)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(rows) > nativeBMGSMaxReports {
		return tabularExportResult{}, validationError("Phạm vi có hơn 50 báo cáo; thu hẹp bộ lọc ngày tại danh sách")
	}
	records := []nativeBMGSRecord{}
	for _, candidate := range rows {
		row, e := s.nativeDailySource(ctx, cleanID(candidate["source_id"]), actorID)
		if e != nil {
			return tabularExportResult{}, e
		}
		records = append(records, nativeBMGSRecord{BMGSSourceRef{Kind: "daily_report", ID: cleanID(candidate["source_id"])}, row})
	}
	return s.exportNativeDailyRecords(ctx, records, actorID, actorName)
}

func (s *Service) ExportNativeWorkReports(ctx context.Context, filter WorkReportFilter, actorID, actorName string) (tabularExportResult, error) {
	rows, err := s.workReports(ctx, filter, actorID, nativeBMGSMaxReports+1)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(rows) > nativeBMGSMaxReports {
		return tabularExportResult{}, validationError("Phạm vi có hơn 50 báo cáo; thu hẹp bộ lọc tại danh sách")
	}
	records := []nativeBMGSRecord{}
	for _, candidate := range rows {
		if _, err = s.nativeExportProject(ctx, cleanID(candidate["project_id"]), actorID, "work_report.export"); err != nil {
			return tabularExportResult{}, err
		}
		ref := BMGSSourceRef{Kind: "work_report", ID: cleanID(candidate["work_report_id"]), RevisionNo: int(number(candidate["current_revision_no"]))}
		var row Record
		if ref.ID == "" {
			ref = BMGSSourceRef{Kind: "daily_report", ID: cleanID(candidate["source_id"])}
			row, err = s.nativeDailySource(ctx, ref.ID, actorID)
		} else {
			row, err = s.nativeWorkReportSource(ctx, ref.ID, ref.RevisionNo, actorID)
		}
		if err != nil {
			return tabularExportResult{}, err
		}
		records = append(records, nativeBMGSRecord{ref, row})
	}
	return s.exportNativeDailyRecords(ctx, records, actorID, actorName)
}

func (s *Service) exportNativeDailyRecords(ctx context.Context, records []nativeBMGSRecord, actorID, actorName string) (tabularExportResult, error) {
	return s.exportNativeRecords(ctx, "BM-GS-02", records, actorID, actorName)
}

func (s *Service) exportNativeRecords(ctx context.Context, code string, records []nativeBMGSRecord, actorID, actorName string) (tabularExportResult, error) {
	if len(records) == 0 {
		return tabularExportResult{}, validationError("Không có báo cáo trong phạm vi lọc")
	}
	if code == "BM-GS-04" && len(records) > 5000 {
		return tabularExportResult{}, validationError("Danh sách có hơn 5000 lỗi; thu hẹp bộ lọc tại nguồn")
	}
	if code != "BM-GS-07" && code != "BM-GS-04" && len(records) > nativeBMGSMaxReports {
		return tabularExportResult{}, validationError("Tối đa 50 báo cáo trong một workbook; thu hẹp bộ lọc")
	}
	data := BMGSWorkbookData{TemplateCode: code, PreparedBy: actorName, GeneratedAt: time.Now(), Sheets: []BMGSSheet{}, Images: []BMGSImage{}}
	first := records[0].row
	data.ProjectCode = cleanText(first["project_code"])
	data.ProjectName = cleanText(first["project_name"])
	data.Address = cleanText(first["project_address"])
	if data.Address == "" {
		data.Address = cleanText(first["address"])
	}
	sourceAudit := []Record{}
	imageAudit := []Record{}
	seen := map[string]bool{}
	var rawTotal int64
	processedTotal := 0
	for index, record := range records {
		if err := ctx.Err(); err != nil {
			return tabularExportResult{}, err
		}
		sheet, err := nativeBMGSProjectSheet(code, record.ref, record.row)
		if err != nil {
			return tabularExportResult{}, err
		}
		if sheet.DateLabel != "" {
			sheet.Name = fmt.Sprintf("%s_%02d", sheet.DateLabel, index+1)
		} else {
			if sheet.Name == "" {
				sheet.Name = code
			}
			sheet.Name = fmt.Sprintf("%s_%02d", sheet.Name, index+1)
		}
		photos, err := s.nativeDailyPhotos(ctx, record, actorID)
		if err != nil {
			return tabularExportResult{}, err
		}
		if statusIn(record.ref.Kind, "daily_report", "work_report") && len(photos) > 50 {
			return tabularExportResult{}, validationError("Báo cáo có hơn 50 ảnh đã liên kết; kiểm tra nguồn báo cáo")
		}
		if code == "BM-GS-02" {
			nativeBMGSProjectDailyPhotos(&sheet, record.row, photos)
		}
		data.Sheets = append(data.Sheets, sheet)
		for _, photo := range photos {
			id := cleanID(photo["id"])
			key := nativeBMGSImageIdentity(record.ref, id)
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(data.Images) >= nativeBMGSMaxImages {
				return tabularExportResult{}, validationError("Phạm vi có hơn 250 phiên bản ảnh; thu hẹp bộ lọc báo cáo")
			}
			var file EvidenceFileResult
			if record.ref.Kind == "work_report" {
				// Repeat native report permission and exact membership before opening a frozen locator.
				latest, e := s.nativeWorkReportSource(ctx, record.ref.ID, record.ref.RevisionNo, actorID)
				if e != nil {
					return tabularExportResult{}, e
				}
				if !workReportContains(workReportSnapshotEvidenceIDs(latest), id) {
					return tabularExportResult{}, forbiddenError("Ảnh không thuộc lần gửi báo cáo")
				}
				if cleanText(photo["storage_provider"]) != "" && cleanText(photo["storage_key"]) != "" {
					file, err = s.storedFileResult(ctx, photo, "storage_provider", "storage_key", "mime_type", evidenceDownloadName(photo))
				} else {
					file, err = s.WorkReportEvidenceFile(ctx, record.ref.ID, record.ref.RevisionNo, id, actorID)
				}
			} else {
				file, err = s.EvidenceFile(ctx, id, actorID)
			}
			if err != nil {
				return tabularExportResult{}, err
			}
			if file.Size > bmgsMaxRawImageBytes {
				file.Reader.Close()
				return tabularExportResult{}, validationError("Ảnh gốc vượt quá 12 MiB; không thể xuất đầy đủ phạm vi")
			}
			raw, e := io.ReadAll(io.LimitReader(file.Reader, bmgsMaxRawImageBytes+1))
			file.Reader.Close()
			if e != nil {
				return tabularExportResult{}, e
			}
			if len(raw) > bmgsMaxRawImageBytes {
				return tabularExportResult{}, validationError("Ảnh gốc vượt quá 12 MiB")
			}
			rawTotal += int64(len(raw))
			if rawTotal > nativeBMGSMaxRawBytes {
				return tabularExportResult{}, validationError("Tổng ảnh gốc vượt quá 256 MiB; thu hẹp bộ lọc báo cáo")
			}
			sum := sha256.Sum256(raw)
			checksum := hex.EncodeToString(sum[:])
			if expected := cleanText(photo["checksum_sha256"]); expected != "" && !strings.EqualFold(expected, checksum) {
				return tabularExportResult{}, validationError("Checksum ảnh không khớp bản gốc của nguồn")
			}
			image, e := prepareBMGSImage(raw, file.MIMEType)
			if e != nil {
				return tabularExportResult{}, validationError("Không thể xuất ảnh đã liên kết", Record{"evidence_id": id, "detail": e.Error()})
			}
			image.EvidenceID = id
			image.SourceRevisionKey = key
			image.Filename = cleanText(photo["name"])
			if record.ref.Kind != "work_report" && image.Filename == "" {
				image.Filename = file.Filename
			}
			image.Caption = cleanText(photo["caption"])
			if image.Caption == "" {
				image.Caption = cleanText(photo["description"])
			}
			capture := recordFromAny(recordFromAny(photo["payload_json"])["capture_metadata"])
			image.Location = cleanText(capture["location"])
			image.CapturedAt = cleanText(capture["captured_at"])
			image.Phase = cleanText(capture["photo_phase"])
			image.SourceLabel = nativeSourceDescription(record.ref)
			processedTotal += len(image.Bytes)
			if processedTotal > nativeBMGSMaxProcessedBytes {
				return tabularExportResult{}, validationError("Ảnh sau nén vượt quá 100 MiB; thu hẹp bộ lọc báo cáo")
			}
			data.Images = append(data.Images, image)
			imageAudit = append(imageAudit, Record{"evidence_id": id, "source_revision_key": key, "input_bytes": len(raw), "output_bytes": len(image.Bytes), "checksum_sha256": checksum})
		}
		sourceAudit = append(sourceAudit, Record{"kind": record.ref.Kind, "id": record.ref.ID, "revision_no": record.ref.RevisionNo})
	}
	if err := ctx.Err(); err != nil {
		return tabularExportResult{}, err
	}
	bytes, err := generateBMGSWorkbook(data)
	if err != nil {
		return tabularExportResult{}, err
	}
	if len(bytes) > nativeBMGSMaxWorkbookBytes {
		return tabularExportResult{}, validationError("Workbook vượt quá 150 MiB; thu hẹp bộ lọc")
	}
	if err = ctx.Err(); err != nil {
		return tabularExportResult{}, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(first["project_id"]), actorID, actorName, "native_source_exported", "native_source", "", "Xuất hồ sơ nguồn theo mẫu client", Record{"template_code": code, "sources": sourceAudit, "images": imageAudit, "file_bytes": len(bytes)})
	return tabularExportResult{Data: bytes, Filename: cleanUploadFilename(data.ProjectCode + "_" + code + "_ho-so.xlsx"), ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, nil
}

// Only the source's canonical saved links are inherited. Retired supplemental
// JSON and current project photos never enlarge a report's membership.
func (s *Service) nativeDailyPhotos(ctx context.Context, source nativeBMGSRecord, actorID string) ([]Record, error) {
	ids := workReportSnapshotEvidenceIDs(source.row)
	out := []Record{}
	for _, id := range ids {
		if !validActivityUUID(id) {
			return nil, validationError("Liên kết ảnh của báo cáo không hợp lệ")
		}
		var photo Record
		if source.ref.Kind == "work_report" {
			members, err := s.repo.WorkReportEvidenceRecords(ctx, cleanID(source.row["project_id"]), cleanID(source.row["work_item_id"]), []string{id})
			if err != nil {
				return nil, err
			}
			if len(members) != 1 {
				return nil, validationError("Ảnh của lần gửi không còn tồn tại")
			}
			photo = bmgsFrozenEvidence(source.row, id, members[0])
			frozen := false
			for _, member := range workflowAnswerRecordList(source.row["bm_gs_evidence"]) {
				if cleanID(member["id"]) == id {
					frozen = true
					break
				}
			}
			if !frozen {
				// Legacy revisions did not capture labels. Do not import a renamed
				// current filename or capture metadata into that historical report.
				photo["name"] = id
			}
		} else {
			var err error
			photo, err = s.EvidenceDetail(ctx, id, actorID)
			if err != nil {
				return nil, err
			}
		}
		if cleanID(photo["project_id"]) != cleanID(source.row["project_id"]) {
			return nil, forbiddenError("Ảnh không thuộc công trình của báo cáo")
		}
		if workItemID := cleanID(source.row["work_item_id"]); workItemID != "" && cleanID(photo["work_item_id"]) != workItemID {
			return nil, forbiddenError("Ảnh không thuộc Đầu việc của báo cáo")
		}
		if cleanText(photo["evidence_type"]) != "photo" {
			continue
		}
		if statusIn(cleanText(photo["status"]), "deleted", "cancelled", "rejected") {
			return nil, validationError("Ảnh liên kết đã bị hủy hoặc từ chối")
		}
		out = append(out, photo)
	}
	return out, nil
}
