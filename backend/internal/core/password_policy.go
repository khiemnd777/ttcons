package core

import "strings"

const (
	passwordMinimumCodePoints = 8
	passwordMaximumUTF8Bytes  = 72
)

func validateNewPassword(password, confirmation string) error {
	if password != confirmation {
		return validationError("Mật khẩu xác nhận không khớp", Record{"field": "new_password_confirmation"})
	}
	return validatePasswordValue(password)
}

func validatePasswordValue(password string) error {
	return validatePasswordField(password, "new_password", "Mật khẩu mới")
}

func validatePasswordField(password, field, label string) error {
	if password == "" {
		return validationError(label+" là bắt buộc", Record{"field": field})
	}
	if strings.TrimSpace(password) == "" {
		return validationError(label+" không được chỉ chứa khoảng trắng", Record{"field": field})
	}
	if len([]rune(password)) < passwordMinimumCodePoints {
		return validationError(label+" phải có ít nhất 8 ký tự", Record{"field": field})
	}
	if len([]byte(password)) > passwordMaximumUTF8Bytes {
		return validationError(label+" không được vượt quá 72 byte", Record{"field": field})
	}
	return nil
}
