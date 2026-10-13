package core

import (
	"context"
	"database/sql"
	"encoding/json"
)

// A synchronized daily report consumes the exact submitted form revision.
// Missing legacy provenance remains missing; delivery must not recapture receipts.
// Current BM06 observations can still be compared with generated task state.
func restoreNativeWorkflowDailySources(ctx context.Context, db queryContext, source Record, freeze bool) (bool, error) {
	instanceID := cleanID(source["workflow_form_instance_id"])
	// Form-event capture builds a source without a report ID. That transaction
	// must derive the new revision, not restore the previous submitted revision.
	if !validActivityUUID(instanceID) || !validActivityUUID(cleanID(source["id"])) {
		return false, nil
	}
	rows, err := queryRecords(ctx, db, `SELECT instance.current_revision,event.metadata_json->'native_daily_sources' AS facts
	 FROM workflow_form_instances instance LEFT JOIN LATERAL (
	 SELECT metadata_json FROM workflow_form_events
	 WHERE instance_id=instance.id AND project_id=instance.project_id AND work_item_id=instance.work_item_id
	 AND event_type IN ('submitted','amended') AND metadata_json->'native_daily_sources'->>'form_revision_no'=instance.current_revision::text
	 ORDER BY created_at DESC,id DESC LIMIT 1) event ON true
	 WHERE instance.id=$1 AND instance.project_id=$2 AND instance.work_item_id=$3 AND instance.status='submitted'`,
		instanceID, cleanID(source["project_id"]), cleanID(source["work_item_id"]))
	if err != nil || len(rows) == 0 {
		return false, err
	}
	answers := bmgsCopy(recordFromAny(source["structured_json"]))
	facts := recordFromAny(rows[0]["facts"])
	if len(facts) > 0 && (number(facts["version"]) != 1 || cleanID(facts["project_id"]) != cleanID(source["project_id"]) || cleanID(facts["work_item_id"]) != cleanID(source["work_item_id"]) || cleanID(facts["form_instance_id"]) != instanceID || number(facts["form_revision_no"]) != number(rows[0]["current_revision"])) {
		return false, validationError("Nguồn đã chốt không khớp phiên bản phiếu báo cáo")
	}
	keys := []string{"material_arrival_items"}
	if freeze {
		keys = append(keys, "next_plan_items")
	}
	for _, key := range keys {
		if value, exists := recordFromAny(facts["structured_json"])[key]; exists {
			answers[key] = value
		}
	}
	source["structured_json"] = answers
	return true, nil
}

// Source facts share the form submission transaction and revision. They are
// internal provenance, not new form answers or live substitutes for history.
func freezeNativeWorkflowDailySources(ctx context.Context, tx *sql.Tx, instance, answers Record, revision int, actorID string) (Record, error) {
	if !nativeBMGSIsDailyForm(instance) {
		return nil, nil
	}
	captured := bmgsCopy(answers)
	// Canonical optional-string normalization removes blanks. Recover a manual
	// clear from the locked previous answers, and carry its internal provenance
	// through later amendments so an unrelated edit cannot rederive task state.
	if cleanText(instance["status"]) == "submitted" && revision > int(number(instance["current_revision"])) {
		previousRaw := workflowAnswerRecordList(recordFromAny(instance["answers_snapshot_json"])["next_plan_items"])
		previousFacts := []Record{}
		events, err := queryRecords(ctx, tx, `SELECT metadata_json->'native_daily_sources' AS facts FROM workflow_form_events
		 WHERE instance_id=$1 AND project_id=$2 AND work_item_id=$3 AND event_type IN ('submitted','amended')
		 AND metadata_json->'native_daily_sources'->>'form_revision_no'=$4 ORDER BY created_at DESC,id DESC LIMIT 1`,
			cleanID(instance["id"]), cleanID(instance["project_id"]), cleanID(instance["work_item_id"]), cleanText(instance["current_revision"]))
		if err != nil {
			return nil, err
		}
		if len(events) > 0 {
			facts := recordFromAny(events[0]["facts"])
			if number(facts["version"]) != 1 || cleanID(facts["project_id"]) != cleanID(instance["project_id"]) || cleanID(facts["work_item_id"]) != cleanID(instance["work_item_id"]) || cleanID(facts["form_instance_id"]) != cleanID(instance["id"]) || number(facts["form_revision_no"]) != number(instance["current_revision"]) {
				return nil, validationError("Nguồn đã chốt không khớp phiên bản phiếu báo cáo")
			}
			previousFacts = workflowAnswerRecordList(recordFromAny(facts["structured_json"])["next_plan_items"])
		}
		rows := workflowAnswerRecordList(answers["next_plan_items"])
		// Stable identity is required for amended native rows. Legacy rows can
		// match only an unchanged unique full row at its original index; state
		// and its internal provenance are the sole excluded fields.
		match := func(previous []Record, row Record, index int) Record {
			id := cleanID(row["row_id"])
			if id != "" {
				var found Record
				for _, old := range previous {
					if cleanID(old["row_id"]) == id {
						if found != nil {
							return nil
						}
						found = old
					}
				}
				return found
			}
			withoutState := func(item Record) Record {
				copy := bmgsCopy(item)
				delete(copy, "state")
				delete(copy, "state_source")
				return copy
			}
			if index >= len(previous) || cleanID(previous[index]["row_id"]) != "" || !nativeDailyRecordsEqual(withoutState(row), withoutState(previous[index])) {
				return nil
			}
			count := 0
			for _, old := range previous {
				if nativeDailyRecordsEqual(withoutState(row), withoutState(old)) {
					count++
				}
			}
			if count != 1 {
				return nil
			}
			count = 0
			for _, current := range rows {
				if nativeDailyRecordsEqual(withoutState(row), withoutState(current)) {
					count++
				}
			}
			if count != 1 {
				return nil
			}
			return previous[index]
		}
		copies := make([]Record, len(rows))
		for index, original := range rows {
			row := bmgsCopy(original)
			copies[index] = row
			if cleanText(row["state"]) != "" {
				continue
			}
			previous := match(previousRaw, row, index)
			frozen := match(previousFacts, row, index)
			manualDisappeared := cleanText(previous["state"]) != "" && cleanText(recordFromAny(previous["state_source"])["source_type"]) != "work_item"
			wasCleared := cleanText(recordFromAny(frozen["state_source"])["source_type"]) == "explicit_clear"
			if manualDisappeared || wasCleared {
				row["state"] = ""
				row["state_source"] = Record{"source_type": "explicit_clear", "source_id": cleanID(instance["id"]), "row_id": cleanID(row["row_id"])}
			}
		}
		if _, exists := answers["next_plan_items"]; exists {
			captured["next_plan_items"] = copies
		}
	}
	source := Record{
		"project_id": instance["project_id"], "work_item_id": instance["work_item_id"],
		"workflow_form_instance_id": instance["id"], "report_date": answers["report_date"],
		"structured_json": captured,
	}
	if err := enrichNativeDailySourceForActor(ctx, tx, source, true, actorID); err != nil {
		return nil, err
	}
	return Record{
		"version": 1, "project_id": instance["project_id"], "work_item_id": instance["work_item_id"],
		"form_instance_id": instance["id"], "form_revision_no": revision,
		"structured_json": source["structured_json"],
	}, nil
}

