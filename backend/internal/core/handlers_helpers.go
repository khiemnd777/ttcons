package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
)

func (h *Handler) list(c *fiber.Ctx, query string, args ...any) error {
	rows, err := h.service.repo.Query(c.Context(), query, args...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) create(c *fiber.Ctx, table string, specs []ColumnSpec) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.repo.Insert(c.Context(), table, specs, body)
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) update(c *fiber.Ctx, table string, specs []ColumnSpec) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.repo.Update(c.Context(), table, specs, c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) requirePermission(c *fiber.Ctx, permissions []string) error {
	allowed, err := h.service.repo.CanUsePermission(c.Context(), actorID(c), permissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác module này", Record{"permissions": permissions})
}

func (h *Handler) requirePermissionExcluding(c *fiber.Ctx, permissions, excludedPermissions []string) error {
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	excluded, err := h.service.repo.UserHasAnyPermission(c.Context(), actorID(c), excludedPermissions)
	if err != nil {
		return err
	}
	if excluded {
		return forbiddenError("Chức năng này nằm ngoài phạm vi đối soát Kế toán", Record{"excluded_permissions": excludedPermissions})
	}
	return nil
}

func (h *Handler) requireProjectPermission(c *fiber.Ctx, projectID string, permissions []string) error {
	allowed, err := h.service.repo.CanAccessProjectPermission(c.Context(), actorID(c), projectID, permissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác dữ liệu của công trình này", Record{"permissions": permissions})
}

func (h *Handler) requireRecordProjectPermission(c *fiber.Ctx, table, id string, permissions []string) error {
	projectID, err := h.service.repo.RecordProjectID(c.Context(), table, id)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, projectID, permissions)
}

func (h *Handler) requireBodyProjectPermission(c *fiber.Ctx, permissions []string) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, cleanID(body["project_id"]), permissions)
}

func (h *Handler) projectAccessFilter(c *fiber.Ctx, projectAlias string, start int) (string, []any, error) {
	return h.service.repo.ProjectAccessFilter(c.Context(), actorID(c), projectAlias, start)
}

func bodyMap(c *fiber.Ctx) (Record, error) {
	if len(c.Body()) == 0 {
		return Record{}, nil
	}
	var body Record
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return nil, response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
	}
	return body, nil
}

func idListFromBody(body Record, key string) ([]string, error) {
	raw := body[key]
	values := []any{}
	switch typed := raw.(type) {
	case nil:
		return []string{}, nil
	case []any:
		values = typed
	case []string:
		for _, value := range typed {
			values = append(values, value)
		}
	default:
		return nil, validationError("Danh sách ID không hợp lệ", Record{"field": key})
	}
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		id := cleanID(value)
		if id == "" || seen[id] {
			continue
		}
		if !validActivityUUID(id) {
			return nil, validationError("ID không hợp lệ", Record{"field": key, "id": id})
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func actionRequest(c *fiber.Ctx) (ActionRequest, error) {
	req := ActionRequest{}
	if len(c.Body()) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return req, response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thao tác không hợp lệ")
	}
	return req, nil
}

func (h *Handler) requireRunNodePermission(c *fiber.Ctx, nodeID string) error {
	allowed, permissions, err := h.service.repo.RunNodePermissionGate(c.Context(), actorID(c), nodeID)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác node này", Record{"permissions": permissions})
}

func (h *Handler) requireRunStepPermission(c *fiber.Ctx, stepID string) error {
	allowed, permissions, err := h.service.repo.RunStepPermissionGate(c.Context(), actorID(c), stepID)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return forbiddenError("Bạn không có quyền thao tác step này", Record{"permissions": permissions})
}

func actorID(c *fiber.Ctx) string {
	return fmt.Sprint(c.Locals("user_id"))
}

func actorName(c *fiber.Ctx) string {
	return fmt.Sprint(c.Locals("email"))
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
