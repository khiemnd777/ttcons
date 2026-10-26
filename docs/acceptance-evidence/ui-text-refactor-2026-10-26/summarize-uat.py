"""Merge valid reruns by scenario, retaining raw attempts and explicit verification limits."""
from pathlib import Path
from PIL import Image, ImageChops, ImageDraw, ImageFont
import json, hashlib

ROOT = Path(__file__).resolve().parent
FOLDERS = ['browser', 'browser-recheck', 'browser-extended', 'browser-developer',
           'browser-workflow-tab', 'browser-embedded', 'browser-embedded-final', 'browser-notifications-final', 'browser-mobile-layout', 'browser-supplement']
rows, comparisons, errors = {}, {}, []
for folder in FOLDERS:
    data = json.loads((ROOT / folder / 'results.json').read_text())
    for row in data['results']:
        if row['name'] == 'login' and (row['view'], row['name']) in rows:
            continue
        row['evidence_dir'] = folder
        measurement=row.get('measurement',{})
        if measurement:
            measurement.setdefault('layoutViewport',measurement['viewport'])
            measurement['viewport']=390 if row['view']=='mobile' else 1440
            if measurement['documentWidth']>measurement['viewport']+1 and row['status']=='PASS':
                row['status']='FAIL';row['error']=f'LAYOUT_FAIL: document overflow {measurement["documentWidth"]}/{measurement["viewport"]}'
        rows[row['view'], row['name']] = row
    for comparison in data.get('comparisons', []):
        comparison['evidence_dir'] = folder
        comparisons[comparison['view'], comparison['name']] = comparison
    errors.extend(data.get('errors', []))

for comparison in comparisons.values():
    a = Image.open(ROOT / comparison['evidence_dir'] / comparison['current']).convert('RGB')
    b = Image.open(ROOT / comparison['evidence_dir'] / comparison['baseline']).convert('RGB')
    diff = ImageChops.difference(a, b)
    comparison['whole_image_exact'] = diff.getbbox() is None
    comparison['whole_image_changed_percent'] = round(sum(diff.convert('L').histogram()[1:]) / (a.width * a.height) * 100, 6)
    # The Form source composition is the content area; shell focus/active navigation is recorded above.
    if comparison['name'] == 'embedded-form':
        crop = (288, 64, a.width, a.height) if comparison['view'] == 'desktop' else (0, 64, a.width, a.height)
        body_diff = ImageChops.difference(a.crop(crop), b.crop(crop))
        comparison['form_content_exact'] = body_diff.getbbox() is None

result = {
    'status': 'BROWSER_VERIFIED_WITH_FINDINGS',
    'scope': 'Agent-run Chrome browser checks, local synthetic guide data; no representative user acceptance',
    'desktop': [1440,1000], 'mobile_emulation': [390,844], 'physical_mobile': 'NOT_RUN',
    'baseline': 'fba39cdcf72a2d6714d5eac3d15eba8cdf4014c9',
    'counts': {status: sum(row['status'] == status for row in rows.values()) for status in ['PASS','FAIL','BLOCKED','NOT_RUN','CHECK_REQUIRED']},
    'results': list(rows.values()), 'protected_comparisons': list(comparisons.values()), 'page_errors': errors,
    'findings': [
      {'id':'MOBILE-01','status':'OPEN','priority':'P1','fact':'Bốn ca mobile tại ba bề mặt có documentWidth vượt viewport cấu hình390px: Audit820, EventLogs814, Thảo luận540 (hai ca).',
       'cause':'Đo bằng viewport cấu hình, không dùng innerWidth tự giãn của Chrome. Đối chiếu baseline được lưu ở từng ca; không tự quy lỗi cho refactor.',
       'proposal':'Giới hạn các vùng bảng/rail trong viewport và kiểm tra lại responsive sau khi được duyệt; không đổi Pipeline/Form.'},
      {'id':'UI-TEXT-01','status':'OPEN','priority':'P2','fact':'Còn tagline tĩnh ở AppShell, hai thẻ Profile và hai banner báo cáo.',
       'sources':['frontend/src/layouts/AppShell.tsx:268','frontend/src/modules/profile/ProfilePage.tsx:323','frontend/src/modules/profile/ProfilePage.tsx:347','frontend/src/modules/operations/ConstructionReportDetailDialog.tsx:241','frontend/src/modules/work-reports/WorkReportReviewDialog.tsx:280'],
       'proposal':'Rút/bỏ copy lặp, cân nhắc help cho hướng dẫn bổ sung; AppShell phải áp riêng ngoài vùng loại trừ. Cần duyệt trước sửa.'},
      {'id':'LOCAL-API-01','status':'OPEN','priority':'P2','fact':'Endpoint catalog thông báo local trả HTTP500; dialog không chứng minh tải danh mục thành công.',
       'cause':'Chưa xác định nguyên nhân gốc; backend image không đổi. Chưa quy kết cho refactor frontend.','proposal':'Chẩn đoán riêng, không sửa trong lượt browser/UAT.'},
    ],
    'limits':['Không người dùng đại diện','Không điện thoại vật lý/camera','Không lưu/phê duyệt/xóa nghiệp vụ hoặc thay quyền',
              'Không kiểm tra mọi biến thể của mọi Workflow Form; source comparison bảo vệ 112 file và 12 hàm',
              'InitialPasswordChange cần tài khoản must_change_password: chưa kiểm tra trực quan',
              'Lượt đầu dùng reload dày gặp rate limit và bị loại; những ca quay về login không được tính PASS'],
}
result['total'] = len(rows)
for row in result['results']:
    if row.get('screenshot'):
        file = ROOT / row['evidence_dir'] / row['screenshot']
        row['screenshot_sha256'] = hashlib.sha256(file.read_bytes()).hexdigest()
(ROOT / 'browser-summary.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')

font = ImageFont.truetype('/System/Library/Fonts/Supplemental/Arial.ttf',17)
sheet_dir = ROOT / 'review-sheets'; sheet_dir.mkdir(exist_ok=True)
for view in ['desktop','mobile']:
    selected = [row for row in rows.values() if row['view'] == view and row.get('screenshot')]
    width,height,columns = (360,250,4) if view == 'desktop' else (195,422,6)
    for index in range(0,len(selected),12):
        chunk = selected[index:index+12]; sheet = Image.new('RGB',(columns*width,3*(height+32)),'#e8eef5'); draw = ImageDraw.Draw(sheet)
        for j,row in enumerate(chunk):
            im=Image.open(ROOT/row['evidence_dir']/row['screenshot']).convert('RGB');im.thumbnail((width,height))
            x=(j%columns)*width;y=(j//columns)*(height+32)
            sheet.paste(im,(x,y));draw.text((x+4,y+height+3),row['name'][:31],font=font,fill='#17283f')
        sheet.save(sheet_dir/f'{view}-{index//12+1}.jpg',quality=88)
print(json.dumps({'total':result['total'],'counts':result['counts'],'protected':len(comparisons),'page_errors':len(errors)},ensure_ascii=False))
