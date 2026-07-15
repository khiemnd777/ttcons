package core

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

const refreshCookieName = "consflow_refresh_token"

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return response.OK(c, Record{"status": "ok", "service": "consflow"})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đăng nhập không hợp lệ")
	}
	data, err := h.service.Login(c.Context(), req, c.Get("User-Agent"), c.IP())
	if err != nil {
		return response.Fail(c, fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Email hoặc mật khẩu không đúng")
	}
	h.setRefreshCookie(c, fmt.Sprint(data["refresh_token"]), fmt.Sprint(data["refresh_expires_at"]))
	delete(data, "refresh_token")
	delete(data, "refresh_expires_at")
	return response.OK(c, data, "Đăng nhập thành công")
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	data, err := h.service.RefreshSession(c.Context(), c.Cookies(refreshCookieName), c.Get("User-Agent"), c.IP())
	if err != nil {
		h.clearRefreshCookie(c)
		return response.Fail(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại")
	}
	h.setRefreshCookie(c, fmt.Sprint(data["refresh_token"]), fmt.Sprint(data["refresh_expires_at"]))
	delete(data, "refresh_token")
	delete(data, "refresh_expires_at")
	return response.OK(c, data)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	if err := h.service.LogoutByRefreshToken(c.Context(), c.Cookies(refreshCookieName)); err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	return response.OK(c, Record{"logged_out": true})
}

func (h *Handler) Me(c *fiber.Ctx) error {
	user, err := h.service.Me(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, user)
}

func (h *Handler) setRefreshCookie(c *fiber.Ctx, token string, expiresAtText string) {
	expiresAt, err := time.Parse(time.RFC3339, expiresAtText)
	if err != nil {
		expiresAt = time.Now().Add(h.service.refreshTokenTTL)
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Expires:  expiresAt,
		HTTPOnly: true,
		Secure:   h.service.refreshCookieSecure,
		SameSite: h.service.refreshCookieSameSite,
		Path:     "/api/auth",
	})
}

func (h *Handler) clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   h.service.refreshCookieSecure,
		SameSite: h.service.refreshCookieSameSite,
		Path:     "/api/auth",
	})
}

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

func (h *Handler) CreateProject(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	project, err := h.service.CreateProject(c.Context(), body)
	if err != nil {
		return err
	}
	return response.Created(c, project)
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
	if err := h.service.DeleteProject(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
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

func (h *Handler) ListTemplates(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	rows, err := h.service.ListTemplates(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	template, err := h.service.CreateTemplate(c.Context(), body, actorID(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) GetTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	template, err := h.service.TemplateDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) UpdateTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	template, err := h.service.UpdateTemplate(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) DeleteTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.DeleteTemplate(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) PublishTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.publish"}); err != nil {
		return err
	}
	template, err := h.service.PublishTemplate(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) CloneTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.create"}); err != nil {
		return err
	}
	template, err := h.service.CloneTemplate(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) AddTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	node, err := h.service.CreateTemplateNode(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.Created(c, node)
}

func (h *Handler) UpdateTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	node, err := h.service.UpdateTemplateNode(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) DeleteTemplateNode(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.DeleteTemplateNode(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["node_id"] = c.Params("id")
	step, err := h.service.CreateTemplateStep(c.Context(), body)
	if err != nil {
		return err
	}
	return response.Created(c, step)
}

func (h *Handler) UpdateTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	step, err := h.service.UpdateTemplateStep(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, step)
}

func (h *Handler) DeleteTemplateStep(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.DeleteTemplateStep(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) AddTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	if err := prepareTemplateEdgeBody(body, false); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	}
	edge, err := h.service.CreateTemplateEdge(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.Created(c, edge)
}

func (h *Handler) PipelineEdgeConditions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	return response.OK(c, PipelineEdgeConditionCatalog())
}

func (h *Handler) WorkflowFormCatalog(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	rows, err := h.service.WorkflowFormCatalog(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) WorkflowFormTemplateLibrary(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	rows, err := h.service.WorkflowFormTemplateLibrary(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetWorkflowFormTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	template, err := h.service.WorkflowFormTemplateDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) CreateWorkflowFormTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	req := WorkflowFormTemplateRequest{}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form template không hợp lệ")
	}
	template, err := h.service.CreateWorkflowFormTemplate(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, template)
}

func (h *Handler) UpdateWorkflowFormTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	req := WorkflowFormTemplateRequest{}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form template không hợp lệ")
	}
	template, err := h.service.UpdateWorkflowFormTemplate(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) ArchiveWorkflowFormTemplate(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	template, err := h.service.ArchiveWorkflowFormTemplate(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, template)
}

func (h *Handler) CreateWorkflowFormVersion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	req := WorkflowFormVersionRequest{}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form version không hợp lệ")
	}
	version, err := h.service.CreateWorkflowFormVersion(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, version)
}

func (h *Handler) UpdateWorkflowFormVersion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	req := WorkflowFormVersionRequest{}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form version không hợp lệ")
	}
	version, err := h.service.UpdateWorkflowFormVersion(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, version)
}

func (h *Handler) PublishWorkflowFormVersion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	version, err := h.service.PublishWorkflowFormVersion(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, version)
}

func (h *Handler) ArchiveWorkflowFormVersion(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	version, err := h.service.ArchiveWorkflowFormVersion(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, version)
}

func (h *Handler) UpdateTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	if err := prepareTemplateEdgeBody(body, true); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	}
	edge, err := h.service.UpdateTemplateEdge(c.Context(), c.Params("id"), body)
	if err != nil {
		return err
	}
	return response.OK(c, edge)
}

