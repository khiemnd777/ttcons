package core

import "strings"

// Presentation uses the six-code contract published in V20. Persisted answers
// and immutable snapshots retain their original codes and free text.
func constructionReportWeatherLabel(value any) string {
	value = nativeBMGSAnswer(value)
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "Chưa ghi thời tiết"
	}
	if label, found := map[string]string{
		"normal": "Bình thường", "hot": "Nắng nóng", "rain": "Mưa",
		"wind": "Gió mạnh", "restricted_access": "Hạn chế mặt bằng", "other": "Khác",
	}[strings.TrimSpace(text)]; found {
		return label
	}
	return text
}
