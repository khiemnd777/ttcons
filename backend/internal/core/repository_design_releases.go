package core

import (
	"context"
	"database/sql"
	"encoding/json"
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
			SELECT jsonb_agg(
				to_jsonb(evidence) || jsonb_build_object(
					'evidence_id', evidence.id,
					'reference_type', 'evidence',
					'document_type', item.document_type,
					'design_release_id', item.design_release_id,
					'design_revision_label', dr.revision_label,
					'design_release_status', dr.status,
					'uploaded_by_name', COALESCE(uploader.full_name, uploader.email, ''),
					'submitted_by_name', COALESCE(submitter.full_name, submitter.email, ''),
					'reviewed_by_name', COALESCE(reviewer.full_name, reviewer.email, '')
				)
				ORDER BY item.position, item.created_at, item.id
			) AS files
			FROM design_release_items item
			JOIN evidence_records evidence
			  ON evidence.id=item.evidence_id
			 AND evidence.project_id=item.project_id
			LEFT JOIN users uploader ON uploader.id=evidence.uploaded_by
			LEFT JOIN users submitter ON submitter.id=evidence.submitted_by
			LEFT JOIN users reviewer ON reviewer.id=evidence.reviewed_by
			WHERE item.project_id=dr.project_id
			  AND item.design_release_id=dr.id
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
			SELECT jsonb_agg(
				to_jsonb(evidence) || jsonb_build_object(
					'evidence_id', evidence.id,
					'reference_type', 'evidence',
					'document_type', item.document_type,
					'design_release_id', item.design_release_id,
					'design_revision_label', dr.revision_label,
					'design_release_status', dr.status,
					'uploaded_by_name', COALESCE(uploader.full_name, uploader.email, ''),
					'submitted_by_name', COALESCE(submitter.full_name, submitter.email, ''),
					'reviewed_by_name', COALESCE(reviewer.full_name, reviewer.email, '')
				)
				ORDER BY item.position, item.created_at, item.id
			) AS files
			FROM design_release_items item
			JOIN evidence_records evidence
			  ON evidence.id=item.evidence_id
			 AND evidence.project_id=item.project_id
			LEFT JOIN users uploader ON uploader.id=evidence.uploaded_by
			LEFT JOIN users submitter ON submitter.id=evidence.submitted_by
			LEFT JOIN users reviewer ON reviewer.id=evidence.reviewed_by
			WHERE item.project_id=dr.project_id
			  AND item.design_release_id=dr.id
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

func (r *Repository) AddDesignReleaseFile(ctx context.Context, releaseID, documentType, originalName string, req WorkItemEvidenceRequest, actorID, actorName string) (result Record, err error) {
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
	workItemID := cleanID(release["work_item_id"])
	if workItemID == "" {
		return nil, validationError("Revision thiết kế cần liên kết đầu việc trước khi tải hồ sơ")
	}
	batchID := cleanID(req.Payload["discussion_batch_id"])
	if batchID != "" {
		var completed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_work_updates WHERE source_event_key=$1)`, discussionWorkUpdateEventKey("design_release", releaseID, "files:"+batchID)).Scan(&completed); err != nil {
			return nil, err
		}
		if completed {
			return nil, conflictError("Lô tải hồ sơ đã được ghi nhận trong trao đổi")
		}
	}
	payload := req.Payload
	if payload == nil {
		payload = Record{}
	}
	payload["source_type"] = "design_release"
	payload["design_release_id"] = releaseID
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO evidence_records (
			work_item_id,project_id,target_type,target_id,evidence_type,document_type,
			name,title,description,file_url,mime_type,file_size,storage_provider,storage_key,
			checksum_sha256,caption,revision_no,visibility,payload_json,status,uploaded_by,submitted_by
		)
		VALUES (
			$1,$2,'work_item',$1,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),
			NULLIF($9,''),NULLIF($10,0),$11,NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),
			$15,'internal',$16::jsonb,'draft',NULLIF($17,'')::uuid,NULLIF($17,'')::uuid
		)
		RETURNING *
	`, workItemID, projectID, req.EvidenceType, documentType, req.Name, req.Title, req.Description, req.FileURL,
		req.MIMEType, req.FileSize, req.StorageProvider, req.StorageKey, req.ChecksumSHA256, req.Caption,
		fmt.Sprint(release["revision_label"]), string(rawPayload), actorID)
	if err != nil {
		return nil, err
	}
	evidenceID := cleanID(rows[0]["id"])
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO design_release_items (project_id,design_release_id,evidence_id,document_type,position)
		SELECT $1,$2,$3,$4,COALESCE(MAX(position),-1)+1
		FROM design_release_items
		WHERE design_release_id=$2
	`, projectID, releaseID, evidenceID, documentType); err != nil {
		return nil, err
	}
	if batchID == "" {
		if err := insertDesignDiscussionUpdateTx(ctx, tx, release, "", "file:"+evidenceID, actorID, rows[0]); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items SET updated_by=NULLIF($2,'')::uuid,updated_at=now() WHERE id=$1
	`, workItemID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,comment,metadata_json)
		VALUES ($1,$2,'work_item',$1,NULLIF($3,'')::uuid,'evidence_added',$4,$5::jsonb)
	`, workItemID, projectID, actorID, req.Name, fmt.Sprintf(`{"evidence_type":%q,"source_type":"design_release"}`, req.EvidenceType)); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_file_uploaded", "evidence_record", evidenceID, "Thêm hồ sơ vào revision thiết kế", Record{
		"design_release_id": releaseID, "revision_label": release["revision_label"], "document_type": documentType,
		"name": req.Name, "original_filename": originalName, "checksum_sha256": req.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getEvidenceDetail(ctx, tx, evidenceID)
	}, tx.Commit)
}

func (r *Repository) CompleteDesignReleaseUploadBatch(ctx context.Context, releaseID, batchID string, evidenceIDs []string, actorID string) (Record, error) {
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
		return nil, validationError("Chỉ revision nháp mới được ghi nhận lô tệp", Record{"status": release["status"]})
	}
	rows, err := queryTx(ctx, tx, `
		SELECT evidence.*
		FROM design_release_items item
		JOIN evidence_records evidence
		  ON evidence.id=item.evidence_id
		 AND evidence.project_id=item.project_id
		WHERE item.project_id=$1
		  AND item.design_release_id=$2
		  AND evidence.payload_json->>'discussion_batch_id'=$3
		  AND evidence.uploaded_by=$4
	`, release["project_id"], releaseID, batchID, actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(evidenceIDs) {
		return nil, validationError("Danh sách tệp thành công không khớp lô tải hồ sơ", Record{"expected": len(rows), "received": len(evidenceIDs)})
	}
	byID := make(map[string]Record, len(rows))
	for _, row := range rows {
		byID[cleanID(row["id"])] = row
	}
	ordered := make([]Record, 0, len(evidenceIDs))
	for _, evidenceID := range evidenceIDs {
		row, exists := byID[evidenceID]
		if !exists {
			return nil, validationError("Tệp không thuộc lô tải hồ sơ này", Record{"evidence_id": evidenceID})
		}
		ordered = append(ordered, row)
	}
	if err := insertDesignUploadBatchDiscussionUpdateTx(ctx, tx, release, batchID, actorID, ordered); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"batch_id": batchID, "file_count": len(ordered)}, nil
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
		SELECT COUNT(*) FROM design_release_items
		WHERE project_id=$1 AND design_release_id=$2
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
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE design_releases
		SET status='released',released_by=NULLIF($2,'')::uuid,released_at=now(),updated_at=now()
		WHERE id=$1
	`, releaseID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE evidence_records evidence
		SET status='submitted',updated_at=now()
		FROM design_release_items item
		WHERE item.design_release_id=$1
		  AND item.evidence_id=evidence.id
		  AND item.project_id=evidence.project_id
		  AND evidence.status='draft'
	`, releaseID); err != nil {
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
		return nil, validationError("Quản lý dự án chỉ có thể bàn giao revision thiết kế hiện hành", Record{"status": release["status"]})
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
	if _, err := tx.ExecContext(ctx, `
		UPDATE evidence_records evidence
		SET status='accepted',reviewed_by=NULLIF($2,'')::uuid,reviewed_at=now(),updated_at=now()
		FROM design_release_items item
		WHERE item.design_release_id=$1
		  AND item.evidence_id=evidence.id
		  AND item.project_id=evidence.project_id
		  AND evidence.status IN ('draft','submitted','accepted')
	`, releaseID, actorID); err != nil {
		return nil, err
	}
	if err := insertDesignDiscussionUpdateTx(ctx, tx, release, workItemID, "handoff:"+handoffID, actorID, nil); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "design_release_handed_off", "design_release_handoff", handoffID, "Quản lý dự án bàn giao revision hồ sơ thiết kế cho QS", Record{
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