func (h *Handler) DeleteTemplateEdge(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	if err := h.service.DeleteTemplateEdge(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return response.OK(c, Record{"deleted": true})
}

func (h *Handler) StartPipeline(c *fiber.Ctx) error {
	body, _ := bodyMap(c)
	run, err := h.service.StartPipeline(c.Context(), c.Params("id"), stringValue(body["template_id"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, run)
}

func (h *Handler) GetPipelineRun(c *fiber.Ctx) error {
	run, err := h.service.PipelineRun(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) PlanningDailyReports(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"planning.view", "pipeline.view"}); err != nil {
		return err
	}
	filter := PlanningDailyReportFilter{
		ProjectID:  c.Query("project_id"),
		DateFrom:   c.Query("date_from"),
		DateTo:     c.Query("date_to"),
		ReportedBy: c.Query("reported_by"),
		Owner:      c.Query("owner"),
		Status:     c.Query("status"),
		Search:     c.Query("search"),
	}
	rows, err := h.service.PlanningDailyReports(c.Context(), filter, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) PlanningMeetingMinutes(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"planning.view", "pipeline.view"}); err != nil {
		return err
	}
	filter := PlanningMeetingMinutesFilter{
		ProjectID: c.Query("project_id"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		Chair:     c.Query("chair"),
		Owner:     c.Query("owner"),
		Status:    c.Query("status"),
		Search:    c.Query("search"),
	}
	rows, err := h.service.PlanningMeetingMinutes(c.Context(), filter, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) PlanningChecklists(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"planning.view", "pipeline.view"}); err != nil {
		return err
	}
	filter := PlanningChecklistFilter{
		ProjectID:  c.Query("project_id"),
		DateFrom:   c.Query("date_from"),
		DateTo:     c.Query("date_to"),
		CheckedBy:  c.Query("checked_by"),
		Assignee:   c.Query("assignee"),
		ItemStatus: c.Query("item_status"),
		Evidence:   c.Query("evidence"),
		Status:     c.Query("status"),
		Search:     c.Query("search"),
	}
	rows, err := h.service.PlanningChecklists(c.Context(), filter, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectWorkItems(c *fiber.Ctx) error {
	items, err := h.service.ProjectWorkItems(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyWorkItems(c *fiber.Ctx) error {
	items, err := h.service.MyWorkItems(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) MyActivityAttention(c *fiber.Ctx) error {
	items, err := h.service.MyActivityAttention(c.Context(), ActivityAttentionFilter{
		Queue:     c.Query("queue"),
		ProjectID: c.Query("project_id"),
		Limit:     c.QueryInt("limit", 80),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, items)
}

func (h *Handler) ProjectOperations(c *fiber.Ctx) error {
	operations, err := h.service.ProjectOperations(c.Context(), c.Params("id"), actorID(c), ProjectOperationsFilter{
		View: c.Query("view"),
	})
	if err != nil {
		return err
	}
	return response.OK(c, operations)
}

func (h *Handler) ProjectEvidence(c *fiber.Ctx) error {
	page, err := h.service.ProjectEvidencePage(c.Context(), c.Params("id"), ProjectEvidenceFilter{
		EvidenceType: c.Query("evidence_type"),
		Status:       c.Query("status"),
		WorkItemID:   c.Query("work_item_id"),
		SubmittedBy:  c.Query("submitted_by"),
		Visibility:   c.Query("visibility"),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
		Search:       c.Query("search"),
		Cursor:       c.Query("cursor"),
		Limit:        c.QueryInt("limit", 25),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectActivity(c *fiber.Ctx) error {
	page, err := h.service.ProjectActivityPage(c.Context(), c.Params("id"), ProjectActivityFilter{
		ScopeType:    c.Query("scope_type"),
		ScopeID:      c.Query("scope_id"),
		ActivityType: c.Query("activity_type"),
		ActorID:      c.Query("actor_id"),
		Visibility:   c.Query("visibility"),
		Kind:         c.Query("kind"),
		Attention:    c.Query("attention"),
		Search:       c.Query("search"),
		Cursor:       c.Query("cursor"),
		Limit:        c.QueryInt("limit", 30),
	}, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func (h *Handler) ProjectActivityOptions(c *fiber.Ctx) error {
	options, err := h.service.ProjectActivityOptions(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, options)
}

func (h *Handler) AddProjectActivity(c *fiber.Ctx) error {
	req := WorkItemActivityRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hoạt động không hợp lệ")
		}
	}
	activity, err := h.service.AddProjectActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, activity)
}

func (h *Handler) MarkProjectActivityRead(c *fiber.Ctx) error {
	result, err := h.service.MarkProjectActivityRead(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) GetWorkItem(c *fiber.Ctx) error {
	item, err := h.service.WorkItemDetail(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) GetWorkItemForm(c *fiber.Ctx) error {
	form, err := h.service.WorkItemForm(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) StartWorkItem(c *fiber.Ctx) error    { return h.workItemAction(c, "start") }
func (h *Handler) CompleteWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "complete") }
func (h *Handler) BlockWorkItem(c *fiber.Ctx) error    { return h.workItemAction(c, "block") }
func (h *Handler) ResolveWorkItemBlocker(c *fiber.Ctx) error {
	return h.workItemAction(c, "resolve-blocker")
}
func (h *Handler) RequestWorkItemReview(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-review")
}
func (h *Handler) RequestWorkItemApproval(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-approval")
}
func (h *Handler) ApproveWorkItem(c *fiber.Ctx) error { return h.workItemAction(c, "approve") }
func (h *Handler) RejectWorkItem(c *fiber.Ctx) error  { return h.workItemAction(c, "reject") }
func (h *Handler) RequestWorkItemChange(c *fiber.Ctx) error {
	return h.workItemAction(c, "request-change")
}

func (h *Handler) workItemAction(c *fiber.Ctx, action string) error {
	req := WorkItemActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
		}
	}
	item, err := h.service.WorkItemAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) AddWorkItemEvidence(c *fiber.Ctx) error {
	req := WorkItemEvidenceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu minh chứng không hợp lệ")
		}
	}
	evidence, err := h.service.AddWorkItemEvidence(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, evidence)
}

func (h *Handler) UploadWorkItemEvidence(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp minh chứng là bắt buộc")
	}
	req := WorkItemEvidenceRequest{
		EvidenceType: c.FormValue("evidence_type"),
		DocumentType: c.FormValue("document_type"),
		Name:         c.FormValue("name"),
		Title:        c.FormValue("title"),
		Description:  c.FormValue("description"),
		Caption:      c.FormValue("caption"),
		RevisionNo:   c.FormValue("revision_no"),
		Visibility:   c.FormValue("visibility"),
	}
	if rawPayload := c.FormValue("payload"); rawPayload != "" {
		payload := Record{}
		if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Nội dung minh chứng không hợp lệ")
		}
		req.Payload = payload
	}
	evidence, err := h.service.AddWorkItemEvidenceUpload(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, evidence)
}

func (h *Handler) AddWorkItemActivity(c *fiber.Ctx) error {
	req := WorkItemActivityRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hoạt động không hợp lệ")
		}
	}
	activity, err := h.service.AddWorkItemActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, activity)
}

func (h *Handler) GetSubcontractorReconciliation(c *fiber.Ctx) error {
	reconciliation, err := h.service.SubcontractorReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SaveSubcontractorReconciliation(c *fiber.Ctx) error {
	req := SubcontractorReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát thầu phụ không hợp lệ")
		}
	}
	reconciliation, err := h.service.SaveSubcontractorReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SubmitSubcontractorReconciliation(c *fiber.Ctx) error {
	req := SubcontractorReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát thầu phụ không hợp lệ")
		}
	}
	reconciliation, err := h.service.SubmitSubcontractorReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) GetOwnerClientReconciliation(c *fiber.Ctx) error {
	reconciliation, err := h.service.OwnerClientReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SaveOwnerClientReconciliation(c *fiber.Ctx) error {
	req := OwnerClientReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát chủ đầu tư không hợp lệ")
		}
	}
	reconciliation, err := h.service.SaveOwnerClientReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) SubmitOwnerClientReconciliation(c *fiber.Ctx) error {
	req := OwnerClientReconciliationRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đối soát chủ đầu tư không hợp lệ")
		}
	}
	reconciliation, err := h.service.SubmitOwnerClientReconciliation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, reconciliation)
}

func (h *Handler) GetMaterialSnapshot(c *fiber.Ctx) error {
	snapshot, err := h.service.MaterialSnapshot(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) SaveMaterialSnapshot(c *fiber.Ctx) error {
	req := MaterialSnapshotRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ghi nhận vật tư không hợp lệ")
		}
	}
	snapshot, err := h.service.SaveMaterialSnapshot(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) SubmitMaterialSnapshot(c *fiber.Ctx) error {
	req := MaterialSnapshotRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ghi nhận vật tư không hợp lệ")
		}
	}
	snapshot, err := h.service.SubmitMaterialSnapshot(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, snapshot)
}

func (h *Handler) GetSurplusHandlings(c *fiber.Ctx) error {
	handlings, err := h.service.SurplusHandlings(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, handlings)
}

func (h *Handler) CreateSurplusHandling(c *fiber.Ctx) error {
	req := SurplusHandlingRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý vật tư dư không hợp lệ")
		}
	}
	handling, err := h.service.CreateSurplusHandling(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, handling)
}

func (h *Handler) UpdateSurplusHandling(c *fiber.Ctx) error {
	req := SurplusHandlingRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý vật tư dư không hợp lệ")
		}
	}
	handling, err := h.service.UpdateSurplusHandling(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, handling)
}

func (h *Handler) SubmitSurplusHandling(c *fiber.Ctx) error {
	handling, err := h.service.SubmitSurplusHandling(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, handling)
}

func (h *Handler) WorkItemIncomingAcceptances(c *fiber.Ctx) error {
	rows, err := h.service.WorkItemIncomingAcceptances(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) WorkItemOutgoingAcceptance(c *fiber.Ctx) error {
	rows, err := h.service.WorkItemOutgoingAcceptance(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) SaveWorkflowFormDraft(c *fiber.Ctx) error {
	req := WorkflowFormSaveRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form không hợp lệ")
		}
	}
	form, err := h.service.SaveWorkflowFormDraft(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) SubmitWorkflowForm(c *fiber.Ctx) error {
	req := WorkflowFormSaveRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu workflow form không hợp lệ")
		}
	}
	form, err := h.service.SubmitWorkflowForm(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, form)
}

func (h *Handler) SignWorkflowForm(c *fiber.Ctx) error {
	req := WorkflowFormSignRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chữ ký workflow form không hợp lệ")
		}
	}
	form, err := h.service.SignWorkflowForm(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, form)
}

func (h *Handler) GenerateWorkflowFormPDFExport(c *fiber.Ctx) error {
	form, err := h.service.GenerateWorkflowFormPDFExport(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, form)
}

func (h *Handler) MarkActivityRead(c *fiber.Ctx) error {
	item, err := h.service.MarkActivityRead(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) ResolveActivity(c *fiber.Ctx) error {
	req := ActivityResolutionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý hoạt động không hợp lệ")
		}
	}
	item, err := h.service.ResolveActivity(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) SetActivityReaction(c *fiber.Ctx) error {
	req := ActivityReactionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu phản ứng không hợp lệ")
	}
	item, err := h.service.SetActivityReaction(c.Context(), c.Params("id"), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) UpdateActivityMessage(c *fiber.Ctx) error {
	req := ActivityMessageUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chỉnh sửa tin nhắn không hợp lệ")
	}
	item, err := h.service.UpdateActivityMessage(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) WithdrawActivityMessage(c *fiber.Ctx) error {
	item, err := h.service.WithdrawActivityMessage(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) GetEvidenceFile(c *fiber.Ctx) error {
	file, err := h.service.EvidenceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", "minh-chung", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(file.Path)
}

func (h *Handler) GetWorkflowFormSignatureVisual(c *fiber.Ctx) error {
	file, err := h.service.WorkflowFormSignatureVisual(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", "signature.png", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(file.Path)
}

func (h *Handler) GetWorkflowFormPDFExportFile(c *fiber.Ctx) error {
	file, err := h.service.WorkflowFormPDFExportFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", "workflow-form.pdf", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(file.Path)
}

func (h *Handler) AcceptEvidence(c *fiber.Ctx) error {
	return h.evidenceAction(c, "accept")
}

func (h *Handler) RejectEvidence(c *fiber.Ctx) error {
	return h.evidenceAction(c, "reject")
}

func (h *Handler) evidenceAction(c *fiber.Ctx, action string) error {
	req := ActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thao tác minh chứng không hợp lệ")
		}
	}
	evidence, err := h.service.EvidenceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, evidence)
}

