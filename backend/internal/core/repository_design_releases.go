package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) ListDesignReleases(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT dr.*,
			COALESCE(creator.full_name, creator.email, '') AS created_by_name,
			COALESCE(releaser.full_name, releaser.email, '') AS released_by_name,
			COALESCE(files.files, '[]'::jsonb) AS files
		FROM design_releases dr
		LEFT JOIN users creator ON creator.id=dr.created_by
		LEFT JOIN users releaser ON releaser.id=dr.released_by
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'id', document.id,
				'name', document.name,
				'document_type', document.document_type,
				'mime_type', document.mime_type,
				'file_size', document.file_size,
				'revision_no', document.revision_no,
				'release_status', document.release_status,
				'uploaded_by', document.uploaded_by,
				'uploaded_by_name', COALESCE(uploader.full_name, uploader.email, ''),
				'created_at', document.created_at
			) ORDER BY document.created_at, document.id) AS files
			FROM documents document
			LEFT JOIN users uploader ON uploader.id=document.uploaded_by
			WHERE document.project_id=dr.project_id
			  AND document.target_type='design_release'
			  AND document.target_id=dr.id
			  AND document.status='active'
		) files ON TRUE
		WHERE dr.project_id=$1
		ORDER BY dr.revision_no DESC
	`, projectID)
}

func (r *Repository) DesignReleaseByID(ctx context.Context, releaseID string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT dr.*,
			COALESCE(creator.full_name, creator.email, '') AS created_by_name,
			COALESCE(releaser.full_name, releaser.email, '') AS released_by_name,
			COALESCE(files.files, '[]'::jsonb) AS files
		FROM design_releases dr
		LEFT JOIN users creator ON creator.id=dr.created_by
		LEFT JOIN users releaser ON releaser.id=dr.released_by
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object(
				'id', document.id,
				'name', document.name,
				'document_type', document.document_type,
				'mime_type', document.mime_type,
				'file_size', document.file_size,
				'revision_no', document.revision_no,
				'release_status', document.release_status,
				'uploaded_by', document.uploaded_by,
				'uploaded_by_name', COALESCE(uploader.full_name, uploader.email, ''),
				'created_at', document.created_at
			) ORDER BY document.created_at, document.id) AS files
			FROM documents document
			LEFT JOIN users uploader ON uploader.id=document.uploaded_by
			WHERE document.project_id=dr.project_id
			  AND document.target_type='design_release'
			  AND document.target_id=dr.id
			  AND document.status='active'
		) files ON TRUE
		WHERE dr.id=$1
	`, releaseID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}

func (r *Repository) LatestDesignReleaseHandoff(ctx context.Context, projectID string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT handoff.*,
			release.revision_no,
			release.revision_label,
			release.status AS release_status,
			COALESCE(actor.full_name, actor.email, '') AS handed_off_by_name
		FROM design_release_handoffs handoff
		JOIN design_releases release ON release.id=handoff.design_release_id
		LEFT JOIN users actor ON actor.id=handoff.handed_off_by
		WHERE handoff.project_id=$1
		ORDER BY handoff.handed_off_at DESC, handoff.id DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *Repository) CreateDesignRelease(ctx context.Context, projectID, workItemID, revisionLabel, changeReason, notes, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "design_release:"+projectID); err != nil {
		return nil, err
	}
	var draftCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_releases WHERE project_id=$1 AND status='draft'`, projectID).Scan(&draftCount); err != nil {
		return nil, err
	}
	if draftCount > 0 {
		return nil, validationError("Công trình đang có một revision thiết kế nháp; hãy tiếp tục revision đó")
	}
	var revisionNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no),0)+1 FROM design_releases WHERE project_id=$1`, projectID).Scan(&revisionNo); err != nil {
		return nil, err
	}
	if revisionLabel == "" {
		revisionLabel = fmt.Sprintf("R%02d", revisionNo)
	}
	var duplicateLabel int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_releases WHERE project_id=$1 AND lower(revision_label)=lower($2)`, projectID, revisionLabel).Scan(&duplicateLabel); err != nil {
		return nil, err
	}
	if duplicateLabel > 0 {
		return nil, validationError("Mã revision thiết kế đã tồn tại", Record{"revision_label": revisionLabel})
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO design_releases (
			project_id, work_item_id, revision_no, revision_label, status,
			change_reason, notes, created_by
		)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,'draft',$5,NULLIF($6,''),NULLIF($7,'')::uuid)
		RETURNING *
	`, projectID, workItemID, revisionNo, revisionLabel, changeReason, notes, actorID)
	if err != nil {
		return nil, err
	}
	releaseID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_created", "design_release", releaseID, "Tạo revision hồ sơ thiết kế", Record{
		"revision_no": revisionNo, "revision_label": revisionLabel, "work_item_id": workItemID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.DesignReleaseByID(ctx, releaseID)
}

