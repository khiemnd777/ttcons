package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// enrichNativeDailySource reuses authorized native source records. The caller
// supplies an authorized current source; historical snapshots never call this.
// freeze=true captures the facts under the existing report-delivery transaction.
func enrichNativeDailySource(ctx context.Context, db queryContext, source Record, freeze bool) error {
	// Repository read helpers have no viewing actor. Only delivery capture may
	// derive the source author; interactive reads use the actor-aware entrypoint.
	if !freeze {
		// Restore frozen facts before the repository removes comparison metadata.
		// The empty actor prevents every live cross-module source query.
		return enrichNativeDailySourceForActor(ctx, db, source, false, "")
	}
	actorID := cleanID(source["reported_by"])
	if actorID == "" {
		actorID = cleanID(source["author_id"])
	}
	return enrichNativeDailySourceForActor(ctx, db, source, freeze, actorID)
}

func enrichNativeDailySourceForActor(ctx context.Context, db queryContext, source Record, freeze bool, actorID string) error {
	if !freeze {
		status := cleanText(source["delivery_status"])
		if status == "" {
			status = cleanText(source["status"])
		}
		if statusIn(status, "submitted", "acknowledged") {
			// Old revisions with no reused facts must stay exactly that way.
			if snapshot := recordFromAny(source["delivery_snapshot_json"]); len(snapshot) > 0 {
				source["structured_json"] = snapshot["structured_json"]
			}
			return nil
		}
	}
	projectID, workItemID := cleanID(source["project_id"]), cleanID(source["work_item_id"])
	if !validActivityUUID(projectID) || !validActivityUUID(workItemID) {
		return nil
	}
	if !validActivityUUID(actorID) {
		return nil
	}
	repo := &Repository{reader: db}
	canView, err := repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return err
	}
	if !canView {
		return nil
	}
	answers := bmgsCopy(recordFromAny(source["structured_json"]))
	formFrozen, err := restoreNativeWorkflowDailySources(ctx, db, source, freeze)
	if err != nil {
		return err
	}
	if formFrozen {
		answers = bmgsCopy(recordFromAny(source["structured_json"]))
		if freeze {
			return nil
		}
	}
	canReceive, err := repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return err
	}
	if !formFrozen && canReceive && len(workflowAnswerRecordList(answers["material_arrival_items"])) == 0 {
		date := bmgsBusinessDate(source["report_date"])
		if date == "" {
			date = bmgsBusinessDate(answers["report_date"])
		}
		if date != "" {
			profile, err := repo.WorkItemAccessProfile(ctx, actorID)
			if err != nil {
				return err
			}
			args := append([]any{projectID, date}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
			query := `SELECT item.id::text AS source_item_id,receipt.id::text AS source_id,
			 receipt.work_item_id::text AS source_work_item_id,COALESCE(item.work_item_id,receipt.work_item_id)::text AS source_item_work_item_id,
			 item.material_name,item.unit,item.delivered_quantity AS quantity,item.comment AS note,
			 receipt.received_at FROM incoming_acceptance_records receipt
			 JOIN incoming_acceptance_items item ON item.incoming_acceptance_id=receipt.id AND item.project_id=receipt.project_id
			 LEFT JOIN work_items parent ON parent.id=receipt.work_item_id AND parent.project_id=receipt.project_id
			 LEFT JOIN work_items effective ON effective.id=COALESCE(item.work_item_id,receipt.work_item_id) AND effective.project_id=receipt.project_id
			 JOIN projects project ON project.id=receipt.project_id AND project.status <> 'cancelled'
			 WHERE receipt.project_id=$1::uuid AND receipt.received_at=$2::date AND receipt.status='submitted'
			 AND (receipt.work_item_id IS NULL OR (parent.id IS NOT NULL AND (` + workItemScopeSQL("parent", 3) + `)))
			 AND (COALESCE(item.work_item_id,receipt.work_item_id) IS NULL OR (effective.id IS NOT NULL AND (` + workItemScopeSQL("effective", 3) + `)))
			 ORDER BY receipt.id,item.item_index,item.id LIMIT 201`
			if freeze {
				query += ` FOR SHARE OF receipt,item`
			}
			rows, err := queryRecords(ctx, db, query, args...)
			if err != nil {
				return err
			}
			if len(rows) > 200 {
				return validationError("Nguồn báo cáo có hơn 200 dòng vật tư nhập hiện trường; thu hẹp phạm vi báo cáo")
			}
			if len(rows) > 0 {
				for _, row := range rows {
					row["row_id"] = row["source_item_id"]
					row["source_type"] = "incoming_acceptance"
					// Supplier is not origin; accepted quantity is not delivered quantity.
				}
				answers["material_arrival_items"] = rows
			}
		}
	}
	instanceID := cleanID(source["workflow_form_instance_id"])
	next := workflowAnswerRecordList(answers["next_plan_items"])
	if validActivityUUID(instanceID) && len(next) > 0 {
		// Workflow normalization omits optional blanks. The same explicit-clear
		// observation must survive the synchronized Operations/export path.
		clears, err := queryRecords(ctx, db, `SELECT event.metadata_json->'native_daily_sources' AS facts
		 FROM workflow_form_instances instance JOIN workflow_form_events event ON event.instance_id=instance.id
		 AND event.project_id=instance.project_id AND event.work_item_id=instance.work_item_id
		 WHERE instance.id=$1::uuid AND instance.project_id=$2::uuid AND instance.work_item_id=$3::uuid
		 AND instance.status='submitted' AND event.event_type IN ('submitted','amended')
		 AND event.metadata_json->'native_daily_sources'->>'form_revision_no'=instance.current_revision::text
		 ORDER BY event.created_at DESC,event.id DESC LIMIT 1`, instanceID, projectID, workItemID)
		if err != nil {
			return err
		}
		if len(clears) > 0 {
			facts := recordFromAny(clears[0]["facts"])
			if number(facts["version"]) != 1 || cleanID(facts["project_id"]) != projectID || cleanID(facts["work_item_id"]) != workItemID || cleanID(facts["form_instance_id"]) != instanceID {
				return validationError("Nguồn đã chốt không khớp phiếu báo cáo")
			}
			next = nativeDailyApplyFrozenClears(next, workflowAnswerRecordList(recordFromAny(facts["structured_json"])["next_plan_items"]))
		}
		profile, err := repo.WorkItemAccessProfile(ctx, actorID)
		if err != nil {
			return err
		}
		args := append([]any{projectID, instanceID, workItemID}, workItemScopeArgs(profile, profileCanObserveAllProjectWork(profile), true)...)
		query := `SELECT task.id::text,task.status,creation.metadata_json FROM work_items task
		 JOIN work_item_activities creation ON creation.work_item_id=task.id AND creation.project_id=task.project_id AND creation.activity_type='created'
		 WHERE task.project_id=$1::uuid AND task.source_type='workflow_form_action_item' AND task.source_id=$2::uuid
		 AND task.status <> 'cancelled' AND creation.metadata_json->>'workflow_form_instance_id'=$2::text
		 AND creation.metadata_json->>'parent_work_item_id'=$3
		 AND (` + workItemScopeSQL("task", 4) + `)
		 ORDER BY task.id,creation.id LIMIT 1001`
		if freeze {
			query += ` FOR SHARE OF task,creation`
		}
		tasks, err := queryRecords(ctx, db, query, args...)
		if err != nil {
			return err
		}
		if len(tasks) > 1000 {
			return validationError("Nguồn báo cáo có hơn 1000 đầu việc kế hoạch; thu hẹp phạm vi báo cáo")
		}
		answers["next_plan_items"] = nativeDailyNextPlanStates(next, tasks)
	}
	source["structured_json"] = answers
	return nil
}

