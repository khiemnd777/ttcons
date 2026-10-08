package core

import (
	"strings"
	"time"
)

func normalizeOptionalDate(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", validationError("Ngày không hợp lệ", Record{field: value})
	}
	return value, nil
}