func (r *Repository) UpdateDesignReleaseDraft(ctx context.Context, releaseID, revisionLabel, changeReason, notes, actorID, actorName string, expectedValues ...Record) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM design_releases WHERE id=$1 FOR UPDATE`, releaseID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	if len(expectedValues) > 0 && len(expectedValues[0]) > 0 {
		for _, key := range []string{"revision_label", "change_reason", "notes"} {
			if cleanID(expectedValues[0][key]) != cleanID(current[key]) {
				return nil, conflictError("Bản nháp thiết kế đã được cập nhật. Nội dung đang nhập vẫn được giữ; hãy mở lại bản mới trước khi lưu.")
			}
		}
	}
	if fmt.Sprint(current["status"]) != "draft" {
		return nil, validationError("Revision đã phát hành không thể chỉnh sửa", Record{"status": current["status"]})
	}
	projectID := cleanID(current["project_id"])
	var duplicateLabel int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_releases WHERE project_id=$1 AND id<>$2 AND lower(revision_label)=lower($3)`, projectID, releaseID, revisionLabel).Scan(&duplicateLabel); err != nil {
		return nil, err
	}
	if duplicateLabel > 0 {
		return nil, validationError("Mã revision thiết kế đã tồn tại", Record{"revision_label": revisionLabel})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE design_releases
		SET revision_label=$2, change_reason=$3, notes=NULLIF($4,''), updated_at=now()
		WHERE id=$1
	`, releaseID, revisionLabel, changeReason, notes); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_draft_updated", "design_release", releaseID, "Cập nhật revision hồ sơ thiết kế nháp", Record{
		"revision_label": revisionLabel,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.DesignReleaseByID(ctx, releaseID)
}

func (r *Repository) AddDesignReleaseFile(ctx context.Context, releaseID, documentType, name, originalName string, file StoredFileMetadata, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	releaseRows, err := queryTx(ctx, tx, `SELECT * FROM design_releases WHERE id=$1 FOR UPDATE`, releaseID)
	if err != nil {
		return nil, err
	}
	if len(releaseRows) == 0 {
		return nil, sql.ErrNoRows
	}
	release := releaseRows[0]
	if fmt.Sprint(release["status"]) != "draft" {
		return nil, validationError("Chỉ revision nháp mới được thêm tệp", Record{"status": release["status"]})
	}
	projectID := cleanID(release["project_id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO documents (
			project_id,target_type,target_id,name,document_type,status,uploaded_by,
			mime_type,file_size,storage_provider,storage_key,checksum_sha256,
			revision_no,release_status
		)
		VALUES ($1,'design_release',$2,$3,$4,'active',NULLIF($5,'')::uuid,$6,$7,$8,$9,$10,$11,'draft')
		RETURNING *
	`, projectID, releaseID, name, documentType, actorID, file.MIMEType, file.FileSize, file.StorageProvider, file.StorageKey, file.ChecksumSHA256, release["revision_no"])
	if err != nil {
		return nil, err
	}
	documentID := cleanID(rows[0]["id"])
	if err := insertDesignDiscussionUpdateTx(ctx, tx, release, "", "file:"+documentID, actorID, rows[0]); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_file_uploaded", "document", documentID, "Thêm tệp vào revision hồ sơ thiết kế", Record{
		"design_release_id": releaseID, "revision_label": release["revision_label"], "document_type": documentType,
		"name": name, "original_filename": originalName, "checksum_sha256": file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	if err := tracker.commit(tx.Commit); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) ReleaseDesignRelease(ctx context.Context, releaseID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	releaseRows, err := queryTx(ctx, tx, `SELECT * FROM design_releases WHERE id=$1 FOR UPDATE`, releaseID)
	if err != nil {
		return nil, err
	}
	if len(releaseRows) == 0 {
		return nil, sql.ErrNoRows
	}
	release := releaseRows[0]
	if fmt.Sprint(release["status"]) != "draft" {
		return nil, validationError("Chỉ revision nháp mới có thể phát hành", Record{"status": release["status"]})
	}
	projectID := cleanID(release["project_id"])
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "design_release:"+projectID); err != nil {
		return nil, err
	}
	var fileCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents
		WHERE project_id=$1 AND target_type='design_release' AND target_id=$2 AND status='active'
	`, projectID, releaseID).Scan(&fileCount); err != nil {
		return nil, err
	}
	if fileCount == 0 {
		return nil, validationError("Cần ít nhất một tệp trước khi phát hành revision thiết kế")
	}
	previousRows, err := queryTx(ctx, tx, `SELECT id FROM design_releases WHERE project_id=$1 AND status='released' FOR UPDATE`, projectID)
	if err != nil {
		return nil, err
	}
	for _, previous := range previousRows {
		previousID := cleanID(previous["id"])
		if _, err := tx.ExecContext(ctx, `UPDATE design_releases SET status='superseded',updated_at=now() WHERE id=$1`, previousID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE documents SET release_status='superseded' WHERE target_type='design_release' AND target_id=$1 AND status='active'`, previousID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE design_releases
		SET status='released',released_by=NULLIF($2,'')::uuid,released_at=now(),updated_at=now()
		WHERE id=$1
	`, releaseID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE documents
		SET release_status='released',released_by=NULLIF($2,'')::uuid,released_at=now()
		WHERE target_type='design_release' AND target_id=$1 AND status='active'
	`, releaseID, actorID); err != nil {
		return nil, err
	}
	if err := insertDesignDiscussionUpdateTx(ctx, tx, release, "", "released", actorID, nil); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_released", "design_release", releaseID, "Phát hành revision hồ sơ thiết kế", Record{
		"revision_no": release["revision_no"], "revision_label": release["revision_label"], "file_count": fileCount,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.DesignReleaseByID(ctx, releaseID)
}

