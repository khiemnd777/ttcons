package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("email").Unique(),
		field.String("password_hash").Sensitive(),
		field.String("full_name"),
		field.String("status").Default("active"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now),
	}
}

type Project struct{ ent.Schema }

func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name"),
		field.String("code").Unique(),
		field.String("client_name"),
		field.String("address").Optional(),
		field.Float("estimated_budget").Default(0),
		field.Float("contract_amount").Default(0),
		field.String("status").Default("draft"),
		field.Int("progress_percent").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now),
	}
}

type PipelineTemplate struct{ ent.Schema }

func (PipelineTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name"),
		field.String("description").Optional(),
		field.String("project_type").Optional(),
		field.Int("version").Default(1),
		field.String("status").Default("draft"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now),
	}
}

type PipelineTemplateNode struct{ ent.Schema }

func (PipelineTemplateNode) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name"),
		field.String("description").Optional(),
		field.String("type").Default("custom"),
		field.Int("order_index").Default(0),
		field.JSON("responsible_role_ids", []string{}),
		field.String("required_permission").Optional(),
		field.JSON("entry_conditions", map[string]any{}),
		field.JSON("exit_conditions", map[string]any{}),
		field.String("approval_rule").Optional(),
		field.Bool("rework_enabled").Default(false),
		field.Bool("replanning_enabled").Default(false),
		field.String("fail_path").Default("previous"),
		field.String("status").Default("active"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now),
	}
}

type AuditLog struct{ ent.Schema }

func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("event_type"),
		field.String("target_type").Optional(),
		field.String("description"),
		field.JSON("metadata_json", map[string]any{}),
		field.Time("created_at").Default(time.Now),
	}
}