func (h *Handler) StartRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	node, err := h.service.repo.QueryOne(c.Context(), `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1 RETURNING *`, c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) SubmitRunStep(c *fiber.Ctx) error {
	if err := h.requireRunStepPermission(c, c.Params("id")); err != nil {
		return err
	}
	req := WorkItemActionRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ")
		}
	}
	step, err := h.service.SubmitRunStep(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, step)
}

func (h *Handler) RequestNodeApproval(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	req, err := h.service.RequestNodeApproval(c.Context(), c.Params("id"), actorID(c), actorName(c), stringValue(body["reason"]))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) AdvanceRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	run, err := h.service.AdvanceRunNode(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, run)
}

func (h *Handler) BlockRunNode(c *fiber.Ctx) error {
	if err := h.requireRunNodePermission(c, c.Params("id")); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	node, err := h.service.BlockRunNode(c.Context(), c.Params("id"), stringValue(body["reason"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, node)
}

func (h *Handler) ListApprovals(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view", "operation.view", "procurement.view", "accounting.view", "report.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT ar.*, p.name AS project_name, u.full_name AS requested_by_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		LEFT JOIN users u ON u.id=ar.requested_by
		WHERE (p.id IS NULL OR p.status <> 'cancelled')
		  AND (p.id IS NOT NULL AND %s)
		ORDER BY ar.created_at DESC
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) PendingApprovals(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view", "operation.view", "procurement.view", "accounting.view", "report.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT ar.*, p.name AS project_name
		FROM approval_requests ar
		LEFT JOIN projects p ON p.id=ar.project_id
		WHERE ar.status='pending' AND (p.id IS NULL OR p.status <> 'cancelled')
		  AND (p.id IS NOT NULL AND %s)
		ORDER BY ar.created_at DESC
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) Approve(c *fiber.Ctx) error       { return h.approvalAction(c, "approve") }
func (h *Handler) Reject(c *fiber.Ctx) error        { return h.approvalAction(c, "reject") }
func (h *Handler) RequestChange(c *fiber.Ctx) error { return h.approvalAction(c, "request-change") }

func (h *Handler) approvalAction(c *fiber.Ctx, action string) error {
	body, _ := bodyMap(c)
	req, err := h.service.ApprovalAction(c.Context(), c.Params("id"), action, stringValue(body["comment"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ProjectBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectBOQ(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	body["project_id"] = c.Params("id")
	body["created_by"] = actorID(c)
	item, err := h.service.CreateBOQ(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, item)
}

func (h *Handler) UpdateBOQ(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	item, err := h.service.UpdateBOQ(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) UpdateBOQExpectedUnitPrice(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	var req BOQUnitPriceRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đơn giá không hợp lệ")
	}
	item, err := h.service.UpdateBOQExpectedUnitPrice(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) LockBOQNormUnitPrice(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.approve"}); err != nil {
		return err
	}
	var req BOQUnitPriceRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đơn giá không hợp lệ")
	}
	item, err := h.service.LockBOQNormUnitPrice(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) LockBOQFinalUnitPrice(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.approve"}); err != nil {
		return err
	}
	var req BOQUnitPriceRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đơn giá không hợp lệ")
	}
	item, err := h.service.LockBOQFinalUnitPrice(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, item)
}

func (h *Handler) SubmitBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	result, err := h.service.BOQProjectAction(c.Context(), c.Params("id"), "submit", actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ApproveBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.approve"}); err != nil {
		return err
	}
	result, err := h.service.BOQProjectAction(c.Context(), c.Params("id"), "approve", actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) GenerateProcurementFromBOQ(c *fiber.Ctx) error {
	req, err := h.service.GenerateProcurementFromBOQ(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) ProjectSupplierQuotations(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"procurement.view", "boq.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectSupplierQuotations(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateSupplierQuotation(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"procurement.create"}); err != nil {
		return err
	}
	var req SupplierQuotationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo giá không hợp lệ")
	}
	quotation, err := h.service.SaveSupplierQuotation(c.Context(), c.Params("id"), "", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, quotation)
}

func (h *Handler) GetSupplierQuotation(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "supplier_quotations", c.Params("id"), []string{"procurement.view", "boq.view"}); err != nil {
		return err
	}
	quotation, err := h.service.SupplierQuotationDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, quotation)
}

func (h *Handler) UpdateSupplierQuotation(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "supplier_quotations", c.Params("id"), []string{"procurement.create"}); err != nil {
		return err
	}
	current, err := h.service.SupplierQuotationDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	var req SupplierQuotationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo giá không hợp lệ")
	}
	quotation, err := h.service.SaveSupplierQuotation(c.Context(), cleanID(current["project_id"]), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, quotation)
}

func (h *Handler) SubmitSupplierQuotation(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "supplier_quotations", c.Params("id"), []string{"procurement.create"}); err != nil {
		return err
	}
	quotation, err := h.service.SubmitSupplierQuotation(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, quotation)
}

func (h *Handler) QualifySupplierQuotation(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "supplier_quotations", c.Params("id"), []string{"procurement.approve", "boq.approve"}); err != nil {
		return err
	}
	var req SupplierQuotationQualificationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu đánh giá báo giá không hợp lệ")
	}
	quotation, err := h.service.QualifySupplierQuotation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, quotation)
}

func (h *Handler) SelectSupplierQuote(c *fiber.Ctx) error {
	row, err := h.service.repo.QueryOne(c.Context(), `
		SELECT q.project_id
		FROM supplier_quotation_items qi
		JOIN supplier_quotations q ON q.id=qi.quotation_id
		WHERE qi.id=$1 AND q.status <> 'cancelled'
	`, c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(row["project_id"]), []string{"procurement.approve", "boq.approve"}); err != nil {
		return err
	}
	var req SupplierQuoteSelectionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chọn báo giá không hợp lệ")
	}
	boq, err := h.service.SelectSupplierQuote(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, boq)
}

func (h *Handler) SupplierQuoteComparison(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"procurement.view", "boq.view"}); err != nil {
		return err
	}
	rows, err := h.service.SupplierQuoteComparison(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectContracts(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"contract.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectContracts(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateContract(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"contract.create"}); err != nil {
		return err
	}
	var req ContractRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hợp đồng không hợp lệ")
	}
	row, err := h.service.SaveContract(c.Context(), c.Params("id"), "", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) GetContract(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.view"}); err != nil {
		return err
	}
	row, err := h.service.ContractDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) UpdateContract(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.create"}); err != nil {
		return err
	}
	current, err := h.service.ContractDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	var req ContractRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu hợp đồng không hợp lệ")
	}
	row, err := h.service.SaveContract(c.Context(), cleanID(current["project_id"]), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateContractVersion(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.create"}); err != nil {
		return err
	}
	var req ContractVersionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu version hợp đồng không hợp lệ")
	}
	row, err := h.service.CreateContractVersion(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitContractVersion(c *fiber.Ctx) error {
	return h.contractVersionAction(c, "submit", []string{"contract.review"})
}
func (h *Handler) ApproveContractVersion(c *fiber.Ctx) error {
	return h.contractVersionAction(c, "approve", []string{"contract.approve"})
}
func (h *Handler) SignContractVersion(c *fiber.Ctx) error {
	return h.contractVersionAction(c, "sign", []string{"contract.sign"})
}

func (h *Handler) contractVersionAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireContractChildPermission(c, "version", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ContractVersionAction(c.Context(), c.Params("id"), action, actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ActivateContract(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.sign"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ActivateContract(c.Context(), c.Params("id"), actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateContractMilestone(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.milestone_manage"}); err != nil {
		return err
	}
	var req ContractMilestoneRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu milestone không hợp lệ")
	}
	row, err := h.service.CreateContractMilestone(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) EligibleContractMilestone(c *fiber.Ctx) error {
	return h.contractMilestoneAction(c, "eligible")
}

func (h *Handler) OpenContractMilestoneObligation(c *fiber.Ctx) error {
	return h.contractMilestoneAction(c, "open-obligation")
}

func (h *Handler) contractMilestoneAction(c *fiber.Ctx, action string) error {
	if err := h.requireContractChildPermission(c, "milestone", c.Params("id"), []string{"contract.milestone_manage"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ContractMilestoneAction(c.Context(), c.Params("id"), action, actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateContractVariation(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.create"}); err != nil {
		return err
	}
	var req ContractVariationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu variation không hợp lệ")
	}
	row, err := h.service.CreateContractVariation(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "submit", []string{"contract.review"})
}
func (h *Handler) ApproveContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "approve", []string{"contract.approve"})
}
func (h *Handler) RejectContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "reject", []string{"contract.approve"})
}
func (h *Handler) IncorporateContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "incorporate", []string{"contract.create"})
}

