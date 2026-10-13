package core

import (
	"context"
	"encoding/json"
)

// Freeze native source facts and selected evidence under the daily transaction.
// The legacy array key stays readable, but new snapshots exclude prototype
// supplemental fields. This helper never modifies stored historical revisions.
func freezeNativeWorkReportEvidence(ctx context.Context, db queryContext, source Record, lock bool) error {
	if err := enrichNativeDailySource(ctx, db, source, lock); err != nil {
		return err
	}
	for _, key := range []string{"bm_gs_json", "bm_gs_version", "bm_gs_updated_by", "bm_gs_updated_at"} {
		delete(source, key)
	}
	ids := workReportSnapshotEvidenceIDs(workReportSnapshot(source))
	source["bm_gs_evidence"] = []Record{}
	if len(ids) == 0 {
		return nil
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	query := `SELECT evidence.id::text,evidence.project_id::text,evidence.work_item_id::text,evidence.evidence_type,evidence.status,evidence.checksum_sha256,evidence.storage_provider,evidence.storage_key,evidence.name,evidence.title,evidence.description,evidence.caption,evidence.file_url,evidence.mime_type,evidence.file_size,evidence.payload_json,evidence.created_at FROM evidence_records evidence WHERE evidence.project_id=$2::uuid AND evidence.id IN (SELECT value::uuid FROM jsonb_array_elements_text($1::jsonb)) ORDER BY evidence.id`
	if lock {
		query += ` FOR UPDATE OF evidence`
	}
	records, err := queryRecords(ctx, db, query, string(raw), cleanID(source["project_id"]))
	if err != nil {
		return err
	}
	if len(records) != len(ids) {
		return validationError("Minh chứng báo cáo không còn thuộc công trình", nil)
	}
	source["bm_gs_evidence"] = records
	return nil
}
