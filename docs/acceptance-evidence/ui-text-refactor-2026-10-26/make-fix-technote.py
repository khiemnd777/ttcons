"""Bản cập nhật TechNote4 từ kết quả khắc phục đã kiểm chứng; không thay video lịch sử."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont
import json, subprocess, re, math

HERE = Path(__file__).resolve().parent
OUT = HERE.parents[2] / 'output' / 'technote-level4-ui-refactor-2026-10-26' / 'sau-khac-phuc'
OUT.mkdir(parents=True, exist_ok=True)
ASSETS = OUT / 'assets'; ASSETS.mkdir(exist_ok=True)
data = json.loads((HERE / 'fix-summary.json').read_text())
assert data['browser_counts'] == {'PASS': 56, 'FAIL': 0, 'BLOCKED': 0}
def font(size, bold=False):
    return ImageFont.truetype('/System/Library/Fonts/Supplemental/Arial Bold.ttf' if bold else '/System/Library/Fonts/Supplemental/Arial.ttf', size)
def wrap(text, draw, f, width):
    lines = []; line = ''
    for word in text.split():
        candidate = (line + ' ' + word).strip()
        if draw.textlength(candidate, font=f) > width and line:
            lines.append(line); line = word
        else: line = candidate
    if line: lines.append(line)
    return lines
def text(draw, body, x, y, width, size=27, color='#18334e', bold=False):
    for block in body.split('\n'):
        for line in wrap(block, draw, font(size, bold), width):
            draw.text((x,y),line,font=font(size,bold),fill=color); y += size+10
def shot(canvas, name, box, key='screenshot'):
    row = next(row for row in data['results'] if row['name'] == name)
    im = Image.open(HERE / row['evidence_dir'] / row[key]).convert('RGB')
    x,y,w,h = box; im.thumbnail((w,h), Image.Resampling.LANCZOS)
    canvas.paste(im,(x+(w-im.width)//2,y+(h-im.height)//2))
scenes = [
    ('Catalog thông báo đã hoạt động', [
        'Đã xác minh: catalog trả mã hai trăm, giữ đủ tám mươi ba loại thông báo và mảng quyền theo hợp đồng hiện có.',
        'Dialog phân biệt lỗi tải với danh sách rỗng. Sáu nhóm trong dữ liệu local đều bắt buộc và vẫn luôn bật.'
    ]),
    ('Ba trang mobile không còn tràn ngang', [
        'Audit, Nhật ký sự kiện và Thảo luận đã đạt phép đo tại ba chiều rộng: ba trăm sáu mươi, ba trăm chín mươi và bốn trăm ba mươi pixel.',
        'Bản sửa giới hạn filter và grid tại trang. Không che phần tràn bằng cách ẩn dữ liệu hoặc sửa bên trong Pipeline.'
    ]),
    ('Pipeline và Workflow Form được giữ nguyên', [
        'Một trăm mười hai tệp và mười hai vùng source được bảo vệ vẫn nguyên văn so với baseline.',
        'Mười bốn đối chiếu browser cùng nội dung. Header Workflow Form vẫn giữ nguyên với cả dấu gạch chéo cuối và chữ hoa.'
    ]),
    ('Kết quả kỹ thuật và giới hạn kiểm thử', [
        'Năm mươi sáu ca browser hợp lệ đều đạt. Sáu trăm hai mươi hai test frontend và regression catalog trên PostgreSQL thật đều đạt.',
        'Luồng lưu nhóm tùy chọn và lỗi lưu một phần dùng HTTP fixtures có ghi rõ. Không thay dữ liệu catalog thật để tạo điều kiện thử.'
    ]),
    ('Phần còn cần nghiệm thu', [
        'Copy lặp ở Profile và đầu báo cáo đã bỏ. Cảnh báo nghiệp vụ, nguồn và lần gửi vẫn giữ nguyên.',
        'Chưa có điện thoại vật lý hoặc người dùng đại diện. Chưa chạy toàn bộ backend tests; chưa commit, push hoặc triển khai production.'
    ]),
]
frames=[]; audios=[]; captions=[]; chapters=[]; at=0
for i,(title,lines) in enumerate(scenes,1):
    canvas=Image.new('RGB',(1280,720),'#eef3f8'); draw=ImageDraw.Draw(canvas)
    draw.rectangle((0,0,1280,98),fill='#14283f')
    draw.text((30,14),'TechNote Level 4 · Consflow · Sau khắc phục · 26-10-2026',font=font(19),fill='#cce0f4')
    draw.text((30,46),title,font=font(32,True),fill='white')
    if i==1:
        shot(canvas,'desktop-real-notification-catalog',(25,113,900,465))
        text(draw,'ĐÃ KIỂM CHỨNG\nHTTP 200\n83 loại\n21 đang bật\n6 nhóm bắt buộc',940,140,315,29,bold=True)
    elif i==2:
        for x,name in zip([25,310,595],['mobile-390-audit','mobile-390-events','mobile-390-discussion']): shot(canvas,name,(x,110,265,465))
        text(draw,'Viewport kiểm tra\n360 / 390 / 430 px\n\nCuộn cục bộ\nở thanh lọc',910,165,340,28,bold=True)
    elif i==3:
        text(draw,'SOURCE GIỮ NGUYÊN',45,140,1100,34,bold=True)
        text(draw,'112 tệp + 12 vùng được bảo vệ',45,205,1140,38,color='#0c6b4c',bold=True)
        text(draw,'14 đối chiếu nội dung browser khớp baseline\n\nForm nhúng được so sánh trong vùng Form.\nThay đổi tagline AppShell bên ngoài được ghi riêng.',45,300,1130,29)
    elif i==4:
        text(draw,'56 / 56 ca browser hợp lệ đạt\n622 / 622 tests frontend đạt\nPostgreSQL thật: regression catalog đạt',45,135,1150,36,color='#0c6b4c',bold=True)
        text(draw,'PHẠM VI KIỂM THỬ\nAPI và layout local thật. Lỗi tải, empty và lưu nhóm\ntùy chọn dùng HTTP fixtures có kiểm soát.',45,365,1150,27)
    else:
        shot(canvas,'desktop-profile-copy',(25,115,820,450))
        text(draw,'CHƯA KIỂM CHỨNG\nĐiện thoại vật lý\nNgười dùng đại diện\nToàn bộ backend suite',870,150,370,27,color='#975d0a',bold=True)
        text(draw,'Đề xuất: nghiệm thu local\ntrước khi giao phát hành.',870,420,360,25)
    chapter_start=at
    for j,line in enumerate(lines,1):
        prefix=f'{i:02d}-{j:02d}'; speech=ASSETS/(prefix+'.txt'); audio=ASSETS/(prefix+'.aiff'); frame=ASSETS/(prefix+'.png')
        speech.write_text(line+'\n')
        subprocess.run(['say','-v','Linh','-r','175','-o',str(audio),'-f',str(speech)],check=True)
        info=subprocess.check_output(['afinfo',str(audio)],text=True)
        seconds=float(re.search(r'estimated duration:\s*([0-9.]+)',info).group(1)); assert seconds>0.1
        duration=math.ceil((seconds+.45)*12)/12
        im=canvas.copy(); d=ImageDraw.Draw(im); d.rectangle((0,587,1280,720),fill='#102039')
        caption=wrap(line,d,font(24),1200); assert len(caption)<=3
        for k,part in enumerate(caption): d.text((40,607+k*32),part,font=font(24),fill='white')
        im.save(frame); frames.append({'image':str(frame),'duration':duration}); audios.append({'audio':str(audio),'duration':duration})
        captions.append((at,at+duration,line)); at+=duration
    chapters.append({'chapter':i,'title':title,'start':chapter_start,'end':at})
def stamp(value):
    ms=round(value*1000); return f'{ms//3600000:02d}:{ms//60000%60:02d}:{ms//1000%60:02d},{ms%1000:03d}'
(OUT/'phu-de.srt').write_text('\n\n'.join(f'{i}\n{stamp(start)} --> {stamp(end)}\n{line}' for i,(start,end,line) in enumerate(captions,1))+'\n')
(OUT/'kich-ban.md').write_text('# TechNote Level 4 — Sau khắc phục\n\nLời đọc tổng hợp Linh vi_VN. Ảnh browser local thật; HTTP fixtures được phân biệt với API thật.\n\n'+'\n\n'.join(f'## {i}. {title}\n\n'+'\n\n'.join(lines) for i,(title,lines) in enumerate(scenes,1))+'\n')
(OUT/'chuong.json').write_text(json.dumps(chapters,ensure_ascii=False,indent=2)+'\n')
(OUT/'manifest.json').write_text(json.dumps({'frames':frames,'scenes':audios,'output':str(OUT/'TechNote-Level4-Sau-khac-phuc.mp4')},ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'segments':len(frames),'seconds':at,'output':str(OUT)},ensure_ascii=False))