func (h *Handler) contractVariationAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireContractChildPermission(c, "variation", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ContractVariationAction(c.Context(), c.Params("id"), action, actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ReleaseContractRetention(c *fiber.Ctx) error {
	if err := h.requireContractChildPermission(c, "retention", c.Params("id"), []string{"contract.retention_release"}); err != nil {
		return err
	}
	var req ContractRetentionReleaseRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu giải phóng giữ lại không hợp lệ")
	}
	row, err := h.service.ReleaseContractRetention(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) requireContractChildPermission(c *fiber.Ctx, kind, id string, permissions []string) error {
	queries := map[string]string{
		"version":   `SELECT c.project_id FROM contract_versions child JOIN contracts c ON c.id=child.contract_id WHERE child.id=$1`,
		"milestone": `SELECT c.project_id FROM contract_milestones child JOIN contracts c ON c.id=child.contract_id WHERE child.id=$1`,
		"variation": `SELECT c.project_id FROM contract_variations child JOIN contracts c ON c.id=child.contract_id WHERE child.id=$1`,
		"retention": `SELECT c.project_id FROM contract_retentions child JOIN contracts c ON c.id=child.contract_id WHERE child.id=$1`,
	}
	query := queries[kind]
	if query == "" {
		return validationError("Loại hồ sơ hợp đồng không hợp lệ")
	}
	row, err := h.service.repo.QueryOne(c.Context(), query, id)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, cleanID(row["project_id"]), permissions)
}

func (h *Handler) ProjectQualityWorkspace(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.view"}); err != nil {
		return err
	}
	row, err := h.service.ProjectQualityWorkspace(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateQualityITP(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	var req QualityITPRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ITP không hợp lệ")
	}
	row, err := h.service.CreateQualityITP(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ReviseQualityITP(c *fiber.Ctx) error {
	if err := h.requireQualityChildPermission(c, "itp", c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReviseQualityITP(c.Context(), c.Params("id"), actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "submit", []string{"quality.review"})
}
func (h *Handler) ApproveQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "approve", []string{"quality.approve"})
}
func (h *Handler) ActivateQualityITP(c *fiber.Ctx) error {
	return h.qualityITPAction(c, "activate", []string{"quality.approve"})
}

func (h *Handler) qualityITPAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireQualityChildPermission(c, "itp", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.QualityITPAction(c.Context(), c.Params("id"), action, actionComment(req), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateQualityInspectionRequest(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.create"}); err != nil {
		return err
	}
	var req QualityInspectionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu RFI không hợp lệ")
	}
	row, err := h.service.CreateQualityInspectionRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityInspectionRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "submit", []string{"quality.create"})
}
func (h *Handler) ScheduleQualityInspectionRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "schedule", []string{"quality.inspect"})
}
func (h *Handler) InspectQualityRequest(c *fiber.Ctx) error {
	return h.qualityInspectionAction(c, "inspect", []string{"quality.inspect"})
}

func (h *Handler) qualityInspectionAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireQualityChildPermission(c, "rfi", c.Params("id"), permissions); err != nil {
		return err
	}
	var req QualityInspectionDecisionRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu inspection không hợp lệ")
		}
	}
	row, err := h.service.QualityInspectionAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateQualityNCR(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.ncr_manage"}); err != nil {
		return err
	}
	var req QualityNCRRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu NCR không hợp lệ")
	}
	row, err := h.service.CreateQualityNCR(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) SubmitQualityNCRRootCause(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "root-cause")
}
func (h *Handler) SubmitQualityNCRCorrective(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "corrective")
}
func (h *Handler) LinkQualityNCRReinspection(c *fiber.Ctx) error {
	return h.qualityNCRAction(c, "reinspection")
}
func (h *Handler) CloseQualityNCR(c *fiber.Ctx) error { return h.qualityNCRAction(c, "close") }