func saveNativeWorkflowDailySourcesTx(ctx context.Context, tx *sql.Tx, eventID string, instance, answers Record, revision int, actorID string) error {
	facts, err := freezeNativeWorkflowDailySources(ctx, tx, instance, answers, revision, actorID)
	if err != nil || facts == nil {
		return err
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE workflow_form_events SET metadata_json=metadata_json || jsonb_build_object('native_daily_sources',$2::jsonb) WHERE id=$1`, eventID, string(raw))
	return err
}

func (s *Service) nativeWorkflowDailySources(ctx context.Context, source Record, actorID string) error {
	if cleanText(source["status"]) != "submitted" {
		source["workflow_form_instance_id"] = source["id"]
		return enrichNativeDailySourceForActor(ctx, s.repo.querySource(), source, false, actorID)
	}
	rows, err := s.repo.Query(ctx, `SELECT metadata_json->'native_daily_sources' AS facts
		FROM workflow_form_events WHERE instance_id=$1 AND project_id=$2 AND work_item_id=$3
		AND event_type IN ('submitted','amended')
		AND metadata_json->'native_daily_sources'->>'form_revision_no'=$4
		ORDER BY created_at DESC,id DESC LIMIT 1`, cleanID(source["id"]), cleanID(source["project_id"]), cleanID(source["work_item_id"]), cleanText(source["current_revision"]))
	if err != nil || len(rows) == 0 {
		return err // Older revisions have no source-fact snapshot; never supplement them live.
	}
	facts := recordFromAny(rows[0]["facts"])
	if number(facts["version"]) != 1 || cleanID(facts["project_id"]) != cleanID(source["project_id"]) || cleanID(facts["work_item_id"]) != cleanID(source["work_item_id"]) || cleanID(facts["form_instance_id"]) != cleanID(source["id"]) || number(facts["form_revision_no"]) != number(source["current_revision"]) {
		return validationError("Nguồn đã chốt không khớp phiên bản phiếu báo cáo")
	}
	answers := bmgsCopy(recordFromAny(source["structured_json"]))
	for _, key := range []string{"material_arrival_items", "next_plan_items"} {
		if value, exists := recordFromAny(facts["structured_json"])[key]; exists {
			answers[key] = value
		}
	}
	source["structured_json"] = answers
	return nil
}

// Compare under the source author's capture scope without returning newly
// derived child facts to a reader who may only view the parent report.
func annotateNativeWorkReportDeliverySourceChanged(ctx context.Context, db queryContext, source Record) error {
	comparison := bmgsCopy(source)
	status := cleanText(source["delivery_status"])
	if status == "" {
		status = cleanText(source["status"])
	}
	if status == "needs_changes" {
		actorID := cleanID(source["reported_by"])
		if actorID == "" {
			actorID = cleanID(source["author_id"])
		}
		// Resubmission eligibility must compare the same form facts that Send
		// freezes. Live generated-task state remains visible in BM06, but only
		// a form amendment can capture that observation into a new revision.
		formFrozen, err := restoreNativeWorkflowDailySources(ctx, db, comparison, true)
		if err != nil {
			return err
		}
		if !formFrozen {
			if err := enrichNativeDailySourceForActor(ctx, db, comparison, false, actorID); err != nil {
				return err
			}
		}
	}
	if err := annotateWorkReportDeliverySourceChanged(comparison); err != nil {
		return err
	}
	source["delivery_source_changed"] = comparison["delivery_source_changed"]
	delete(source, "delivery_snapshot_json")
	delete(source, "delivery_snapshot_sha256")
	return nil
}
