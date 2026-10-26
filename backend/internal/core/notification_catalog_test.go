package core

import (
	"context"
	"reflect"
	"testing"
)

func TestNotificationCatalogReadsPostgresPermissionArrays(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "notification_catalog_test_", []string{"schema_migrations", "notification_event_types"})
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO notification_event_types
			(event_key,domain_key,category_key,category_label,audience_policy_key,action_route_template,permissions_any,enabled,required_delivery)
		VALUES
			('catalog_test.empty','test','catalog_test','Kiểm thử','explicit_users','/notifications','{}',true,false),
			('catalog_test.multiple','test','catalog_test','Kiểm thử','explicit_users','/notifications',ARRAY['work_report.view','ký,tự','quote"value'],true,true),
			('catalog_test.single','test','catalog_test','Kiểm thử','explicit_users','/notifications',ARRAY['pipeline.view'],false,false)
	`)
	if err != nil {
		t.Fatal(err)
	}
	items, err := NewRepository(db).NotificationCatalog(context.Background())
	if err != nil {
		t.Fatalf("catalog must decode arrays through the actual pgx database/sql driver: %v", err)
	}
	want := map[string][]string{
		"catalog_test.empty":    {},
		"catalog_test.multiple": {"work_report.view", "ký,tự", "quote\"value"},
		"catalog_test.single":   {"pipeline.view"},
	}
	for _, item := range items {
		expected, ok := want[item.EventKey]
		if !ok {
			continue
		}
		if !reflect.DeepEqual(item.Permissions, expected) {
			t.Errorf("%s permissions: got %#v want %#v", item.EventKey, item.Permissions, expected)
		}
		if item.EventKey == "catalog_test.multiple" && (!item.RequiredDelivery || !item.Enabled) {
			t.Error("required delivery and enabled flags must remain intact")
		}
		if item.EventKey == "catalog_test.single" && item.Enabled {
			t.Error("catalog must preserve disabled entries, not silently enable them")
		}
		delete(want, item.EventKey)
	}
	if len(want) != 0 {
		t.Fatalf("catalog omitted fixtures: %v", want)
	}
}
