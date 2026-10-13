package core

// Both the chooser and mutation use the same exact runtime/form source contract.
// Rev8+ combines the legacy contract work into execution_package and retains the
// published hardcopy-dispatch form pin. An arbitrary project form is not a
// contract signing source, even when it contains an approved_by signature.
const contractSigningFormSourceSQL = `
	wfi.status='submitted'
	AND wi.project_id=wfi.project_id AND rs.project_id=wfi.project_id
	AND wi.run_step_id=rs.id AND wi.run_node_id=wfi.run_node_id
	AND wi.pipeline_run_id=wfi.pipeline_run_id
	AND rs.run_node_id=wfi.run_node_id
	AND rs.form_version_id=wfi.form_version_id
	AND (
		(rs.business_code='outbound_contract.app_sign' AND wft.code='client_internal_contract_approval')
		OR (rs.business_code='outbound_contract.execution_package' AND wft.code='client_hardcopy_dispatch')
	)`
