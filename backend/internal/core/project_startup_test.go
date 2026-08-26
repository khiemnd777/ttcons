package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectStartupRoleCoverageIncludesWholeTemplateAndAssignedMembers(t *testing.T) {
	templateRows := []Record{
		{
			"node_id": "node-1", "node_role_ids": []string{"PM"},
			"step_id": "step-1", "step_role_ids": []string{},
		},
		{
			"node_id": "node-2", "node_role_ids": []string{"QS"},
			"step_id": "step-2", "step_role_ids": []string{"Kế toán", "Mua hàng"},
		},
		{
			"node_id": "node-2", "node_role_ids": []string{"QS"},
			"step_id": "step-3", "step_role_ids": []string{},
		},
	}
	members := []Record{
		{"user_id": "user-pm", "staff_id": "staff-pm", "full_name": "PM dự án", "role_ids": []string{"PM"}},
		{"user_id": "user-accounting", "staff_id": "staff-accounting", "full_name": "Kế toán dự án", "role_ids": []string{"Kế toán"}},
	}

	coverage := projectStartupRoleCoverage(templateRows, members)
	if len(coverage) != 4 {
		t.Fatalf("role coverage = %#v, want 4 whole-template roles", coverage)
	}

	byRole := map[string]Record{}
	for _, item := range coverage {
		byRole[stringValue(item["role_label"])] = item
	}
	if !boolValue(byRole["PM"]["covered"]) || number(byRole["PM"]["node_count"]) != 1 || number(byRole["PM"]["step_count"]) != 1 {
		t.Fatalf("PM coverage = %#v, want one node, one step and assigned member", byRole["PM"])
	}
	if len(startupRecordSlice(byRole["Kế toán"]["members"])) != 1 || !boolValue(byRole["Kế toán"]["covered"]) {
		t.Fatalf("Kế toán coverage = %#v, want assigned member", byRole["Kế toán"])
	}
	if boolValue(byRole["QS"]["covered"]) || boolValue(byRole["Mua hàng"]["covered"]) {
		t.Fatalf("missing coverage QS=%#v Mua hàng=%#v, want uncovered", byRole["QS"], byRole["Mua hàng"])
	}

	payload, err := json.Marshal(coverage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"members":null`) {
		t.Fatalf("role coverage JSON = %s, want uncovered roles to expose an empty members array", payload)
	}
}
