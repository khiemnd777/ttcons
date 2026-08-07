package core

import (
	"context"
	"database/sql"
)

// validateLegacyProjectEntryPrerequisites preserves immutable Rev3/Rev4 behavior.
// New revisions must use workflow forms and connector conditions instead.
func validateLegacyProjectEntryPrerequisites(ctx context.Context, tx *sql.Tx, projectID, templateID string) error {
	var usesLegacyEntryMode bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pipeline_template_edges
			WHERE template_id=$1 AND condition_type='project_entry_mode'
		)
	`, templateID).Scan(&usesLegacyEntryMode); err != nil {
		return err
	}
	if !usesLegacyEntryMode {
		return nil
	}

	var requiresClientBOQ bool
	if err := tx.QueryRowContext(ctx, `
		SELECT workflow_entry_mode='client_boq' FROM projects WHERE id=$1
	`, projectID).Scan(&requiresClientBOQ); err != nil {
		return err
	}
	if !requiresClientBOQ {
		return nil
	}

	var hasClientBOQ bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM documents
			WHERE project_id=$1 AND target_type='project_intake' AND document_type='client_boq' AND status='active'
		)
	`, projectID).Scan(&hasClientBOQ); err != nil {
		return err
	}
	if !hasClientBOQ {
		return validationError("Công trình dùng workflow legacy nhánh BOQ nhưng chưa có hồ sơ BOQ đầu vào")
	}
	return nil
}
