package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"errors"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"net/url"
)

func (h *Handler) ListStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListStaff(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.create"}); err != nil {
		return err
	}
	if err := h.requirePermission(c, []string{"staff.account_security.manage"}); err != nil {
		return err
	}
	var req CreateStaffRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nhân sự không hợp lệ")
	}
	row, err := h.service.CreateStaff(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) PreviewStaffAccountImport(c *fiber.Ctx) error {
	if err := h.requireStaffAccountImportPermissions(c); err != nil {
		return err
	}
	var req StaffAccountImportPreviewRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước import không hợp lệ")
	}
	result, err := h.service.PreviewStaffAccountImport(c.Context(), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra danh sách import")
}

func (h *Handler) ExecuteStaffAccountImport(c *fiber.Ctx) error {
	if err := h.requireStaffAccountImportPermissions(c); err != nil {
		return err
	}
	var req StaffAccountImportExecuteRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thực thi import không hợp lệ")
	}
	result, err := h.service.ExecuteStaffAccountImport(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) requireStaffAccountImportPermissions(c *fiber.Ctx) error {
	for _, permission := range []string{"staff.import", "staff.create", "staff.account_security.manage", "permission.assign"} {
		if err := h.requirePermission(c, []string{permission}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) PreviewStaffDeletion(c *fiber.Ctx) error {
	if err := h.requireStaffDeletionPermissions(c); err != nil {
		return err
	}
	var req StaffDeletionPreviewRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước xóa nhân sự không hợp lệ")
	}
	result, err := h.service.PreviewStaffDeletion(c.Context(), actorID(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra ảnh hưởng khi xóa nhân sự")
}

func (h *Handler) ExecuteStaffDeletion(c *fiber.Ctx) error {
	if err := h.requireStaffDeletionPermissions(c); err != nil {
		return err
	}
	var req StaffDeletionExecuteRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xác nhận xóa nhân sự không hợp lệ")
	}
	result, err := h.service.ExecuteStaffDeletion(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã xóa nhân sự và vô hiệu hóa tài khoản liên quan")
}

func (h *Handler) requireStaffDeletionPermissions(c *fiber.Ctx) error {
	for _, permission := range []string{"staff.delete", "staff.account_security.manage"} {
		if err := h.requirePermission(c, []string{permission}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) UpdateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.update"}); err != nil {
		return err
	}
	var req UpdateStaffRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nhân sự không hợp lệ")
	}
	if req.hasAccountSecurityChange() {
		if err := h.requirePermission(c, []string{"staff.account_security.manage"}); err != nil {
			return err
		}
	}
	row, err := h.service.UpdateStaff(c.Context(), c.Params("id"), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, row, "Đã cập nhật nhân sự")
}

func (h *Handler) ListRoles(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.view"}); err != nil {
		return err
	}
	return h.list(c, `
		SELECT r.*,
			COALESCE(jsonb_agg(rp.permission_id::text ORDER BY p.module, p.key) FILTER (WHERE rp.permission_id IS NOT NULL), '[]'::jsonb) AS permission_ids,
			COUNT(rp.permission_id) AS permission_count
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id=r.id
		LEFT JOIN permissions p ON p.id=rp.permission_id
		GROUP BY r.id
		ORDER BY r.display_name
	`)
}

func (h *Handler) CreateRole(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.create"}); err != nil {
		return err
	}
	return h.create(c, "roles", roleColumns())
}

func (h *Handler) UpdateRole(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"role.update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateRole(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM permissions ORDER BY module, key`)
}

