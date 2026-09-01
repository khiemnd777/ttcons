package core

import (
	"fmt"
	"strings"
)

func cleanOptionalText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []Record:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case []ActivityAttachmentRequest:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, Record{"type": item.Type, "id": item.ID, "label": item.Label})
		}
		return out
	default:
		return nil
	}
}

func statusIn(status string, allowed ...string) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}
