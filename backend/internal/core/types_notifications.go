package core

import "time"

type NotificationFilter struct {
	State     string
	EventKey  string
	Category  string
	ProjectID string
	Search    string
	FromDate  string
	ToDate    string
	Cursor    string
	Limit     int
}

type NotificationPage struct {
	Items      []Record `json:"items"`
	HasMore    bool     `json:"has_more"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type NotificationSummary struct {
	UnreadTotal    int            `json:"unread_total"`
	ActionRequired int            `json:"action_required"`
	ByCategory     map[string]int `json:"by_category"`
	ActorRevision  int64          `json:"actor_revision"`
}

type NotificationCatalogItem struct {
	EventKey         string   `json:"event_key"`
	DomainKey        string   `json:"domain_key"`
	CategoryKey      string   `json:"category_key"`
	CategoryLabel    string   `json:"category_label"`
	Intent           string   `json:"intent"`
	Severity         string   `json:"severity"`
	IconKey          string   `json:"icon_key"`
	ToneKey          string   `json:"tone_key"`
	Enabled          bool     `json:"enabled"`
	RequiredDelivery bool     `json:"required_delivery"`
	Permissions      []string `json:"permissions_any"`
}

type NotificationPreference struct {
	CategoryKey string         `json:"category_key"`
	EventKey    string         `json:"event_key"`
	InApp       bool           `json:"in_app_enabled"`
	Email       bool           `json:"email_enabled"`
	Push        bool           `json:"push_enabled"`
	DigestMode  string         `json:"digest_mode"`
	QuietHours  map[string]any `json:"quiet_hours_json"`
}

type NotificationDomainEvent struct {
	Key            string
	AggregateType  string
	AggregateID    string
	SourceEventID  string
	ProjectID      string
	ActorUserID    string
	Payload        Record
	Audience       Record
	CorrelationKey string
	IdempotencyKey string
	OccurredAt     time.Time
}

type WorkReportSummary struct {
	Scopes       map[string]int `json:"scopes"`
	TotalVisible int            `json:"total_visible"`
}