func nativeDailyApplyFrozenClears(rows, frozen []Record) []Record {
	result := make([]Record, len(rows))
	for index, original := range rows {
		row := bmgsCopy(original)
		result[index] = row
		if _, explicit := row["state"]; explicit {
			continue
		}
		id := cleanID(row["row_id"])
		if !validActivityUUID(id) {
			continue
		}
		count := 0
		for _, candidate := range rows {
			if cleanID(candidate["row_id"]) == id {
				count++
			}
		}
		if count != 1 {
			continue
		}
		var matched Record
		for _, candidate := range frozen {
			if cleanID(candidate["row_id"]) == id {
				if matched != nil {
					matched = nil
					break
				}
				matched = candidate
			}
		}
		if cleanText(recordFromAny(matched["state_source"])["source_type"]) == "explicit_clear" {
			row["state"] = ""
			row["state_source"] = matched["state_source"]
		}
	}
	return result
}

// Lifecycle state is a task observation, never progress or an issued date.
// Preserve each nonterminal phase rather than inferring completion percentage.
func nativeDailyTaskState(status string) string {
	return map[string]string{
		"pending": "Chưa sẵn sàng", "ready": "Sẵn sàng", "in_progress": "Đang làm",
		"waiting_evidence": "Chờ minh chứng", "waiting_review": "Chờ kiểm tra",
		"waiting_approval": "Chờ duyệt", "blocked": "Bị chặn",
		"needs_changes": "Cần chỉnh sửa", "completed": "Hoàn thành",
	}[status]
}

func nativeDailyNextPlanStates(rows, tasks []Record) []Record {
	result := make([]Record, len(rows))
	for index, original := range rows {
		row := bmgsCopy(original)
		result[index] = row
		if _, explicit := row["state"]; explicit {
			continue // Empty text is an intentional clear.
		}
		matches := []Record{}
		for _, task := range tasks {
			metadata := recordFromAny(task["metadata_json"])
			captured := recordFromAny(metadata["next_plan_item"])
			rowID, capturedID := cleanID(row["row_id"]), cleanID(captured["row_id"])
			if rowID != "" {
				if rowID != capturedID {
					continue
				}
				count := 0
				for _, candidate := range rows {
					if cleanID(candidate["row_id"]) == rowID {
						count++
					}
				}
				if count != 1 {
					continue
				}
			} else {
				// Legacy rows have no stable ID. Require the original index AND
				// the complete unchanged original row; never match just a title.
				position, exists := metadata["next_plan_item_index"]
				originalIndex, err := strconv.Atoi(fmt.Sprint(position))
				if !exists || err != nil || originalIndex != index || capturedID != "" || !nativeDailyRecordsEqual(row, captured) {
					continue
				}
				identical := 0
				for _, candidate := range rows {
					if nativeDailyRecordsEqual(candidate, captured) {
						identical++
					}
				}
				if identical != 1 {
					continue
				}
			}
			if nativeDailyTaskState(cleanText(task["status"])) != "" {
				matches = append(matches, task)
			}
		}
		if len(matches) == 1 {
			task := matches[0]
			row["state"] = nativeDailyTaskState(cleanText(task["status"]))
			row["state_source"] = Record{"source_type": "work_item", "source_id": cleanID(task["id"]), "status": cleanText(task["status"])}
		}
	}
	return result
}

func nativeDailyRecordsEqual(a, b Record) bool {
	// Normalize JSON numeric representations before comparing legacy captures.
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	var left, right any
	if json.Unmarshal(x, &left) != nil || json.Unmarshal(y, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}
