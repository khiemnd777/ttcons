package core

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateOwnProfileRequest struct {
	FullName        string `json:"full_name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	CurrentPassword string `json:"current_password"`
}

type ChangeOwnPasswordRequest struct {
	CurrentPassword         string `json:"current_password"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}

type CreateStaffRequest struct {
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
	Position      string `json:"position"`
	Status        string `json:"status"`
	Password      string `json:"password"`
	RecoveryEmail string `json:"recovery_email"`
}

type StaffAccountImportRowRequest struct {
	RowNumber     int    `json:"row_number"`
	FullName      string `json:"full_name"`
	LoginEmail    string `json:"login_email"`
	Password      string `json:"password"`
	RecoveryEmail string `json:"recovery_email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
	Position      string `json:"position"`
	RoleCode      string `json:"role_code"`
}

type StaffAccountImportPreviewRequest struct {
	SourceFilename  string                         `json:"source_filename"`
	SourceSizeBytes int64                          `json:"source_size_bytes"`
	Rows            []StaffAccountImportRowRequest `json:"rows"`
}

type StaffAccountImportExecuteRequest struct {
	StaffAccountImportPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type StaffDeletionPreviewRequest struct {
	StaffIDs []string `json:"staff_ids"`
}

type StaffDeletionExecuteRequest struct {
	StaffDeletionPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PartnerDeletionPreviewRequest struct {
	PartnerIDs []string `json:"partner_ids"`
}

type PartnerDeletionExecuteRequest struct {
	PartnerDeletionPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ProjectDeletionPreviewRequest struct {
	ProjectIDs []string `json:"project_ids"`
}

type ProjectDeletionExecuteRequest struct {
	ProjectDeletionPreviewRequest
	PreviewHash    string `json:"preview_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

type UpdateStaffRequest struct {
	UserID                  *string `json:"user_id"`
	FullName                *string `json:"full_name"`
	Email                   *string `json:"email"`
	Phone                   *string `json:"phone"`
	Department              *string `json:"department"`
	Position                *string `json:"position"`
	Status                  *string `json:"status"`
	NewPassword             string  `json:"new_password"`
	NewPasswordConfirmation string  `json:"new_password_confirmation"`
	RecoveryEmail           string  `json:"recovery_email"`
}

type UpdateRecoveryEmailRequest struct {
	RecoveryEmail   string `json:"recovery_email"`
	CurrentPassword string `json:"current_password"`
}

type RemoveRecoveryEmailRequest struct {
	CurrentPassword string `json:"current_password"`
}

type VerifyRecoveryEmailRequest struct {
	Token string `json:"token"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token                   string `json:"token"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}
