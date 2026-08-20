package core

type EventLogFilter struct {
	Category   string
	HTTPStatus int
	Method     string
	Route      string
	ActorEmail string
	ProjectID  string
	DateFrom   string
	DateTo     string
	Search     string
	Page       int
	PageSize   int
}
