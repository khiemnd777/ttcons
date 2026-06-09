package core

type Record map[string]any

type ColumnSpec struct {
	Name string
	Cast string
}

func col(name string) ColumnSpec {
	return ColumnSpec{Name: name}
}

func jsonCol(name string) ColumnSpec {
	return ColumnSpec{Name: name, Cast: "jsonb"}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ActionRequest struct {
	Comment string `json:"comment"`
	Reason  string `json:"reason"`
}

type StockActionRequest struct {
	ProjectID     string  `json:"project_id"`
	WarehouseID   string  `json:"warehouse_id"`
	MaterialID    string  `json:"material_id"`
	MovementType  string  `json:"movement_type"`
	Quantity      float64 `json:"quantity"`
	Unit          string  `json:"unit"`
	Reason        string  `json:"reason"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   string  `json:"reference_id"`
}