func (h *Handler) AssignRolePermissions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.assign"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	permissionIDs, err := idListFromBody(body, "permission_ids")
	if err != nil {
		return err
	}
	result, err := h.service.repo.ReplaceRolePermissions(c.Context(), c.Params("id"), permissionIDs)
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) AssignUserRoles(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"permission.assign"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	roleIDs, err := idListFromBody(body, "role_ids")
	if err != nil {
		return err
	}
	result, err := h.service.repo.ReplaceUserRoles(c.Context(), c.Params("id"), roleIDs, stringValue(body["project_id"]))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) AuditLogs(c *fiber.Ctx) error {
	if err := h.requirePermissionExcluding(c, []string{"audit.view"}, []string{"project.reconciliation.view"}); err != nil {
		return err
	}
	page, err := h.service.AuditLogs(c.Context(), "", actorID(c), auditLogFilterFromQuery(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectAuditLogs(c *fiber.Ctx) error {
	if err := h.requirePermissionExcluding(c, []string{"audit.view"}, []string{"project.reconciliation.view"}); err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"audit.view"}); err != nil {
		return err
	}
	page, err := h.service.AuditLogs(c.Context(), c.Params("id"), actorID(c), auditLogFilterFromQuery(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func auditLogFilterFromQuery(c *fiber.Ctx) AuditLogFilter {
	return AuditLogFilter{
		ProjectID:  c.Query("project_id"),
		EventType:  c.Query("event_type"),
		TargetType: c.Query("target_type"),
		ActorName:  c.Query("actor_name"),
		DateFrom:   c.Query("date_from"),
		DateTo:     c.Query("date_to"),
		Search:     c.Query("search"),
		Page:       c.QueryInt("page", 1),
		PageSize:   c.QueryInt("page_size", 25),
	}
}

func (h *Handler) Settings(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.view"}); err != nil {
		return err
	}
	canUpdate, err := h.service.repo.CanUsePermission(c.Context(), actorID(c), []string{"settings.update"})
	if err != nil {
		return err
	}
	if h.service.repo.systemSettings == nil {
		return errors.New("system settings store is not configured")
	}
	result, err := h.service.repo.systemSettings.List(c.Context(), canUpdate)
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) UpdateSettings(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.update"}); err != nil {
		return err
	}
	var request struct {
		Updates []SystemSettingUpdateRequest `json:"updates"`
	}
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu cấu hình không hợp lệ")
	}
	if h.service.repo.systemSettings == nil {
		return errors.New("system settings store is not configured")
	}
	result, err := h.service.repo.systemSettings.Update(c.Context(), request.Updates, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã lưu thiết lập hệ thống")
}

func (h *Handler) DownloadSettingsBackup(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.view"}); err != nil {
		return err
	}
	if h.service.repo.systemSettings == nil {
		return errors.New("system settings store is not configured")
	}
	raw, filename, metadata, err := h.service.repo.systemSettings.ExportBackup(c.Context())
	if err != nil {
		return err
	}
	if err := h.service.repo.AddAudit(c.Context(), "", actorID(c), actorName(c), "system_settings_backup_exported", "system_settings_backup", "", "Đã tải bản sao thiết lập hệ thống", metadata); err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "application/json; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", filename, url.PathEscape(filename)))
	return c.Send(raw)
}

func (h *Handler) PreviewSettingsBackupRestore(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.update"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp JSON bản sao cấu hình là bắt buộc")
	}
	raw, err := readSystemSettingsBackupUpload(fileHeader)
	if err != nil {
		return err
	}
	if h.service.repo.systemSettings == nil {
		return errors.New("system settings store is not configured")
	}
	result, err := h.service.repo.systemSettings.PreviewBackupRestore(c.Context(), raw)
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) RestoreSettingsBackup(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.update"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp JSON bản sao cấu hình là bắt buộc")
	}
	raw, err := readSystemSettingsBackupUpload(fileHeader)
	if err != nil {
		return err
	}
	expectedVersions := map[string]int64{}
	if err := json.Unmarshal([]byte(c.FormValue("expected_versions")), &expectedVersions); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Danh sách phiên bản thiết lập không hợp lệ")
	}
	if h.service.repo.systemSettings == nil {
		return errors.New("system settings store is not configured")
	}
	result, err := h.service.repo.systemSettings.RestoreBackup(
		c.Context(), raw, c.FormValue("preview_hash"), expectedVersions, actorID(c), actorName(c),
	)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã khôi phục thiết lập hệ thống")
}
