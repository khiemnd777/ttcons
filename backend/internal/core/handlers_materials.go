package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"strings"
)

func (h *Handler) ListMaterials(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"material.view", "warehouse.view", "procurement.view"}); err != nil {
		return err
	}
	rows, err := h.service.repo.ListActiveMaterials(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateMaterial(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.CreateMaterial(c.Context(), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) UpdateMaterial(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	body, err := bodyMap(c)
	if err != nil {
		return err
	}
	row, err := h.service.UpdateMaterial(c.Context(), c.Params("id"), body, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
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
				WHEN sm.movement_type IN ('receive','return_to_warehouse','adjustment_in') THEN sm.quantity
				WHEN sm.movement_type IN ('issue','loss','adjustment_out') THEN -sm.quantity
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
	permissions := stockDocumentPermissions(strings.TrimSpace(req.MovementType))
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

func (h *Handler) UpdateStockDocument(c *fiber.Ctx) error {
	var req StockDocumentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu chỉnh sửa phiếu kho không hợp lệ")
	}
	current, err := h.service.repo.GetStockDocument(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	currentPermissions := stockDocumentPermissions(strings.TrimSpace(fmt.Sprint(current["movement_type"])))
	requestedPermissions := stockDocumentPermissions(strings.TrimSpace(req.MovementType))
	if len(currentPermissions) == 0 || len(requestedPermissions) == 0 {
		return validationError("Loại phiếu kho không hợp lệ")
	}
	currentProjectID := cleanID(current["project_id"])
	requestedProjectID := cleanID(req.ProjectID)
	if currentProjectID != "" {
		if err := h.requireProjectPermission(c, currentProjectID, []string{"warehouse.adjust"}); err != nil {
			return err
		}
		if err := h.requireProjectPermission(c, currentProjectID, currentPermissions); err != nil {
			return err
		}
	} else {
		if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
			return err
		}
		if err := h.requirePermission(c, currentPermissions); err != nil {
			return err
		}
	}
	if requestedProjectID != "" {
		if err := h.requireProjectPermission(c, requestedProjectID, []string{"warehouse.adjust"}); err != nil {
			return err
		}
		if err := h.requireProjectPermission(c, requestedProjectID, requestedPermissions); err != nil {
			return err
		}
	} else {
		if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
			return err
		}
		if err := h.requirePermission(c, requestedPermissions); err != nil {
			return err
		}
	}
	document, err := h.service.UpdateStockDocument(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, document)
}

func (h *Handler) CreateStockAdjustment(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	var req StockAdjustmentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu điều chỉnh tồn kho không hợp lệ")
	}
	document, err := h.service.CreateStockAdjustment(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, document)
}

func stockDocumentPermissions(movementType string) []string {
	return map[string][]string{
		"receive":                 {"warehouse.receive"},
		"issue":                   {"warehouse.issue"},
		"transfer":                {"warehouse.transfer"},
		"return_to_warehouse":     {"warehouse.surplus_handle"},
		"return_to_subcontractor": {"warehouse.surplus_handle"},
	}[movementType]
}

func (h *Handler) StockReceive(c *fiber.Ctx) error { return h.stockAction(c, "receive") }

func (h *Handler) StockIssue(c *fiber.Ctx) error { return h.stockAction(c, "issue") }

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

func (h *Handler) AdjustSiteMaterialUsage(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.requireProjectPermission(c, projectID, []string{"warehouse.adjust"}); err != nil {
		return err
	}
	var req SiteMaterialUsageAdjustmentRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu điều chỉnh sử dụng vật tư không hợp lệ")
	}
	balance, err := h.service.AdjustSiteMaterialUsage(c.Context(), projectID, c.Params("materialId"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, balance)
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
