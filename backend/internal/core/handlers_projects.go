package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	fiber "github.com/gofiber/fiber/v2"
)

func (h *Handler) ListProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListProjects(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectExplorer(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	page, err := h.service.ProjectExplorer(c.Context(), actorID(c), ProjectExplorerRequest{
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		Sort:      c.Query("sort"),
		Direction: c.Query("direction"),
		Page:      c.QueryInt("page", 1),
		PageSize:  c.QueryInt("page_size", projectExplorerDefaultPageSize),
	})
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectProgressBoard(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	board, err := h.service.ProjectProgressBoard(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, board)
}

func (h *Handler) CreateProject(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.CreateProject(c.Context(), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, project)
}

func (h *Handler) ProjectStartupReadiness(c *fiber.Ctx) error {
	readiness, err := h.service.ProjectStartupReadiness(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, readiness)
}

func (h *Handler) MyProjectStartupActions(c *fiber.Ctx) error {
	page, err := h.service.MyProjectStartupActions(c.Context(), actorID(c), c.Query("project_id"), c.Query("search"))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) SetProjectManager(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.SetProjectManager(c.Context(), c.Params("id"), cleanID(body["staff_id"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) GetProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.view"}); err != nil {
		return err
	}
	project, err := h.service.ProjectDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) UpdateProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.UpdateProject(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, project)
}

func (h *Handler) DeleteProject(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.delete"}); err != nil {
		return err
	}
	if err := h.service.DeleteProject(c.Context(), c.Params("id"), actorID(c), actorName(c)); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) PreviewProjectDeletion(c *fiber.Ctx) error {
	var req ProjectDeletionPreviewRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước xóa công trình không hợp lệ")
	}
	result, err := h.service.PreviewProjectDeletion(c.Context(), actorID(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra ảnh hưởng khi xóa công trình")
}

func (h *Handler) ExecuteProjectDeletion(c *fiber.Ctx) error {
	var req ProjectDeletionExecuteRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xác nhận xóa công trình không hợp lệ")
	}
	result, err := h.service.ExecuteProjectDeletion(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã hủy các công trình được chọn")
}

func (h *Handler) ProjectMembers(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"project.view"}); err != nil {
		return err
	}
	page, err := h.service.ProjectMembers(c.Context(), projectID, actorID(c), c.QueryBool("include_inactive", false))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) CreateProjectMember(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"project.member.manage"}); err != nil {
		return err
	}
	req := ProjectMemberMutationRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thành viên không hợp lệ")
	}
	member, err := h.service.SaveProjectMember(c.Context(), projectID, "", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, member)
}

func (h *Handler) CreateProjectMembersBulk(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"project.member.manage"}); err != nil {
		return err
	}
	req := ProjectMemberBulkMutationRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu danh sách thành viên không hợp lệ")
	}
	result, err := h.service.SaveProjectMembersBulk(c.Context(), projectID, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) UpdateProjectMember(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"project.member.manage"}); err != nil {
		return err
	}
	req := ProjectMemberMutationRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thành viên không hợp lệ")
	}
	member, err := h.service.SaveProjectMember(c.Context(), projectID, c.Params("memberId"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, member)
}

func (h *Handler) DeactivateProjectMember(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"project.member.manage"}); err != nil {
		return err
	}
	member, err := h.service.DeactivateProjectMember(c.Context(), projectID, c.Params("memberId"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, member)
}

func (h *Handler) ListPartners(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"catalog.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListPartners(c.Context(), PartnerFilter{
		Query:  c.Query("q"),
		Type:   c.Query("type"),
		Group:  c.Query("group"),
		Status: c.Query("status"),
	})
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePartner(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"catalog.manage"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreatePartner(c.Context(), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdatePartner(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"catalog.manage"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdatePartner(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) PreviewPartnerDeletion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"catalog.manage"}); err != nil {
		return err
	}
	var req PartnerDeletionPreviewRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xem trước xóa đối tác không hợp lệ")
	}
	result, err := h.service.PreviewPartnerDeletion(c.Context(), actorID(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã kiểm tra ảnh hưởng khi xóa đối tác")
}

func (h *Handler) ExecutePartnerDeletion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"catalog.manage"}); err != nil {
		return err
	}
	var req PartnerDeletionExecuteRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xác nhận xóa đối tác không hợp lệ")
	}
	result, err := h.service.ExecutePartnerDeletion(c.Context(), actorID(c), actorName(c), req)
	if err != nil {
		return err
	}
	return response.OK(c, result, "Đã vô hiệu hóa các đối tác được chọn")
}
