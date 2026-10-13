"""Canonical synthetic BM03/BM08 Development UAT, with preserved older data.

Application writes use existing authorized APIs only. SQL is read-only evidence.
Requires the established temporary API/auth helper and bundled Python runtime.
Never writes directly to workflow/acceptance/handover tables or edits exports.
"""
import datetime
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('prior_uat', '/private/tmp/consflow-bm02-receipt-mapping-uat.py')
prior = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prior)
api = prior.api


def save(name, value):
    (OUT / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def read_sql(sql):
    raw = subprocess.check_output(['docker', 'exec', 'ttcons-postgres-1', 'sh', '-c', 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "$1"', 'sh', sql], text=True)
    return json.loads(raw)


def baseline(prefix):
    tables = ['projects', 'project_members', 'work_items', 'pipeline_templates', 'pipeline_template_nodes', 'pipeline_template_steps', 'pipeline_template_edges', 'pipeline_runs', 'pipeline_run_nodes', 'pipeline_run_steps', 'workflow_form_templates', 'workflow_form_versions', 'workflow_form_fields', 'workflow_form_instances', 'workflow_form_events', 'workflow_form_signatures', 'workflow_form_pdf_exports', 'acceptance_records', 'acceptance_checklist_items', 'handover_records', 'handover_checklist_items', 'documents', 'subcontractor_reconciliations', 'subcontractor_reconciliation_items', 'owner_client_reconciliations', 'owner_client_reconciliation_items', 'evidence_records', 'work_reports', 'work_report_revisions', 'work_report_events']
    existing = read_sql("SELECT coalesce(json_agg(table_name),'[]') FROM information_schema.columns WHERE table_schema='public' AND column_name='id';")
    tables = [table for table in tables if table in existing]
    fields = ','.join("'" + table + "',(SELECT coalesce(json_agg(json_build_object('id',id,'row_md5',md5(row_to_json(r)::text)) ORDER BY id),'[]') FROM " + table + ' r)' for table in tables)
    data = read_sql("SELECT json_build_object('database',current_database(),'migration_count',(SELECT count(*) FROM schema_migrations)," + fields + ');')
    save(prefix + '-database.json', data)
    print(json.dumps({'captured_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'tables': tables, 'counts': {key: len(value) for key, value in data.items() if isinstance(value, list)}}))


def inventory():
    prior.renew()
    projects = api.call('/projects?limit=100')
    save('projects-inventory.json', projects)
    catalog = api.call('/workflow-form-templates')
    save('form-catalog.json', catalog)
    print(json.dumps({'forms': [{'code': row['template_code'], 'version': row['version_no'], 'id': row['version_id']} for row in catalog if row['template_code'] in ['builtin_subcontractor_checklist', 'client_subcontractor_acceptance_package']]}))
    for project_id in ['e71b542f-2b5f-4ae5-bef5-75fb92acc206', 'c0acb592-b160-43c9-b8b2-370f386d4de0']:
        work = api.call('/projects/' + project_id + '/work-items')
        save(project_id + '-work-items.json', work)
        for suffix in ['acceptances', 'handover/options']:
            value = api.call('/projects/' + project_id + '/' + suffix)
            save(project_id + '-' + suffix.replace('/', '-') + '.json', value)
        rows = work if isinstance(work, list) else work.get('items', [])
        print(json.dumps({'project_id': project_id, 'work_items': [{key: row.get(key) for key in ['id', 'title', 'status', 'run_node_id', 'step_type', 'form_version_id']} for row in rows]}, ensure_ascii=False))


def declarations():
    official = json.loads((OUT / 'official-template-inventory.json').read_text())
    node_keys = ['business_code', 'name', 'description', 'type', 'order_index', 'responsible_role_ids', 'required_permission', 'permissions', 'entry_conditions', 'exit_conditions', 'approval_rule', 'approval_policy_json', 'rework_enabled', 'replanning_enabled', 'fail_path', 'status']
    step_keys = ['business_code', 'name', 'description', 'step_type', 'required_role_ids', 'required_permission', 'checklist_required', 'document_required', 'photo_required', 'form_version_id', 'form_required', 'surface_kind', 'domain_action_key', 'order_index']
    def copy_node(code, selected_steps=None):
        source = next(n for n in official['nodes'] if n['business_code'] == code)
        node = {k: source.get(k) for k in node_keys}
        node['steps'] = [{k: s.get(k) for k in step_keys} for s in source['steps'] if selected_steps is None or s['business_code'] in selected_steps]
        return node
    client = copy_node('incoming_acceptance', ['incoming.subcontractor_package'])
    builtin = json.loads(json.dumps(client))
    builtin['steps'][0].update(name='[Synthetic] BM03 checklist thầu phụ có sẵn', business_code='incoming.subcontractor_checklist', form_version_id='61e9bf6c-33fb-4e62-ad98-5dc6d9cdba19', domain_action_key='source.subcontractor_reconciliation_ready')
    final = copy_node('outgoing_acceptance')
    # Retain both physical and accounting steps and the node approval policy.
    # Only physical handover is tested; do not advance or claim financial closure.
    closure = copy_node('handover_finance')
    closure['name'] = 'Bàn giao công trình và hồ sơ kế toán'
    result = {}
    for key, nodes in [('builtin', [builtin]), ('client', [client]), ('handover', [final, closure])]:
        nodes = json.loads(json.dumps(nodes))
        for i, n in enumerate(nodes):
            n['order_index'] = i + 1
        result[key] = {'name': '[Synthetic Development] ' + {'builtin': 'BM03 checklist hợp lệ', 'client': 'BM03 gói client độc lập', 'handover': 'BM08 nghiệm thu cuối và bàn giao'}[key], 'code': 'DEV-' + key.upper() + '-BM0308-20261013', 'nodes': nodes}
    save('fixture-declarations.json', result)
    print('Saved declarations only; no API writes.')


def persist(state):
    save('fixture-state.json', state)


def scaffold():
    prior.renew()
    declared = json.loads((OUT / 'fixture-declarations.json').read_text())
    state = json.loads((OUT / 'fixture-state.json').read_text()) if (OUT / 'fixture-state.json').exists() else {}
    for key, d in declared.items():
        s = state.setdefault(key, {})
        if not s.get('template_id'):
            nodes = [{k: v for k, v in n.items() if k != 'steps'} for n in d['nodes']]
            t = api.call('/pipeline-templates', 'POST', {'name': d['name'], 'description': '[Synthetic] Isolated acceptance fixtures. Preserve official step guards. No whole Rev10 or accounting-closure acceptance claim.', 'nodes': nodes})
            s.update(template_id=t['id'], node_ids=[n['id'] for n in t['nodes']], step_ids=[])
            persist(state)
        while len(s['step_ids']) < sum(len(n['steps']) for n in d['nodes']):
            index = 0
            for ni, n in enumerate(d['nodes']):
                for step in n['steps']:
                    if index >= len(s['step_ids']):
                        saved = api.call('/pipeline-nodes/' + s['node_ids'][ni] + '/steps', 'POST', step)
                        s['step_ids'].append(saved['id']); persist(state)
                    index += 1
        if len(d['nodes']) > 1 and not s.get('edge_id'):
            edge = api.call('/pipeline-templates/' + s['template_id'] + '/edges', 'POST', {'from_node_id': s['node_ids'][0], 'to_node_id': s['node_ids'][1], 'condition_type': 'always', 'condition_config': {}, 'label': 'Sau nghiệm thu cuối hợp lệ', 'priority': 1})
            s['edge_id'] = edge['id']; persist(state)
        if not s.get('published'):
            t = api.call('/pipeline-templates/' + s['template_id'] + '/publish', 'POST', {})
            save(key + '-published-template.json', t)
            s['published'] = True; persist(state)
        if not s.get('project_id'):
            p = api.call('/projects', 'POST', {'name': d['name'], 'code': d['code'], 'client_name': '[Synthetic] Chủ đầu tư nghiệm thu thử', 'address': '[Synthetic] Khu thử Development', 'pipeline_template_id': s['template_id']})
            s['project_id'] = p['id']; persist(state)
        if not s.get('member_id'):
            actor = json.loads(api.PRIVATE.read_text())['author']['user']['id']
            options = api.call('/projects/' + s['project_id'] + '/members')
            staff = next(r for r in options['options']['staff'] if r['user_id'] == actor)
            member = api.call('/projects/' + s['project_id'] + '/members', 'POST', {'staff_id': staff['id'], 'role_ids': ['PM'], 'project_role_ids': [], 'notes': '[Synthetic] Canonical UAT owner, Admin actor. Ordinary role parity is outside this acceptance.'})
            s.update(member_id=member['id'], staff_id=staff['id']); persist(state)
        if not s.get('pm_set'):
            api.call('/projects/' + s['project_id'] + '/project-manager', 'PUT', {'staff_id': s['staff_id']})
            s['pm_set'] = True; persist(state)
        if not s.get('roles_covered'):
            roles = sorted({'PM'} | {role for node in d['nodes'] for role in node['responsible_role_ids']} | {role for node in d['nodes'] for step in node['steps'] for role in step['required_role_ids']})
            # Canonical membership only on this new synthetic project. No global RBAC edits.
            api.call('/projects/' + s['project_id'] + '/members/' + s['member_id'], 'PUT', {'staff_id': s['staff_id'], 'role_ids': roles, 'project_role_ids': [], 'notes': '[Synthetic] Test project source roles; no real project or global permission change.'})
            s['roles_covered'] = roles; persist(state)
        if not s.get('run_id'):
            ready = api.call('/projects/' + s['project_id'] + '/startup-readiness'); save(key + '-startup-readiness.json', ready)
            assert ready['ready_to_start'], ready
            run = api.call('/projects/' + s['project_id'] + '/start-pipeline', 'POST', {'template_id': s['template_id']})
            s['run_id'] = run['id']; persist(state)
        items = api.call('/projects/' + s['project_id'] + '/work-items')
        save(key + '-work-items-created.json', items)
        s['work_items'] = [{'id': i['id'], 'title': i['title'], 'status': i['status'], 'run_node_id': i['run_node_id'], 'run_step_id': i['run_step_id']} for i in items]
        persist(state)
    print(json.dumps(state, ensure_ascii=False))


def prepare_forms():
    prior.renew()
    state = json.loads((OUT / 'fixture-state.json').read_text())
    for key in ['builtin', 'client', 'handover']:
        s = state[key]
        f = api.call('/work-items/' + s['work_items'][0]['id'] + '/form')
        if f['instance']['status'] != 'draft':
            print(key, 'already submitted'); continue
        if key in ['builtin', 'client']:
            rows = [
                {'title': '[Synthetic] Bề mặt sàn trước chống thấm', 'standard_reference': '[Synthetic] Quy trình thử S-01', 'drawing_reference': '[Synthetic] KT-WC-02 RevA', 'technical_requirement': 'Sàn sạch; góc chân tường xử lý theo bản vẽ thử', 'claimed_quantity': 23, 'confirmed_quantity': 23, 'design_quantity': 25, 'actual_quantity': 24.8, 'unit': 'm2', 'status': 'done', 'owner_response': 'accepted', 'quality_result': 'pass', 'quality_note': '[Synthetic] Đạt yêu cầu bề mặt tại vùng thử', 'measurement_note': '[Synthetic] Đo theo sơ đồ ô thử'},
                {'title': '[Synthetic] Thử nước chống thấm 24 giờ', 'standard_reference': '[Synthetic] Quy trình thử S-02', 'drawing_reference': '[Synthetic] KT-WC-03 RevB', 'technical_requirement': 'Ngâm nước thử 50 mm trong 24 giờ; không rò', 'claimed_quantity': 20, 'confirmed_quantity': 20, 'design_quantity': 25, 'actual_quantity': 25, 'unit': 'm2', 'status': 'done', 'owner_response': 'accepted', 'quality_result': 'pass', 'quality_note': '[Synthetic] Không phát hiện rò trong mẫu kiểm tra'},
                {'title': '[Synthetic] Độ dốc và lát gạch sàn', 'standard_reference': '[Synthetic] Mẫu duyệt MAT-03', 'drawing_reference': '[Synthetic] KT-WC-04 RevC', 'technical_requirement': 'Độ dốc thử 1.5-2%; ron không quá 1 mm', 'claimed_quantity': 21, 'confirmed_quantity': 21, 'design_quantity': 25, 'actual_quantity': 24.6, 'unit': 'm2', 'status': 'done', 'owner_response': 'accepted', 'quality_result': 'pass', 'quality_note': '[Synthetic] Đạt yêu cầu đo tại mẫu thử'}]
            answers = {'reference_no': 'UAT-BM03-' + key.upper() + '-20261013', 'check_date': '2026-10-13', 'subcontractor_name': '[Synthetic] Tổ chống thấm UAT', 'subcontractor_representative': '[Synthetic] Đại diện tổ thử', 'work_package': 'flooring', 'construction_area': '[Synthetic] WC tầng 1 - Khu thử', 'checklist_items': rows, 'overall_comment': '[Synthetic] Dữ liệu thử; không phải hồ sơ công trình hay biên bản pháp lý.', 'acceptance_conclusion': 'accepted'}
            if key == 'client':
                answers['quantity_items'] = [{'item': row['title'], 'claimed_quantity': row['claimed_quantity'], 'approved_quantity': row['confirmed_quantity'], 'note': '[Synthetic] Khối lượng đối soát khác khối lượng đo thực tế'} for row in rows]
        else:
            answers = {'check_date': '2026-10-13', 'client_representative': '[Synthetic] Đại diện CĐT thử', 'company_representative': 'Consflow Admin', 'handover_scope': 'handover_item', 'scope_detail': '[Synthetic] Khu WC tầng 1 hoàn thiện thử', 'checklist_items': [{'title': '[Synthetic] Hoàn thiện chống thấm và lát gạch', 'output_ref': '[Synthetic] OUT-01', 'status': 'done', 'client_decision': 'pass', 'comment': '[Synthetic] Quan sát và kiểm tra đạt trong fixture UAT'}], 'final_client_note': '[Synthetic] Chỉ nghiệm thu thử luồng hồ sơ trên Development; không có hiệu lực pháp lý.'}
        assert set(answers) <= {field['field_key'] for field in f['fields']}
        save(key + '-form-answers.json', answers)
        result = api.call('/workflow-form-instances/' + s['form_id'] + '/draft', 'PUT', {'answers': answers, 'expected_answers': f['instance'].get('answers', {}), 'comment': '[Synthetic] Chuẩn bị phiếu hợp lệ để chốt qua giao diện'})
        save(key + '-draft-saved.json', result)
        print(key, s['form_id'], result['instance']['status'] if 'instance' in result else 'saved')


def final_acceptance():
    prior.renew()
    state = json.loads((OUT / 'fixture-state.json').read_text())
    s = state['handover']; work = s['work_items'][0]['id']
    form = api.call('/work-items/' + work + '/form'); save('final-source-signed-pdf.json', form)
    assert form['instance']['status'] == 'submitted' and len(form['signatures']) == 1 and len(form['pdf_exports']) == 1
    s['signature_id'] = form['signatures'][0]['id']; s['pdf_export_id'] = form['pdf_exports'][0]['id']; persist(state)
    if not s.get('acceptance_id'):
        request = {'work_item_id': work, 'form_instance_id': s['form_id'], 'pdf_export_id': s['pdf_export_id'], 'type': 'final', 'title': '[Synthetic] Nghiệm thu cuối khu WC thử BM08', 'acceptance_date': '2026-10-13', 'location': '[Synthetic] Khu WC tầng 1', 'client_representative': '[Synthetic] Đại diện CĐT thử', 'contractor_representative': 'Consflow Admin', 'scope_summary': '[Synthetic] Hoàn thiện chống thấm và lát gạch', 'notes': '[Synthetic] Kiểm tra nghiệp vụ trên Development, không có hiệu lực pháp lý.', 'checklist': [{'item_index': 0, 'title': '[Synthetic] Hoàn thiện chống thấm và lát gạch', 'description': '[Synthetic] Hoàn thiện mẫu thử theo tiêu chí của phiếu đã chốt', 'decision': 'pass', 'severity': 'low', 'photo_required': False, 'note': '[Synthetic] Kiểm tra vùng mẫu đạt'}]}
        save('final-acceptance-request.json', request)
        record = api.call('/projects/' + s['project_id'] + '/acceptances', 'POST', request)
        save('final-acceptance-created.json', record); s['acceptance_id'] = record['id']; persist(state)
    record = api.call('/acceptances/' + s['acceptance_id']); assert record['signature_status'] == 'signed' and record['document_status'] == 'linked', record
    if record['status'] == 'draft':
        record = api.call('/acceptances/' + s['acceptance_id'] + '/submit', 'POST', {}); save('final-acceptance-submitted.json', record)
    if record['status'] != 'completed':
        record = api.call('/acceptances/' + s['acceptance_id'] + '/pass', 'POST', {'comment': '[Synthetic] Hồ sơ thử đủ tiêu chí, chữ ký nội bộ và PDF nguồn thực tế.'}); save('final-acceptance-passed.json', record)
    assert record['status'] == 'completed' and record['result'] == 'pass', record
    reconciliation = api.call('/work-items/' + work + '/owner-client-reconciliation'); save('owner-reconciliation-before.json', reconciliation)
    assert reconciliation['available'], reconciliation
    if reconciliation['reconciliation']['status'] != 'submitted':
        body = {'comment': '[Synthetic] Chốt đối chiếu với hồ sơ nghiệm thu cuối đã hoàn tất', 'items': [{'item_index': 0, 'acceptance_id': s['acceptance_id'], 'decision': 'accepted', 'comment': '[Synthetic] Liên kết đúng hồ sơ nghiệm thu cuối cùng công trình'}]}
        result = api.call('/work-items/' + work + '/owner-client-reconciliation/submit', 'POST', body); save('owner-reconciliation-submitted.json', result)
    save('handover-options-valid-final.json', api.call('/projects/' + s['project_id'] + '/handover/options'))
    print(json.dumps({'acceptance_id': s['acceptance_id'], 'signature_id': s['signature_id'], 'pdf_export_id': s['pdf_export_id'], 'status': record['status'], 'result': record['result']}))


DOCUMENTS = {
    'as_built_documents': ('As-built drawing and inspection record', ['Test WC zone: 5.0 m x 5.0 m; schematic QA fixture.', 'Drawing: UAT-ASBUILT-01 RevA; floor drain and waterproofing.', 'Internal test review: QA and PM acknowledgement, synthetic only.']),
    'operation_manuals': ('Operation and maintenance instructions', ['Keep test floor drain unobstructed; inspect joint seal monthly.', 'Clean with neutral detergent; check leak test record before use.', 'Document UAT-OM-01 RevA; applies only to the Development fixture.']),
    'warranty_documents': ('Warranty dossier test record', ['Test item: waterproofing and floor tile fixture.', 'Synthetic warranty interval: 2026-10-13 to 2027-10-13.', 'No commercial commitment; no actual client warranty is issued.']),
    'signed_handover_minutes': ('Two-party handover acknowledgement fixture', ['Source acceptance: final, completed, pass on 2026-10-13.', 'Test client party: Synthetic CĐT; contractor party: Consflow Admin.', 'Both are simulated test acknowledgements; no legal effect.'])}


def make_documents():
    from reportlab.pdfgen import canvas
    from reportlab.lib.pagesizes import A4
    for key, (title, lines) in DOCUMENTS.items():
        p = OUT / (key + '.pdf')
        c = canvas.Canvas(str(p), pagesize=A4)
        c.setTitle('[Synthetic] ' + title)
        c.setFillColorRGB(0.15, 0.25, 0.45); c.setFont('Helvetica-Bold', 16)
        c.drawString(42, 795, 'SYNTHETIC DEVELOPMENT UAT')
        c.setFont('Helvetica-Bold', 13); c.drawString(42, 760, title)
        c.setFillColorRGB(0.1, 0.1, 0.1); c.setFont('Helvetica', 10)
        c.drawString(42, 730, 'Project: DEV-HANDOVER-BM0308-20261013')
        for i, line in enumerate(lines):
            c.drawString(42, 690-i*28, line.replace('CĐT', 'client'))
        c.setStrokeColorRGB(.25,.4,.65)
        if key == 'as_built_documents':
            c.rect(70, 350, 250, 250); c.line(70, 350, 270, 400); c.circle(270,400,8); c.drawString(335, 470, 'Test floor drain')
        if key == 'signed_handover_minutes':
            c.drawString(60, 460, 'TEST CLIENT ACKNOWLEDGEMENT'); c.drawString(300,460,'TEST CONTRACTOR ACKNOWLEDGEMENT')
            c.line(70,410,130,435); c.line(130,435,190,405); c.line(310,410,370,435); c.line(370,435,440,405)
            c.drawString(60, 380, 'Synthetic mark, no legal effect'); c.drawString(300,380,'Synthetic mark, no legal effect')
        c.setFont('Helvetica-Bold', 10); c.drawString(42, 100, 'QA TEST FILE ONLY - NOT A REAL PROJECT OR LEGAL DOCUMENT')
        c.showPage(); c.save()
    print('Created four actual synthetic test PDFs. Render and inspect before uploads.')


def upload_documents():
    prior.renew(); state = json.loads((OUT / 'fixture-state.json').read_text()); s = state['handover']
    s.setdefault('documents', {})
    for key in DOCUMENTS:
        if key in s['documents']: continue
        path = OUT / (key + '.pdf'); boundary = 'bm08-uat-' + uuid.uuid4().hex
        fields = {'document_type': key, 'name': '[Synthetic] BM08 ' + key}
        data = b''
        for k, v in fields.items():
            data += f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode()
        data += f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{path.name}"\r\nContent-Type: application/pdf\r\n\r\n'.encode() + path.read_bytes() + f'\r\n--{boundary}--\r\n'.encode()
        token = json.loads(api.PRIVATE.read_text())['author']['token']
        req = urllib.request.Request(api.BASE + '/projects/' + s['project_id'] + '/documents/upload', data=data, headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'multipart/form-data; boundary=' + boundary}, method='POST')
        with urllib.request.urlopen(req, timeout=30) as response:
            document = json.load(response)['data']
        s['documents'][key] = document['id']; save(key + '-uploaded.json', document); persist(state)
    print(json.dumps(s['documents']))


if __name__ == '__main__':
    {'before': lambda: baseline('before'), 'after': lambda: baseline('after'), 'inventory': inventory, 'declarations': declarations, 'scaffold': scaffold, 'prepare-forms': prepare_forms, 'final-acceptance': final_acceptance, 'make-documents': make_documents, 'upload-documents': upload_documents}[sys.argv[1]]()
