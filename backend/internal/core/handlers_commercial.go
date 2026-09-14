package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
)

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

func (h *Handler) GetBOQ(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.view"}); err != nil {
		return err
	}
	item, err := h.service.GetBOQ(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, item)
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

func (h *Handler) ReviseBOQ(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "boq_items", c.Params("id"), []string{"boq.update"}); err != nil {
		return err
	}
	var req BOQReviseRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu yêu cầu chỉnh sửa BOQ không hợp lệ")
	}
	item, err := h.service.ReviseBOQ(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
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
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	result, err := h.service.BOQProjectAction(c.Context(), c.Params("id"), "submit", req.WorkItemContext, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) ApproveBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"boq.approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	result, err := h.service.BOQProjectAction(c.Context(), c.Params("id"), "approve", req.WorkItemContext, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, result)
}

func (h *Handler) GenerateProcurementFromBOQ(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"procurement.create", "procurement.approve", "boq.approve"}); err != nil {
		return err
	}
	mutationReq, err := actionRequest(c)
	if err != nil {
		return err
	}
	req, err := h.service.GenerateProcurementFromBOQ(c.Context(), c.Params("id"), mutationReq.WorkItemContext, actorID(c), actorName(c))
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
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	quotation, err := h.service.SubmitSupplierQuotation(c.Context(), c.Params("id"), req.WorkItemContext, actorID(c), actorName(c))
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
	if err := h.requireContractChildPermission(c, "version", c.Params("id"), []string{"contract.sign"}); err != nil {
		return err
	}
	var req ContractVersionSignRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ký hợp đồng không hợp lệ")
	}
	row, err := h.service.SignContractVersion(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) contractVersionAction(c *fiber.Ctx, action string, permissions []string) error {
	if err := h.requireContractChildPermission(c, "version", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ContractVersionAction(c.Context(), c.Params("id"), action, actionComment(req), req.WorkItemContext, actorID(c), actorName(c))
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
	row, err := h.service.ActivateContract(c.Context(), c.Params("id"), actionComment(req), req.WorkItemContext, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) UploadProjectDocument(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"project.update"}); err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp hồ sơ là bắt buộc")
	}
	document, err := h.service.UploadProjectDocument(c.Context(), c.Params("id"), "project_intake", c.Params("id"), c.FormValue("document_type"), c.FormValue("name"), fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func (h *Handler) UploadContractDocument(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.create", "contract.dispatch", "contract.scan_manage"}); err != nil {
		return err
	}
	contract, err := h.service.ContractDetail(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tệp hồ sơ hợp đồng là bắt buộc")
	}
	document, err := h.service.UploadProjectDocument(c.Context(), cleanID(contract["project_id"]), "contract", c.Params("id"), c.FormValue("document_type"), c.FormValue("name"), fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func (h *Handler) UploadContractDispatchDocument(c *fiber.Ctx) error {
	if err := h.requireContractChildPermission(c, "dispatch", c.Params("id"), []string{"contract.scan_manage", "contract.dispatch"}); err != nil {
		return err
	}
	row, err := h.service.repo.QueryOne(c.Context(), `
		SELECT contract.project_id
		FROM contract_dispatches dispatch JOIN contracts contract ON contract.id=dispatch.contract_id
		WHERE dispatch.id=$1
	`, c.Params("id"))
	if err != nil {
		return err
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Bản scan hợp đồng là bắt buộc")
	}
	document, err := h.service.UploadProjectDocument(c.Context(), cleanID(row["project_id"]), "contract_returned_scan", c.Params("id"), "signed_contract_scan", c.FormValue("name"), fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func (h *Handler) GetProjectDocumentFile(c *fiber.Ctx) error {
	file, err := h.service.ProjectDocumentFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "ho-so")
}

func (h *Handler) CreateContractDispatch(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "contracts", c.Params("id"), []string{"contract.dispatch"}); err != nil {
		return err
	}
	var req ContractDispatchRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi hồ sơ giấy không hợp lệ")
	}
	row, err := h.service.CreateContractDispatch(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ReceiveContractDispatch(c *fiber.Ctx) error {
	if err := h.requireContractChildPermission(c, "dispatch", c.Params("id"), []string{"contract.scan_manage"}); err != nil {
		return err
	}
	var req ContractDispatchReceiveRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu nhận lại hồ sơ giấy không hợp lệ")
	}
	row, err := h.service.ReceiveContractDispatch(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
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

func (h *Handler) QSReviewContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "qs-review", []string{"contract.variation_qs_review"})
}

func (h *Handler) PurchaseContractVariation(c *fiber.Ctx) error {
	return h.contractVariationAction(c, "purchase", []string{"contract.variation_purchase"})
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

func (h *Handler) GenerateContractVariationProcurement(c *fiber.Ctx) error {
	if err := h.requireContractChildPermission(c, "variation", c.Params("id"), []string{"procurement.create", "contract.variation_purchase"}); err != nil {
		return err
	}
	row, err := h.service.GenerateProcurementFromContractVariation(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
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
		"dispatch":  `SELECT c.project_id FROM contract_dispatches child JOIN contracts c ON c.id=child.contract_id WHERE child.id=$1`,
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

func (h *Handler) ProcurementProjects(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"procurement.view"}); err != nil {
		return err
	}
	req, err := projectOptionsRequest(c)
	if err != nil {
		return err
	}
	data, err := h.service.ProcurementProjects(c.Context(), req, actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
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
	req, err := h.service.ProcurementWorkspace(c.Context(), c.Params("id"), actorID(c))
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

func (h *Handler) SubmitProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "submit") }

func (h *Handler) ApproveProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "approve") }

func (h *Handler) OrderProcurement(c *fiber.Ctx) error { return h.procurementAction(c, "order") }

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
