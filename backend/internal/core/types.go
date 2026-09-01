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