func (r *Repository) HandoffDesignRelease(ctx context.Context, releaseID, workItemID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	releaseRows, err := queryTx(ctx, tx, `SELECT * FROM design_releases WHERE id=$1 FOR SHARE`, releaseID)
	if err != nil {
		return nil, err
	}
	if len(releaseRows) == 0 {
		return nil, sql.ErrNoRows
	}
	release := releaseRows[0]
	if fmt.Sprint(release["status"]) != "released" {
		return nil, validationError("PM chỉ có thể bàn giao revision thiết kế hiện hành", Record{"status": release["status"]})
	}
	projectID := cleanID(release["project_id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO design_release_handoffs (
			project_id,design_release_id,work_item_id,comment,handed_off_by
		)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),NULLIF($5,'')::uuid)
		RETURNING *
	`, projectID, releaseID, workItemID, comment, actorID)
	if err != nil {
		return nil, err
	}
	handoffID := cleanID(rows[0]["id"])
	if err := insertDesignDiscussionUpdateTx(ctx, tx, release, workItemID, "handoff:"+handoffID, actorID, nil); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_handed_off", "design_release_handoff", handoffID, "PM bàn giao revision hồ sơ thiết kế cho QS", Record{
		"design_release_id": releaseID, "revision_no": release["revision_no"], "revision_label": release["revision_label"], "work_item_id": workItemID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.LatestDesignReleaseHandoff(ctx, projectID)
}

func designReleaseFromRows(rows []Record, status string) Record {
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprint(row["status"]), status) {
			return row
		}
	}
	return nil
}