func (h *Handler) qualityNCRAction(c *fiber.Ctx, action string) error {
	if err := h.requireQualityChildPermission(c, "ncr", c.Params("id"), []string{"quality.ncr_manage"}); err != nil {
		return err
	}
	var req QualityNCRActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu xử lý NCR không hợp lệ")
	}
	row, err := h.service.QualityNCRAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateRecurringPlan(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	var req ConstructionRecurringPlanRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu lịch lặp không hợp lệ")
	}
	row, err := h.service.CreateRecurringPlan(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) GenerateRecurringOccurrences(c *fiber.Ctx) error {
	if err := h.requireQualityChildPermission(c, "recurring_plan", c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	body, _ := bodyMap(c)
	row, err := h.service.GenerateRecurringOccurrences(c.Context(), c.Params("id"), stringValue(body["through_date"]), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CompleteRecurringOccurrence(c *fiber.Ctx) error {
	return h.recurringOccurrenceAction(c, "complete")
}
func (h *Handler) SkipRecurringOccurrence(c *fiber.Ctx) error {
	return h.recurringOccurrenceAction(c, "skip")
}

func (h *Handler) recurringOccurrenceAction(c *fiber.Ctx, action string) error {
	if err := h.requireQualityChildPermission(c, "occurrence", c.Params("id"), []string{"quality.recurring_manage"}); err != nil {
		return err
	}
	var req ConstructionRecurringOccurrenceRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu occurrence không hợp lệ")
	}
	row, err := h.service.RecurringOccurrenceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) requireQualityChildPermission(c *fiber.Ctx, kind, id string, permissions []string) error {
	queries := map[string]string{
		"itp":            `SELECT project_id FROM quality_itp_plans WHERE id=$1`,
		"rfi":            `SELECT project_id FROM quality_inspection_requests WHERE id=$1`,
		"ncr":            `SELECT project_id FROM quality_ncrs WHERE id=$1`,
		"recurring_plan": `SELECT project_id FROM construction_recurring_plans WHERE id=$1`,
		"occurrence":     `SELECT project_id FROM construction_recurring_occurrences WHERE id=$1`,
	}
	query := queries[kind]
	if query == "" {
		return validationError("Loại hồ sơ QA/QC không hợp lệ")
	}
	row, err := h.service.repo.QueryOne(c.Context(), query, id)
	if err != nil {
		return err
	}
	return h.requireProjectPermission(c, cleanID(row["project_id"]), permissions)
}

func (h *Handler) ListProcurement(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"procurement.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	projectFilter := "TRUE"
	if projectID := cleanID(c.Query("project_id")); projectID != "" {
		if err := h.requireProjectPermission(c, projectID, []string{"procurement.view"}); err != nil {
			return err
		}
		projectFilter = fmt.Sprintf("pr.project_id=$%d", len(scopeArgs)+1)
		scopeArgs = append(scopeArgs, projectID)
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT pr.*, p.name AS project_name,
			COALESCE(items.item_count,0) AS item_count,
			COALESCE(items.quantity_requested,0) AS quantity_requested,
			COALESCE(items.accepted_quantity,0) AS accepted_quantity,
			COALESCE(items.stock_received_quantity,0) AS stock_received_quantity,
			acceptance.id AS incoming_acceptance_id,
			acceptance.work_item_id AS incoming_acceptance_work_item_id,
			acceptance.status AS incoming_acceptance_status
		FROM procurement_requests pr
		JOIN projects p ON p.id=pr.project_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS item_count, SUM(pri.quantity_requested) AS quantity_requested,
				SUM(COALESCE(received.accepted_quantity,0)) AS accepted_quantity,
				SUM(LEAST(COALESCE(received.accepted_quantity,0), COALESCE(received.stock_received_quantity,0)+COALESCE(legacy_stock.received_quantity,0))) AS stock_received_quantity
			FROM procurement_request_items pri
			LEFT JOIN LATERAL (
				SELECT SUM(iai.accepted_quantity) AS accepted_quantity,
					SUM(COALESCE(stock.received_quantity,0)) AS stock_received_quantity
				FROM incoming_acceptance_items iai
				JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
				LEFT JOIN LATERAL (
					SELECT SUM(sm.quantity) AS received_quantity
					FROM stock_movements sm
					WHERE sm.movement_type='receive'
					  AND sm.reference_type='incoming_acceptance_item'
					  AND sm.reference_id=iai.id
				) stock ON true
				WHERE iai.procurement_request_item_id=pri.id
			) received ON true
			LEFT JOIN LATERAL (
				SELECT SUM(sm.quantity) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type='receive'
				  AND sm.reference_type='boq_item'
				  AND sm.reference_id=pri.boq_item_id
			) legacy_stock ON true
			WHERE pri.request_id=pr.id
		) items ON true
		LEFT JOIN LATERAL (
			SELECT iar.id, iar.work_item_id, iar.status
			FROM incoming_acceptance_records iar
			WHERE iar.procurement_request_id=pr.id AND iar.status <> 'cancelled'
			ORDER BY iar.created_at DESC, iar.id
			LIMIT 1
		) acceptance ON true
		WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled' AND %s AND %s
		ORDER BY pr.created_at DESC
	`, scopeSQL, projectFilter), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateProcurement(c *fiber.Ctx) error {
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(body["project_id"]), []string{"procurement.create"}); err != nil {
		return err
	}
	if body["request_code"] == nil {
		body["request_code"] = fmt.Sprintf("PR-%d", timeNowUnix())
	}
	req, err := h.service.CreateProcurement(c.Context(), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, req)
}

func (h *Handler) GetProcurement(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), []string{"procurement.view"}); err != nil {
		return err
	}
	req, err := h.service.repo.GetProcurementDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) UpdateProcurement(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), []string{"procurement.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	req, err := h.service.UpdateProcurement(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) SubmitProcurement(c *fiber.Ctx) error  { return h.procurementAction(c, "submit") }
func (h *Handler) ApproveProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "approve") }
func (h *Handler) OrderProcurement(c *fiber.Ctx) error   { return h.procurementAction(c, "order") }
func (h *Handler) ReceiveProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "receive") }

func (h *Handler) procurementAction(c *fiber.Ctx, action string) error {
	permissions := map[string][]string{
		"submit":  {"procurement.create"},
		"approve": {"procurement.approve"},
		"order":   {"procurement.order"},
		"receive": {"procurement.receive"},
	}[action]
	if err := h.requireRecordProjectPermission(c, "procurement_requests", c.Params("id"), permissions); err != nil {
		return err
	}
	actionReq, err := actionRequest(c)
	if err != nil {
		return err
	}
	req, err := h.service.ProcurementAction(c.Context(), c.Params("id"), action, actionReq, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, req)
}

func (h *Handler) ListMaterials(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view", "procurement.view"}); err != nil {
		return err
	}
	return h.list(c, `
		SELECT m.*, p.name AS default_supplier_partner_name, p.tax_code AS default_supplier_tax_code
		FROM materials m
		LEFT JOIN partners p ON p.id=m.default_supplier_partner_id
		WHERE m.active
		ORDER BY m.category, m.name
	`)
}
func (h *Handler) CreateMaterial(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateMaterial(c.Context(), body)
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ListWarehouses(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM warehouses WHERE active ORDER BY type, name`)
}
func (h *Handler) CreateWarehouse(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateWarehouse(c.Context(), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) Stock(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	rows, err := h.service.repo.Query(c.Context(), fmt.Sprintf(`
		SELECT m.id AS material_id, m.code, m.name, m.unit, m.category,
			COALESCE(SUM(CASE
				WHEN sm.movement_type IN ('receive','return_to_warehouse') THEN sm.quantity
				WHEN sm.movement_type IN ('issue','loss') THEN -sm.quantity
				WHEN sm.movement_type='transfer' AND sm.target_warehouse_id IS NULL THEN -sm.quantity
				ELSE 0
			END),0) AS stock_quantity
		FROM materials m
		LEFT JOIN stock_movements sm ON sm.material_id=m.id
			AND (
				sm.project_id IS NULL
				OR EXISTS (
					SELECT 1
					FROM projects p
					WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
				)
			)
		WHERE m.active
		GROUP BY m.id
		ORDER BY m.category, m.name
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) StockPositions(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.StockPositions(c.Context(), actorID(c), cleanID(c.Query("warehouse_id")))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) StockMovements(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.StockMovements(c.Context(), actorID(c), cleanID(c.Query("project_id")), cleanID(c.Query("warehouse_id")))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) StockDocuments(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.ListStockDocuments(c.Context(), actorID(c), cleanID(c.Query("project_id")))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetStockDocument(c *fiber.Ctx) error {
	document, err := h.service.repo.GetStockDocument(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	projectID := cleanID(document["project_id"])
	if projectID != "" {
		if err := h.requireProjectPermission(c, projectID, []string{"warehouse.view"}); err != nil {
			return err
		}
	} else if err := h.requirePermission(c, []string{"warehouse.view"}); err != nil {
		return err
	}
	return response.OK(c, document)
}

func (h *Handler) CreateStockDocument(c *fiber.Ctx) error {
	var req StockDocumentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu phiếu kho không hợp lệ")
	}
	permissions := map[string][]string{
		"receive":                 {"warehouse.receive"},
		"issue":                   {"warehouse.issue"},
		"transfer":                {"warehouse.transfer"},
		"return_to_warehouse":     {"warehouse.surplus_handle"},
		"return_to_subcontractor": {"warehouse.surplus_handle"},
	}[strings.TrimSpace(req.MovementType)]
	if len(permissions) == 0 {
		return validationError("Loại phiếu kho không hợp lệ")
	}
	projectID := cleanID(req.ProjectID)
	if projectID == "" && strings.TrimSpace(req.ReferenceType) == "incoming_acceptance" && cleanID(req.ReferenceID) != "" {
		source, err := h.service.repo.QueryOne(c.Context(), `SELECT project_id FROM incoming_acceptance_records WHERE id=$1 AND status='submitted'`, cleanID(req.ReferenceID))
		if err != nil {
			return err
		}
		projectID = cleanID(source["project_id"])
		req.ProjectID = projectID
	}
	if projectID != "" {
		if err := h.requireProjectPermission(c, projectID, permissions); err != nil {
			return err
		}
	} else if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	document, err := h.service.CreateStockDocument(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func (h *Handler) StockReceive(c *fiber.Ctx) error  { return h.stockAction(c, "receive") }
func (h *Handler) StockIssue(c *fiber.Ctx) error    { return h.stockAction(c, "issue") }
func (h *Handler) StockTransfer(c *fiber.Ctx) error { return h.stockAction(c, "transfer") }
func (h *Handler) StockReturnWarehouse(c *fiber.Ctx) error {
	return h.stockAction(c, "return_to_warehouse")
}
func (h *Handler) StockReturnSubcontractor(c *fiber.Ctx) error {
	return h.stockAction(c, "return_to_subcontractor")
}

func (h *Handler) stockAction(c *fiber.Ctx, movementType string) error {
	var req StockActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nhập xuất kho không hợp lệ")
	}
	req.MovementType = movementType
	permissions := map[string][]string{
		"receive":                 {"warehouse.receive"},
		"issue":                   {"warehouse.issue"},
		"transfer":                {"warehouse.transfer"},
		"return_to_warehouse":     {"warehouse.surplus_handle"},
		"return_to_subcontractor": {"warehouse.surplus_handle"},
	}[movementType]
	if cleanID(req.ProjectID) != "" {
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	} else if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	movement, err := h.service.CreateStockMovement(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, movement)
}

func (h *Handler) MaterialBalances(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.ProjectMaterialBalances(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) WarehouseReceiptOptions(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"warehouse.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.WarehouseReceiptOptions(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ProjectIncomingAcceptances(c *fiber.Ctx) error {
	rows, err := h.service.ProjectIncomingAcceptances(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateIncomingAcceptance(c *fiber.Ctx) error {
	req := IncomingAcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu đầu vào không hợp lệ")
		}
	}
	record, err := h.service.CreateIncomingAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) GetIncomingAcceptance(c *fiber.Ctx) error {
	record, err := h.service.GetIncomingAcceptance(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (h *Handler) UpdateIncomingAcceptance(c *fiber.Ctx) error {
	req := IncomingAcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu đầu vào không hợp lệ")
		}
	}
	record, err := h.service.UpdateIncomingAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (h *Handler) SubmitIncomingAcceptance(c *fiber.Ctx) error {
	record, err := h.service.SubmitIncomingAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}

func (h *Handler) ProjectDailyReports(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectDailyReports(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateDailyReport(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	req := ConstructionDailyReportCreateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo cáo thi công không hợp lệ")
	}
	row, err := h.service.CreateDailyReport(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) UpdateDailyReport(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "operation_daily_reports", c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	req := ConstructionDailyReportCreateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu báo cáo thi công không hợp lệ")
	}
	row, err := h.service.UpdateDailyReport(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
func (h *Handler) UploadDailyReportEvidence(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "operation_daily_reports", c.Params("id"), []string{"operation.report"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp minh chứng là bắt buộc")
	}
	req := WorkItemEvidenceRequest{
		EvidenceType: c.FormValue("evidence_type"),
		DocumentType: c.FormValue("document_type"),
		Name:         c.FormValue("name"),
		Title:        c.FormValue("title"),
		Description:  c.FormValue("description"),
		Caption:      c.FormValue("caption"),
		RevisionNo:   c.FormValue("revision_no"),
		Visibility:   c.FormValue("visibility"),
	}
	row, err := h.service.AddDailyReportEvidenceUpload(c.Context(), c.Params("id"), c.FormValue("work_item_id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ProjectConstructionWorkspace(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view", "quality.view", "rework.view"}); err != nil {
		return err
	}
	workspace, err := h.service.ProjectConstructionWorkspace(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, workspace)
}

func (h *Handler) ProjectConstructionHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view", "planning.view", "construction.handoff.manage"}); err != nil {
		return err
	}
	result, err := h.service.ProjectConstructionHandoff(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ActivateConstructionHandoff(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.handoff.manage"}); err != nil {
		return err
	}
	req := ConstructionHandoffActivateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu bàn giao thi công không hợp lệ")
	}
	result, err := h.service.ActivateConstructionHandoff(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, result)
}

func (h *Handler) CreateConstructionCheckTemplate(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.template.manage"}); err != nil {
		return err
	}
	req := ConstructionCheckTemplateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu mẫu kiểm tra không hợp lệ")
	}
	row, err := h.service.CreateConstructionCheckTemplate(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) CreateConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.check.request"}); err != nil {
		return err
	}
	req := ConstructionCheckRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kiểm tra không hợp lệ")
	}
	row, err := h.service.CreateConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_checks", c.Params("id"), []string{"construction.check.edit"}); err != nil {
		return err
	}
	req := ConstructionCheckUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu cập nhật kiểm tra không hợp lệ")
	}
	row, err := h.service.UpdateConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CompleteConstructionCheck(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_checks", c.Params("id"), []string{"construction.check.perform"}); err != nil {
		return err
	}
	req := ConstructionCheckCompleteRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả kiểm tra không hợp lệ")
	}
	row, err := h.service.CompleteConstructionCheck(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateConstructionAction(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"construction.action.create"}); err != nil {
		return err
	}
	req := ConstructionActionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu việc cần xử lý không hợp lệ")
	}
	row, err := h.service.CreateConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateConstructionAction(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.edit"}); err != nil {
		return err
	}
	req := ConstructionActionUpdateRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu cập nhật việc cần xử lý không hợp lệ")
	}
	row, err := h.service.UpdateConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SubmitConstructionActionResolution(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.resolve"}); err != nil {
		return err
	}
	req := ConstructionActionResolutionRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả khắc phục không hợp lệ")
	}
	row, err := h.service.SubmitConstructionActionResolution(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) VerifyConstructionAction(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.verify"}); err != nil {
		return err
	}
	req := ConstructionActionVerifyRequest{}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Kết quả xác nhận không hợp lệ")
	}
	row, err := h.service.VerifyConstructionAction(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ApproveConstructionActionImpact(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "construction_action_items", c.Params("id"), []string{"construction.action.verify", "replanning.approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveConstructionActionImpact(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectIssues(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectIssues(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateIssue(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"operation.issue_create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateIssue(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) UpdateIssue(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "issues", c.Params("id"), []string{"operation.issue_update"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateIssue(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectAcceptances(c *fiber.Ctx) error {
	rows, err := h.service.ProjectAcceptances(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateAcceptance(c *fiber.Ctx) error {
	req := AcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu không hợp lệ")
		}
	}
	record, err := h.service.CreateAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}
func (h *Handler) GetAcceptance(c *fiber.Ctx) error {
	acc, err := h.service.GetAcceptance(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, acc)
}
func (h *Handler) UpdateAcceptance(c *fiber.Ctx) error {
	req := AcceptanceRequest{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nghiệm thu không hợp lệ")
		}
	}
	record, err := h.service.UpdateAcceptance(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}
func (h *Handler) SubmitAcceptance(c *fiber.Ctx) error {
	record, err := h.service.SubmitAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, record)
}
func (h *Handler) PassAcceptance(c *fiber.Ctx) error { return h.acceptanceAction(c, "pass") }
func (h *Handler) ConditionalPassAcceptance(c *fiber.Ctx) error {
	return h.acceptanceAction(c, "conditional_pass")
}
func (h *Handler) FailAcceptance(c *fiber.Ctx) error { return h.acceptanceAction(c, "fail") }
func (h *Handler) acceptanceAction(c *fiber.Ctx, result string) error {
	permissions := []string{"acceptance.approve"}
	if result == "fail" {
		permissions = []string{"acceptance.reject"}
	}
	if err := h.requireRecordProjectPermission(c, "acceptance_records", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	acc, err := h.service.AcceptanceAction(c.Context(), c.Params("id"), result, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, acc)
}
func (h *Handler) CreateReworkFromAcceptance(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "acceptance_records", c.Params("id"), []string{"rework.create", "acceptance.reject"}); err != nil {
		return err
	}
	rework, err := h.service.CreateReworkFromAcceptance(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, rework)
}

func (h *Handler) ProjectReworks(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"rework.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectReworks(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateRework(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"rework.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateRework(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ApproveRework(c *fiber.Ctx) error {
	return h.reworkStatusAction(c, "approved")
}
func (h *Handler) CompleteRework(c *fiber.Ctx) error {
	return h.reworkStatusAction(c, "done")
}
func (h *Handler) ProjectReplannings(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"replanning.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectReplannings(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateReplanning(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"replanning.create"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateReplanning(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ApproveReplanning(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "replanning_requests", c.Params("id"), []string{"replanning.approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveReplanning(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) reworkStatusAction(c *fiber.Ctx, status string) error {
	if err := h.requireRecordProjectPermission(c, "rework_requests", c.Params("id"), []string{"rework.approve"}); err != nil {
		return err
	}
	row, err := h.service.UpdateReworkStatus(c.Context(), c.Params("id"), status, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ProjectHandover(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectHandover(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) HandoverOptions(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.view", "handover.create"}); err != nil {
		return err
	}
	options, err := h.service.HandoverOptions(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, options)
}
func (h *Handler) CreateHandover(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"handover.create"}); err != nil {
		return err
	}
	var req HandoverCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	row, err := h.service.CreateHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) GetHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.view"}); err != nil {
		return err
	}
	row, err := h.service.HandoverDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
func (h *Handler) SubmitHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.create", "handover.verify"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.SubmitHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
func (h *Handler) CheckHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.verify", "handover.approve"}); err != nil {
		return err
	}
	check, err := h.service.HandoverCheck(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, check)
}

func (h *Handler) UpdateHandoverChecklistItem(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_checklist_items", c.Params("id"), []string{"handover.verify"}); err != nil {
		return err
	}
	var req HandoverChecklistItemRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	row, err := h.service.UpdateHandoverChecklistItem(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
func (h *Handler) CompleteHandover(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "handover_records", c.Params("id"), []string{"handover.approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	done, err := h.service.CompleteHandover(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, done)
}

func (h *Handler) ProjectBudget(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting.budget_view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectBudget(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
func (h *Handler) CreateBudget(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting.budget_approve"}); err != nil {
		return err
	}
	var req BudgetCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu ngân sách không hợp lệ")
	}
	row, err := h.service.CreateBudget(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ApproveBudget(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "accounting_budgets", c.Params("id"), []string{"accounting.budget_approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveBudget(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
func (h *Handler) ListPayables(c *fiber.Ctx) error {
	permissions := []string{"accounting.view", "accounting.payable_view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" {
		if !validActivityUUID(projectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		if err := h.requireProjectPermission(c, projectID, permissions); err != nil {
			return err
		}
		scopeSQL = fmt.Sprintf("(%s) AND pay.project_id=$%d::uuid", scopeSQL, len(scopeArgs)+1)
		scopeArgs = append(scopeArgs, projectID)
	}
	return h.list(c, fmt.Sprintf(`
		SELECT
			pay.*,
			p.name AS project_name,
			COALESCE(inv.invoice_count, 0) AS invoice_count,
			COALESCE(inv.invoice_total, 0) AS invoice_total,
			COALESCE(inv.verified_invoice_total, 0) AS verified_invoice_total,
			COALESCE(sch.schedule_count, 0) AS schedule_count,
			COALESCE(sch.scheduled_total, 0) AS scheduled_total,
			COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
			COALESCE(adv.advance_request_count, 0) AS advance_request_count,
			COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
			COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
			COALESCE(adv.advance_paid_total, 0) AS advance_paid_total,
			COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
			COALESCE(payments.approved_payment_total, 0) AS approved_payment_total,
			latest.latest_invoice_status,
			latest.latest_invoice_number,
			latest.latest_invoice_date,
			COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) AS effective_due_date,
			CASE
				WHEN COALESCE(pay.remaining_amount,0) <= 0 THEN 0
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) IS NULL THEN 0
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date)
			END AS overdue_days,
			CASE
				WHEN COALESCE(pay.remaining_amount,0) <= 0 THEN 'settled'
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) IS NULL THEN 'no_due_date'
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 30 THEN '1_30'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 60 THEN '31_60'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			(
				CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(inv.verified_invoice_total,0) > COALESCE(pay.gross_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(pay.paid_amount,0) > COALESCE(inv.verified_invoice_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(sch.schedule_count,0) = 0 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(sch.scheduled_remaining,0) > COALESCE(pay.remaining_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_paid_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_paid_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
			) AS reconciliation_issue_count,
			CASE
				WHEN (
					CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(inv.verified_invoice_total,0) > COALESCE(pay.gross_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(pay.paid_amount,0) > COALESCE(inv.verified_invoice_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(sch.schedule_count,0) = 0 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(sch.scheduled_remaining,0) > COALESCE(pay.remaining_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_paid_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_paid_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
				) > 0 THEN 'needs_review'
				ELSE 'ok'
			END AS reconciliation_status
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='verified'), 0) AS verified_invoice_total
			FROM input_invoices
			GROUP BY payable_id
		) inv ON inv.payable_id=pay.id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS scheduled_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0) AS next_due_date
			FROM payable_payment_schedules
			GROUP BY payable_id
		) sch ON sch.payable_id=pay.id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS advance_request_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_paid_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_settled_total
			FROM advance_requests
			WHERE payable_id IS NOT NULL
			GROUP BY payable_id
		) adv ON adv.payable_id=pay.id
		LEFT JOIN (
			SELECT
				pa.payable_id,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='payable_payment'), 0) AS approved_payment_total
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			GROUP BY pa.payable_id
		) payments ON payments.payable_id=pay.id
		LEFT JOIN LATERAL (
			SELECT status AS latest_invoice_status, invoice_number AS latest_invoice_number, invoice_date AS latest_invoice_date
			FROM input_invoices ii
			WHERE ii.payable_id=pay.id AND ii.status NOT IN ('deleted','cancelled')
			ORDER BY ii.created_at DESC
			LIMIT 1
		) latest ON true
		WHERE pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		  AND %s
		ORDER BY pay.created_at DESC
	`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreatePayable(c *fiber.Ctx) error {
	var req PayableCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.payable_approve"}); err != nil {
		return err
	}
	row, err := h.service.CreatePayable(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ApprovePayable(c *fiber.Ctx) error {
	return h.payableAction(c, "approve")
}
func (h *Handler) PayPayable(c *fiber.Ctx) error {
	return h.payableAction(c, "pay")
}

func (h *Handler) ListPayablePaymentSchedules(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view"}); err != nil {
		return err
	}
	rows, err := h.service.PayablePaymentSchedules(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create", "accounting.payable_approve"}); err != nil {
		return err
	}
	var req PayablePaymentScheduleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kỳ thanh toán không hợp lệ")
	}
	row, err := h.service.CreatePayablePaymentSchedule(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payable_payment_schedules", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayablePaymentScheduleAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPayableAdvanceRequests(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view"}); err != nil {
		return err
	}
	rows, err := h.service.PayableAdvanceRequests(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create", "accounting.payable_approve"}); err != nil {
		return err
	}
	var req AdvanceRequestCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ứng trước không hợp lệ")
	}
	row, err := h.service.CreatePayableAdvanceRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "advance_requests", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayableAdvanceRequestAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SettlePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "advance_requests", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	var req AdvanceSettlementRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu quyết toán ứng trước không hợp lệ")
	}
	row, err := h.service.SettlePayableAdvanceRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPayablePayments(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	rows, err := h.service.PayablePayments(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayablePayment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	var req PayablePaymentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thanh toán không hợp lệ")
	}
	row, err := h.service.CreatePayablePayment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListPayableInputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.PayableInputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetPayableReconciliation(c *fiber.Ctx) error {
	data, err := h.service.PayableReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectInputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ProjectInputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) UploadPayableInputInvoice(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "File PDF hoá đơn là bắt buộc")
	}
	amountBeforeVAT, err := parseFormFloat(c.FormValue("amount_before_vat"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Giá trị trước VAT không hợp lệ")
	}
	vatAmount, err := parseFormFloat(c.FormValue("vat_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tiền VAT không hợp lệ")
	}
	totalAmount, err := parseFormFloat(c.FormValue("total_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tổng tiền không hợp lệ")
	}
	req := InputInvoiceRequest{
		InvoiceNumber:        c.FormValue("invoice_number"),
		InvoiceSymbol:        c.FormValue("invoice_symbol"),
		InvoiceDate:          c.FormValue("invoice_date"),
		VendorPartnerID:      c.FormValue("vendor_partner_id"),
		VendorName:           c.FormValue("vendor_name"),
		VendorTaxCode:        c.FormValue("vendor_tax_code"),
		AmountBeforeVAT:      amountBeforeVAT,
		VATAmount:            vatAmount,
		TotalAmount:          totalAmount,
		ProcurementRequestID: c.FormValue("procurement_request_id"),
		DocumentID:           c.FormValue("document_id"),
		Notes:                c.FormValue("notes"),
	}
	record, err := h.service.CreateInputInvoice(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) VerifyInputInvoice(c *fiber.Ctx) error {
	return h.inputInvoiceAction(c, "verify")
}

func (h *Handler) RejectInputInvoice(c *fiber.Ctx) error {
	return h.inputInvoiceAction(c, "reject")
}

func (h *Handler) inputInvoiceAction(c *fiber.Ctx, action string) error {
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.InputInvoiceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) GetInputInvoiceFile(c *fiber.Ctx) error {
	file, err := h.service.InputInvoiceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", "input-invoice.pdf", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(file.Path)
}

func parseFormFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func (h *Handler) ListReceivables(c *fiber.Ctx) error {
	permissions := []string{"accounting.view", "accounting.receivable_view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" {
		if !validActivityUUID(projectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		if err := h.requireProjectPermission(c, projectID, permissions); err != nil {
			return err
		}
		scopeSQL = fmt.Sprintf("(%s) AND rec.project_id=$%d::uuid", scopeSQL, len(scopeArgs)+1)
		scopeArgs = append(scopeArgs, projectID)
	}
	return h.list(c, fmt.Sprintf(`
		SELECT
			rec.*,
			p.name AS project_name,
			COALESCE(inv.invoice_count, 0) AS invoice_count,
			COALESCE(inv.invoice_total, 0) AS invoice_total,
			COALESCE(inv.issued_invoice_total, 0) AS issued_invoice_total,
			COALESCE(sched.schedule_count, 0) AS schedule_count,
			COALESCE(sched.scheduled_total, 0) AS scheduled_total,
			COALESCE(sched.scheduled_remaining, 0) AS scheduled_remaining,
			COALESCE(adv.advance_receipt_count, 0) AS advance_receipt_count,
			COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
			COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
			COALESCE(adv.advance_received_total, 0) AS advance_received_total,
			COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
			COALESCE(receipts.approved_receipt_total, 0) AS approved_receipt_total,
			COALESCE(receipts.approved_advance_receipt_total, 0) AS approved_advance_receipt_total,
			COALESCE(receipts.receipt_allocation_total, 0) AS receipt_allocation_total,
			COALESCE(receipts.pending_receipt_count, 0) AS pending_receipt_count,
			latest.latest_invoice_status,
			latest.latest_invoice_number,
			latest.latest_invoice_date,
			COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) AS effective_due_date,
			CASE
				WHEN COALESCE(rec.remaining_amount,0) <= 0 THEN 0
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) IS NULL THEN 0
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date)
			END AS overdue_days,
			CASE
				WHEN COALESCE(rec.remaining_amount,0) <= 0 THEN 'settled'
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) IS NULL THEN 'no_due_date'
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 30 THEN '1_30'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 60 THEN '31_60'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			(
				CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(inv.issued_invoice_total,0) > COALESCE(rec.gross_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(rec.received_amount,0) > COALESCE(inv.issued_invoice_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(sched.schedule_count,0) = 0 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(sched.scheduled_remaining,0) > COALESCE(rec.remaining_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_received_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_received_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(receipts.pending_receipt_count,0) > 0 THEN 1 ELSE 0 END
			) AS reconciliation_issue_count,
			CASE
				WHEN (
					CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(inv.issued_invoice_total,0) > COALESCE(rec.gross_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(rec.received_amount,0) > COALESCE(inv.issued_invoice_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(sched.schedule_count,0) = 0 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(sched.scheduled_remaining,0) > COALESCE(rec.remaining_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_received_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_received_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(receipts.pending_receipt_count,0) > 0 THEN 1 ELSE 0 END
				) > 0 THEN 'needs_review'
				ELSE 'ok'
			END AS reconciliation_status
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='issued'), 0) AS issued_invoice_total
			FROM output_invoices
			GROUP BY receivable_id
		) inv ON inv.receivable_id=rec.id
		LEFT JOIN LATERAL (
			SELECT status AS latest_invoice_status, invoice_number AS latest_invoice_number, invoice_date AS latest_invoice_date
			FROM output_invoices oi
			WHERE oi.receivable_id=rec.id AND oi.status NOT IN ('deleted','cancelled')
			ORDER BY oi.created_at DESC
			LIMIT 1
		) latest ON true
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0) AS next_due_date
			FROM receivable_payment_schedules
			GROUP BY receivable_id
		) sched ON sched.receivable_id=rec.id
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_receipt_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_received','received','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_received_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM client_advance_receipts
			GROUP BY receivable_id
		) adv ON adv.receivable_id=rec.id
		LEFT JOIN (
			SELECT
				ra.receivable_id,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='receivable_receipt'), 0) AS approved_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='advance_receipt'), 0) AS approved_advance_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS receipt_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_receipt_count
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			GROUP BY ra.receivable_id
		) receipts ON receipts.receivable_id=rec.id
		WHERE rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		  AND %s
		ORDER BY rec.created_at DESC
	`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreateReceivable(c *fiber.Ctx) error {
	var req ReceivableCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.receivable_approve"}); err != nil {
		return err
	}
	row, err := h.service.CreateReceivable(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListReceivablePaymentSchedules(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivablePaymentSchedules(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create", "accounting.receivable_approve"}); err != nil {
		return err
	}
	var req PayablePaymentScheduleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kỳ thu không hợp lệ")
	}
	row, err := h.service.CreateReceivablePaymentSchedule(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveReceivablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivable_payment_schedules", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivablePaymentScheduleAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListReceivableAdvanceReceipts(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivableAdvanceReceipts(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create", "accounting.receivable_approve"}); err != nil {
		return err
	}
	var req ReceivableAdvanceReceiptCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ứng trước đầu ra không hợp lệ")
	}
	row, err := h.service.CreateReceivableAdvanceReceipt(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "client_advance_receipts", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivableAdvanceReceiptAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SettleReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "client_advance_receipts", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	var req ReceivableAdvanceSettlementRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu quyết toán ứng trước đầu ra không hợp lệ")
	}
	row, err := h.service.SettleReceivableAdvanceReceipt(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListReceivablePayments(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivablePayments(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivablePayment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	var req ReceivableReceiptCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu phiếu thu không hợp lệ")
	}
	row, err := h.service.CreateReceivablePayment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListReceivableOutputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ReceivableOutputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetReceivableReconciliation(c *fiber.Ctx) error {
	data, err := h.service.ReceivableReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectOutputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ProjectOutputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) UploadReceivableOutputInvoice(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "File PDF hoá đơn là bắt buộc")
	}
	amountBeforeVAT, err := parseFormFloat(c.FormValue("amount_before_vat"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Giá trị trước VAT không hợp lệ")
	}
	vatAmount, err := parseFormFloat(c.FormValue("vat_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tiền VAT không hợp lệ")
	}
	totalAmount, err := parseFormFloat(c.FormValue("total_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tổng tiền không hợp lệ")
	}
	req := OutputInvoiceRequest{
		InvoiceNumber:   c.FormValue("invoice_number"),
		InvoiceSymbol:   c.FormValue("invoice_symbol"),
		InvoiceDate:     c.FormValue("invoice_date"),
		ClientPartnerID: c.FormValue("client_partner_id"),
		ClientName:      c.FormValue("client_name"),
		ClientTaxCode:   c.FormValue("client_tax_code"),
		AmountBeforeVAT: amountBeforeVAT,
		VATAmount:       vatAmount,
		TotalAmount:     totalAmount,
		PaymentID:       c.FormValue("payment_id"),
		DocumentID:      c.FormValue("document_id"),
		Notes:           c.FormValue("notes"),
	}
	record, err := h.service.CreateOutputInvoice(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) IssueOutputInvoice(c *fiber.Ctx) error {
	return h.outputInvoiceAction(c, "issue")
}

func (h *Handler) VoidOutputInvoice(c *fiber.Ctx) error {
	return h.outputInvoiceAction(c, "void")
}

func (h *Handler) outputInvoiceAction(c *fiber.Ctx, action string) error {
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.OutputInvoiceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) GetOutputInvoiceFile(c *fiber.Ctx) error {
	file, err := h.service.OutputInvoiceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", "output-invoice.pdf", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(file.Path)
}

func (h *Handler) ApproveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "approve")
}
func (h *Handler) ReceiveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "receive")
}
func (h *Handler) ListPayments(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	return h.list(c, fmt.Sprintf(`SELECT pay.*, p.name AS project_name FROM payments pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s ORDER BY pay.created_at DESC`, scopeSQL), scopeArgs...)
}
func (h *Handler) CreatePayment(c *fiber.Ctx) error {
	var req PaymentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu phiếu thu/chi không hợp lệ")
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	row, err := h.service.CreatePayment(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}
func (h *Handler) ApprovePayment(c *fiber.Ctx) error {
	return h.paymentAction(c, "approve")
}

func (h *Handler) payableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.payable_approve"}
	if action == "pay" {
		permissions = []string{"accounting.payable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) receivableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.receivable_approve"}
	if action == "receive" {
		permissions = []string{"accounting.receivable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) paymentAction(c *fiber.Ctx, action string) error {
	if err := h.requireRecordProjectPermission(c, "payments", c.Params("id"), []string{"accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PaymentAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.view"}); err != nil {
		return err
	}
	return h.list(c, `
		SELECT s.*,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id', r.id::text,
					'name', r.name,
					'display_name', r.display_name,
					'project_id', COALESCE(ur.project_id::text, '')
				) ORDER BY r.display_name)
				FROM user_roles ur
				JOIN roles r ON r.id=ur.role_id
				WHERE ur.user_id=s.user_id
			), '[]'::jsonb) AS user_roles
		FROM staff s
		WHERE s.status <> 'deleted'
		ORDER BY s.full_name
	`)
}
func (h *Handler) CreateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.create"}); err != nil {
		return err
	}
	return h.create(c, "staff", staffColumns())
}
func (h *Handler) UpdateStaff(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"staff.update"}); err != nil {
		return err
	}
	return h.update(c, "staff", staffColumns())
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
	return h.update(c, "roles", roleColumns())
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
	if err := h.requirePermission(c, []string{"audit.view"}); err != nil {
		return err
	}
	page, err := h.service.AuditLogs(c.Context(), "", actorID(c), auditLogFilterFromQuery(c))
	if err != nil {
		return err
	}
	return response.OK(c, page)
}
func (h *Handler) ProjectAuditLogs(c *fiber.Ctx) error {
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

func (h *Handler) Dashboard(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"report.view"}); err != nil {
		return err
	}
	data, err := h.service.Dashboard(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) CashflowReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := cashflowReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.CashflowReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) CashflowReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := cashflowReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.CashflowReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := cashflowCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="cashflow.csv"`)
	return c.Send(payload)
}
func (h *Handler) TaxReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := taxReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.TaxReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) TaxReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := taxReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.TaxReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := taxCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="tax-report.csv"`)
	return c.Send(payload)
}
func (h *Handler) ProfitReport(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := profitReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.ProfitReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}
func (h *Handler) ProfitReportCSV(c *fiber.Ctx) error {
	permissions := []string{"report.finance.view", "accounting.view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	req := profitReportRequest(c)
	if req.ProjectID != "" {
		if !validActivityUUID(req.ProjectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": req.ProjectID})
		}
		if err := h.requireProjectPermission(c, req.ProjectID, permissions); err != nil {
			return err
		}
	}
	data, err := h.service.ProfitReport(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	payload, err := profitCSV(data)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="profit-report.csv"`)
	return c.Send(payload)
}
func (h *Handler) ProjectReport(c *fiber.Ctx) error     { return h.report(c, "projects") }
func (h *Handler) WarehouseReport(c *fiber.Ctx) error   { return h.report(c, "warehouse") }
func (h *Handler) ProcurementReport(c *fiber.Ctx) error { return h.report(c, "procurement") }
func (h *Handler) AccountingReport(c *fiber.Ctx) error  { return h.report(c, "accounting") }
func (h *Handler) StaffReport(c *fiber.Ctx) error       { return h.report(c, "staff") }
func (h *Handler) report(c *fiber.Ctx, kind string) error {
	permissions := []string{"report.view"}
	if kind == "accounting" {
		permissions = []string{"report.finance.view", "accounting.view"}
	}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	rows, err := h.service.Report(c.Context(), kind, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func cashflowReportRequest(c *fiber.Ctx) CashflowReportRequest {
	return CashflowReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func taxReportRequest(c *fiber.Ctx) TaxReportRequest {
	return TaxReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Source:    strings.TrimSpace(c.Query("source")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func profitReportRequest(c *fiber.Ctx) ProfitReportRequest {
	return ProfitReportRequest{
		ProjectID: cleanID(c.Query("project_id")),
		FromDate:  strings.TrimSpace(c.Query("from")),
		ToDate:    strings.TrimSpace(c.Query("to")),
		GroupBy:   strings.TrimSpace(c.Query("group_by")),
		Status:    strings.TrimSpace(c.Query("status")),
	}
}

func cashflowCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"occurred_at", "payment_code", "project_name", "category", "direction", "payment_type", "payment_method", "status", "inflow_amount", "outflow_amount", "net_amount", "target_type"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["occurred_at"]),
			fmt.Sprint(row["payment_code"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["category"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["payment_type"]),
			fmt.Sprint(row["payment_method"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["inflow_amount"]),
			fmt.Sprint(row["outflow_amount"]),
			fmt.Sprint(row["net_amount"]),
			fmt.Sprint(row["target_type"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func taxCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"invoice_date", "invoice_number", "source", "project_name", "counterparty_name", "tax_code", "amount_before_vat", "vat_amount", "total_amount", "status", "source_type", "source_code", "document_name", "warning_flags"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["invoice_date"]),
			fmt.Sprint(row["invoice_number"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["counterparty_name"]),
			fmt.Sprint(row["tax_code"]),
			fmt.Sprint(row["amount_before_vat"]),
			fmt.Sprint(row["vat_amount"]),
			fmt.Sprint(row["total_amount"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["source_type"]),
			fmt.Sprint(row["source_code"]),
			fmt.Sprint(row["document_name"]),
			fmt.Sprint(row["warning_flags"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func profitCSV(data Record) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"invoice_date", "invoice_number", "direction", "cost_type", "project_name", "counterparty_name", "amount_before_vat", "revenue_amount", "direct_cost_amount", "indirect_cost_amount", "status", "source_type", "source_code", "classification_reason", "warning_flags"}); err != nil {
		return nil, err
	}
	ledger, _ := data["ledger"].([]Record)
	for _, row := range ledger {
		if err := writer.Write([]string{
			fmt.Sprint(row["invoice_date"]),
			fmt.Sprint(row["invoice_number"]),
			fmt.Sprint(row["direction"]),
			fmt.Sprint(row["cost_type"]),
			fmt.Sprint(row["project_name"]),
			fmt.Sprint(row["counterparty_name"]),
			fmt.Sprint(row["amount_before_vat"]),
			fmt.Sprint(row["revenue_amount"]),
			fmt.Sprint(row["direct_cost_amount"]),
			fmt.Sprint(row["indirect_cost_amount"]),
			fmt.Sprint(row["status"]),
			fmt.Sprint(row["source_type"]),
			fmt.Sprint(row["source_code"]),
			fmt.Sprint(row["classification_reason"]),
			fmt.Sprint(row["warning_flags"]),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func (h *Handler) Settings(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"settings.view"}); err != nil {
		return err
	}
	return h.list(c, `SELECT * FROM system_settings ORDER BY key`)
}

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
