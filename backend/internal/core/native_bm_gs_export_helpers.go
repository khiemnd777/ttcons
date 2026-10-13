package core

import "time"

const bmgsMaxRawImageBytes = 12 * 1024 * 1024

func bmgsValidateDates(from, to string) error {
	for _, value := range []string{from, to} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return validationError("Ngày phải có định dạng YYYY-MM-DD")
			}
		}
	}
	if from != "" && to != "" && from > to {
		return validationError("Ngày bắt đầu phải trước ngày kết thúc")
	}
	return nil
}

func bmgsFrozenEvidence(snapshot Record, id string, live Record) Record {
	for _, frozen := range workflowAnswerRecordList(snapshot["bm_gs_evidence"]) {
		if cleanID(frozen["id"]) == id {
			return bmgsCopy(frozen)
		}
	}
	// Older immutable reports lack enrichment snapshots. Keep only protected file
	// identity, never substitute a current caption/location/phase into history.
	return Record{"id": id, "name": id, "project_id": snapshot["project_id"], "work_item_id": snapshot["work_item_id"], "evidence_type": live["evidence_type"], "mime_type": live["mime_type"], "file_size": live["file_size"], "checksum_sha256": live["checksum_sha256"]}
}
