"""Render a narrated Vietnamese TechNote from the verified browser evidence, entirely locally."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont
import json, subprocess, re, math

HERE=Path(__file__).resolve().parent
OUT=HERE.parents[2]/'output'/'technote-level4-ui-refactor-2026-10-26'
OUT.mkdir(parents=True,exist_ok=True)
ASSETS=OUT/'assets';ASSETS.mkdir(exist_ok=True)
DATA=json.loads((HERE/'browser-summary.json').read_text())
assert DATA['total']==128 and DATA['counts']['PASS']==122 and DATA['counts']['FAIL']==4 and DATA['counts']['BLOCKED']==2
def evidence(view,name,open_help=False):
    row=next(r for r in DATA['results'] if r['view']==view and r['name']==name)
    return HERE/row['evidence_dir']/row['openScreenshot' if open_help else 'screenshot']
font_path='/System/Library/Fonts/Supplemental/Arial.ttf'
bold_path='/System/Library/Fonts/Supplemental/Arial Bold.ttf'
def font(size,bold=False):return ImageFont.truetype(bold_path if bold else font_path,size)
def wrap(text,draw,f,max_width):
    lines=[];line=''
    for word in text.split():
        candidate=(line+' '+word).strip()
        if draw.textlength(candidate,font=f)>max_width and line:lines.append(line);line=word
        else:line=candidate
    if line:lines.append(line)
    return lines
def paragraph(draw,text,x,y,width,size=27,color='#26374e',bold=False):
    f=font(size,bold)
    for block in text.split('\n'):
        for line in wrap(block,draw,f,width):draw.text((x,y),line,font=f,fill=color);y+=size+10
    return y
def shot(canvas,file,box,crop=None):
    im=Image.open(file).convert('RGB')
    if crop:im=im.crop(crop)
    x,y,w,h=box;im.thumbnail((w,h),Image.Resampling.LANCZOS)
    canvas.paste(im,(x+(w-im.width)//2,y+(h-im.height)//2))
def base(index,title,status='ĐÃ KIỂM CHỨNG',tone='#0b6b4b'):
    im=Image.new('RGB',(1280,720),'#edf2f7');d=ImageDraw.Draw(im)
    d.rectangle((0,0,1280,95),fill='#14243c')
    d.text((36,18),'TechNote Level 4 · Consflow · 26-10-2026',font=font(18),fill='#c7d9ef')
    d.text((36,46),title,font=font(31,True),fill='white')
    d.rounded_rectangle((1010,17,1246,46),radius=8,fill=tone)
    d.text((1020,22),status,font=font(14,True),fill='white')
    d.text((1207,63),f'{index}/8',font=font(17),fill='#c7d9ef')
    return im
scenes=[
 {'title':'Kết quả kiểm tra giao diện','kind':'summary','status':'CÒN ĐIỂM MỞ','tone':'#ad6200','lines':[
  'Mục đích là kiểm tra giao diện đã rút gọn trên máy tính và điện thoại giả lập, rồi đối chiếu các vùng được giữ nguyên.',
  'Trong một trăm hai mươi tám ca, một trăm hai mươi hai ca đạt tiêu chí; bốn ca mobile tràn ngang và hai ca thông báo bị chặn bởi lỗi API local.',
  'Mục tiêu dọn text toàn phạm vi còn điểm mở, nên chưa kết luận nghiệm thu toàn bộ.']},
 {'title':'Desktop: trang, bảng và dialog','kind':'desktop','lines':[
  'Trên desktop, tiêu đề trang gọn hơn; dữ liệu, trạng thái và nhãn hành động nghiệp vụ vẫn còn.',
  'Các dialog đã được mở và đóng bằng dữ liệu thử có sẵn. Import nhân sự được kiểm tra bằng tài khoản Developer đã có quyền.',
  'Các ca này không lưu, duyệt, xóa hoặc thực hiện giao dịch nghiệp vụ.']},
 {'title':'Mobile: phát hiện tràn ngang','kind':'mobile','status':'4 CA CHƯA ĐẠT','tone':'#a53328','lines':[
  'Bốn ca kiểm tra trên mobile bị tràn ngang, tại ba bề mặt: Audit, Nhật ký sự kiện và Thảo luận.',
  'Độ rộng nội dung lần lượt là tám trăm hai mươi, tám trăm mười bốn và năm trăm bốn mươi pixel, vượt viewport cấu hình ba trăm chín mươi pixel.',
  'Cần đo theo viewport cấu hình vì Chrome có thể tự giãn khung bố cục. Đây là giả lập, chưa kiểm tra trên điện thoại vật lý.']},
 {'title':'Trợ giúp: mở, đóng và focus','kind':'help','lines':[
  'Trợ giúp đã được kiểm tra tại ảnh đại diện, thống kê, hồ sơ kế toán và sửa lỗi trên cả hai viewport.',
  'Nút mở được bằng bàn phím hoặc chạm. Escape đóng trợ giúp, trả focus về nút và giữ dialog cha mở.',
  'Click ngoài và chuyển focus ra ngoài cũng đóng nội dung trợ giúp. Giới hạn nhập và cảnh báo cần thiết vẫn hiển thị.']},
 {'title':'Pipeline và Workflow Form giữ nguyên','kind':'protected','lines':[
  'Mười bốn đối chiếu browser cho cùng nội dung nhìn thấy với baseline trước refactor.',
  'Các mẫu gồm Pipeline Builder, thư viện Form, quy trình công trình, tab quy trình và các thẻ hoặc Form nhúng.',
  'So sánh source xác nhận một trăm mười hai tệp và mười hai hàm được bảo vệ không đổi. Ảnh có thể khác trạng thái hover hoặc focus; không tuyên bố mọi pixel giống nhau.']},
 {'title':'Còn copy tĩnh cần dọn tiếp','kind':'copy','status':'ĐỀ XUẤT · CẦN DUYỆT','tone':'#ad6200','lines':[
  'Còn tagline ở thanh trên cùng, mô tả lặp trong hai thẻ Profile và câu hướng dẫn ở đầu hai dialog báo cáo.',
  'Đề xuất bỏ hoặc rút gọn phần lặp, chuyển giải thích bổ sung sang trợ giúp. Thanh dùng chung cần áp riêng ngoài vùng Pipeline và Form.',
  'Các điểm này mới được ghi nhận trong UAT; chưa sửa thêm source ứng dụng.']},
 {'title':'Dialog thông báo: API local trả 500','kind':'api','status':'BỊ CHẶN','tone':'#a53328','lines':[
  'Danh mục thông báo local trả mã HTTP năm trăm trên cả desktop và mobile.',
  'Dialog hiển thị chưa có nhóm thông báo, nên ca tải danh mục chưa đạt. Backend vẫn là image local có sẵn.',
  'Nguyên nhân gốc chưa xác định. Cần chẩn đoán riêng; chưa quy kết lỗi này cho refactor giao diện.']},
 {'title':'Điểm chốt và bước tiếp theo','kind':'closing','status':'CHƯA NGHIỆM THU','tone':'#ad6200','lines':[
  'Đã có bằng chứng browser local và đối chiếu vùng loại trừ. Còn tràn ngang mobile, copy cần dọn và lỗi danh mục thông báo cần xử lý.',
  'Đề xuất duyệt phạm vi sửa các điểm mở, sau đó kiểm tra lại và mời người dùng đại diện nghiệm thu.',
  'Chưa có điện thoại vật lý, chưa đo thời gian thao tác và chưa kiểm tra mọi biến thể Form. Không commit, push hoặc deploy trong lượt này.']},
]
frames=[];audio_scenes=[];subtitles=[];chapters=[];at=0.0;counter=0
for index,scene in enumerate(scenes,1):
    canvas=base(index,scene['title'],scene.get('status','ĐÃ KIỂM CHỨNG'),scene.get('tone','#0b6b4b'));d=ImageDraw.Draw(canvas)
    kind=scene['kind']
    if kind=='summary':
        for x,n,label,color in [(25,'128','Ca đã chạy','#1b4d8d'),(275,'122','Đạt tiêu chí ca','#0b6b4b'),(525,'4','Ca mobile chưa đạt','#a53328'),(775,'2','Ca bị chặn','#a53328'),(1025,'14','Đối chiếu vùng giữ nguyên','#1b4d8d')]:
            d.rounded_rectangle((x,145,x+230,340),radius=16,fill='white');d.text((x+24,173),n,font=font(66,True),fill=color);paragraph(d,label,x+24,267,188,21)
        paragraph(d,'Local · Dữ liệu giả định · Chrome 156\nDesktop 1440×1000 / Mobile giả lập 390×844',50,390,1170,30)
        paragraph(d,'Còn điểm mở → chưa nghiệm thu toàn phạm vi',50,505,1160,32,'#9c5900',True)
    elif kind=='desktop':
        shot(canvas,evidence('desktop','procurement'),(24,115,880,450),(288,64,1440,850))
        paragraph(d,'Giữ tiền, trạng thái và hành động có nhãn',928,145,310,29,bold=True)
        paragraph(d,'Bỏ tagline và dòng giải thích lặp.',928,315,310,25)
    elif kind=='mobile':
        shot(canvas,evidence('mobile','audit'),(35,112,260,455));shot(canvas,evidence('mobile','project-discussion-rail'),(320,112,260,455))
        paragraph(d,'Viewport: 390 px',640,140,565,36,bold=True)
        paragraph(d,'Audit: 820 px\nNhật ký sự kiện: 814 px\nThảo luận: 540 px',640,235,565,29,'#a53328',True)
        paragraph(d,'4 ca chưa đạt · 3 bề mặt',640,440,565,29,'#a53328')
    elif kind=='help':
        shot(canvas,evidence('mobile','profile-avatar-help',True),(25,110,295,455))
        shot(canvas,evidence('desktop','profile-avatar-help',True),(345,110,550,455),(432,190,1010,820))
        paragraph(d,'Mở bằng nút\nEscape giữ dialog\nTrả focus\nClick ngoài đóng',925,160,310,27,bold=True)
    elif kind=='protected':
        comparison=next(c for c in DATA['protected_comparisons'] if c['view']=='desktop' and c['name']=='workflow-forms')
        d.text((38,113),'TRƯỚC REFACTOR',font=font(20,True),fill='#334c6e');d.text((658,113),'HIỆN TẠI',font=font(20,True),fill='#0b6b4b')
        shot(canvas,HERE/comparison['evidence_dir']/comparison['baseline'],(25,150,600,350),(288,64,1440,850))
        shot(canvas,HERE/comparison['evidence_dir']/comparison['current'],(650,150,600,350),(288,64,1440,850))
        paragraph(d,'14 đối chiếu cùng nội dung · 112 tệp + 12 hàm không đổi',45,520,1190,29,bold=True)
    elif kind=='copy':
        shot(canvas,evidence('desktop','profile'),(24,110,815,455),(288,64,1440,700))
        paragraph(d,'Còn mô tả lặp',870,140,365,34,'#9c5900',True)
        paragraph(d,'Thanh trên cùng\nHai thẻ Profile\nĐầu báo cáo',870,230,365,28)
        paragraph(d,'Cần duyệt trước khi sửa.',870,450,365,25,'#9c5900')
    elif kind=='api':
        shot(canvas,evidence('desktop','notifications-loaded'),(25,140,900,375),(400,350,1040,650))
        paragraph(d,'HTTP 500',955,170,280,40,'#a53328',True)
        paragraph(d,'2 ca bị chặn\nNguyên nhân gốc chưa rõ',955,290,280,26)
    else:
        items=[('Đã kiểm chứng','Browser local và vùng giữ nguyên','#0b6b4b'),('Đề xuất','Sửa mobile, dọn copy; chẩn đoán API','#9c5900'),('Chưa kiểm chứng','Người dùng thực và điện thoại vật lý','#9c5900')]
        for y,(label,body,color) in zip([145,285,425],items):
            d.rounded_rectangle((45,y,1235,y+115),radius=12,fill='white');d.text((70,y+18),label,font=font(28,True),fill=color);paragraph(d,body,395,y+25,800,28)
    chapter_start=at
    for line_index,line in enumerate(scene['lines'],1):
        counter+=1;prefix=f'{index:02d}-{line_index:02d}'
        text_file=ASSETS/f'{prefix}.txt';audio=ASSETS/f'{prefix}.aiff';image=ASSETS/f'{prefix}.png'
        text_file.write_text(line+'\n')
        subprocess.run(['say','-v','Linh','-r','175','-o',str(audio),'-f',str(text_file)],check=True)
        info=subprocess.check_output(['afinfo',str(audio)],text=True)
        seconds=float(re.search(r'estimated duration:\s*([0-9.]+)',info).group(1));assert seconds>0.1,(prefix,info)
        duration=math.ceil((seconds+0.45)*12)/12
        frame=canvas.copy();draw=ImageDraw.Draw(frame);draw.rectangle((0,587,1280,720),fill='#102039')
        caption_lines=wrap(line,draw,font(24),1200);assert len(caption_lines)<=3,(prefix,caption_lines)
        for j,text in enumerate(caption_lines):draw.text((40,607+j*32),text,font=font(24),fill='white')
        frame.save(image)
        frames.append({'image':str(image),'duration':duration});audio_scenes.append({'audio':str(audio),'duration':duration})
        subtitles.append({'start':at,'end':at+duration,'text':line});at+=duration
    chapters.append({'chapter':index,'title':scene['title'],'start':chapter_start,'end':at})
def stamp(value):
    ms=round(value*1000);return f'{ms//3600000:02d}:{ms//60000%60:02d}:{ms//1000%60:02d},{ms%1000:03d}'
(OUT/'phu-de.srt').write_text('\n\n'.join(f'{i}\n{stamp(s["start"])} --> {stamp(s["end"])}\n{s["text"]}' for i,s in enumerate(subtitles,1))+'\n')
(OUT/'kich-ban.md').write_text('# TechNote Level 4 — Kiểm tra giao diện\n\nLời đọc tổng hợp: Linh vi_VN. Ảnh từ browser local, dữ liệu giả định.\n\n'+'\n\n'.join(f'## {i}. {s["title"]}\n\n'+'\n\n'.join(s['lines']) for i,s in enumerate(scenes,1))+'\n')
(OUT/'chuong.json').write_text(json.dumps(chapters,ensure_ascii=False,indent=2)+'\n')
(OUT/'manifest.json').write_text(json.dumps({'frames':frames,'scenes':audio_scenes,'output':str(OUT/'TechNote-Level4-Kiem-tra-giao-dien.mp4')},ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'output':str(OUT),'segments':len(frames),'estimated_duration':at},ensure_ascii=False))
