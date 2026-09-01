package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"net/url"
)

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

func (h *Handler) GetTemplatePDF(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.view"}); err != nil {
		return err
	}
	file, err := h.service.PipelineTemplatePDF(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, file.MIMEType)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", "workflow-template.pdf", url.PathEscape(file.Filename)))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.Send(file.Data)
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

func (h *Handler) UpdateTemplateLayout(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"pipeline.edit"}); err != nil {
		return err
	}
	req := PipelineTemplateLayoutRequest{}
	if err := c.BodyParser(&req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu bố cục workflow không hợp lệ")
	}
	template, err := h.service.UpdateTemplateLayout(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
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
	markWorkflowFormVersionRequestPresence(c.Body(), &req)
	version, err := h.service.UpdateWorkflowFormVersion(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, version)
}

func markWorkflowFormVersionRequestPresence(body []byte, req *WorkflowFormVersionRequest) {
	if req == nil {
		return
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(body, &fields) != nil {
		return
	}
	_, req.SchemaProvided = fields["schema_json"]
	_, req.ValidationProvided = fields["validation_json"]
	_, req.FieldsProvided = fields["fields"]
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
