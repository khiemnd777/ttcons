package core

// Assign identities before serializing canonical answers. Existing IDs travel
// through edits and amendments; only newly added rows receive new IDs.
func normalizeBMGSWorkflowCrewIDs(answers Record) error {
	if _, exists := answers["crew_items"]; !exists {
		return nil
	}
	rows := workflowAnswerRecordList(answers["crew_items"])
	seen := map[string]bool{}
	for _, row := range rows {
		rowID := cleanID(row["row_id"])
		if rowID == "" {
			rowID = newID()
		}
		if !validActivityUUID(rowID) || seen[rowID] {
			return validationError("Mã tổ đội không hợp lệ hoặc trùng", nil)
		}
		row["row_id"] = rowID
		seen[rowID] = true
	}
	answers["crew_items"] = rows
	return nil
}
