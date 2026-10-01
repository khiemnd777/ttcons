from __future__ import annotations

import html
import re
from pathlib import Path

from PIL import Image as PILImage
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_RIGHT
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    HRFlowable,
    Image,
    KeepTogether,
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)


ROOT = Path(__file__).resolve().parent
SOURCE = ROOT / "huong-dan-demo-cong-trinh.md"
OUTPUT = ROOT / "huong-dan-demo-cong-trinh.pdf"

pdfmetrics.registerFont(TTFont("TahomaVN", "/System/Library/Fonts/Supplemental/Tahoma.ttf"))
pdfmetrics.registerFont(TTFont("TahomaVN-Bold", "/System/Library/Fonts/Supplemental/Tahoma Bold.ttf"))
pdfmetrics.registerFontFamily("TahomaVN", normal="TahomaVN", bold="TahomaVN-Bold")

NAVY = colors.HexColor("#143052")
BLUE = colors.HexColor("#1559C7")
MID = colors.HexColor("#53657C")
PALE = colors.HexColor("#EAF2FF")
LINE = colors.HexColor("#DCE5F0")
INK = colors.HexColor("#1D2B3C")
PAPER = colors.white

PAGE_W, PAGE_H = landscape(A4)
MARGIN_X = 39
CONTENT_W = PAGE_W - MARGIN_X * 2

styles = {
    "title": ParagraphStyle(
        "title",
        fontName="TahomaVN-Bold",
        fontSize=23,
        leading=31,
        textColor=NAVY,
        spaceAfter=12,
    ),
    "meta": ParagraphStyle(
        "meta",
        fontName="TahomaVN",
        fontSize=9,
        leading=14,
        textColor=MID,
        spaceAfter=15,
    ),
    "h2": ParagraphStyle(
        "h2",
        fontName="TahomaVN-Bold",
        fontSize=15,
        leading=21,
        textColor=NAVY,
        spaceBefore=17,
        spaceAfter=10,
        keepWithNext=True,
    ),
    "h3": ParagraphStyle(
        "h3",
        fontName="TahomaVN-Bold",
        fontSize=11.2,
        leading=16,
        textColor=BLUE,
        spaceBefore=13,
        spaceAfter=7,
        keepWithNext=True,
    ),
    "body": ParagraphStyle(
        "body",
        fontName="TahomaVN",
        fontSize=9.1,
        leading=14.2,
        textColor=INK,
        spaceAfter=7,
    ),
    "list": ParagraphStyle(
        "list",
        fontName="TahomaVN",
        fontSize=9.1,
        leading=14.2,
        leftIndent=19,
        firstLineIndent=-15,
        textColor=INK,
        spaceAfter=5,
    ),
    "caption": ParagraphStyle(
        "caption",
        fontName="TahomaVN",
        fontSize=8,
        leading=12,
        alignment=TA_CENTER,
        textColor=MID,
        spaceBefore=6,
        spaceAfter=14,
    ),
    "tableHead": ParagraphStyle(
        "tableHead",
        fontName="TahomaVN-Bold",
        fontSize=8.1,
        leading=11.5,
        textColor=NAVY,
    ),
    "tableCell": ParagraphStyle(
        "tableCell",
        fontName="TahomaVN",
        fontSize=8,
        leading=11.8,
        textColor=INK,
    ),
    "tocCell": ParagraphStyle(
        "tocCell",
        fontName="TahomaVN",
        fontSize=8.6,
        leading=12,
        textColor=INK,
    ),
    "tocMain": ParagraphStyle(
        "tocMain",
        fontName="TahomaVN-Bold",
        fontSize=8.6,
        leading=12,
        textColor=NAVY,
    ),
    "tocPage": ParagraphStyle(
        "tocPage",
        fontName="TahomaVN-Bold",
        fontSize=8.6,
        leading=12,
        alignment=TA_RIGHT,
        textColor=BLUE,
    ),
}


def inline_markup(value: str) -> str:
    value = html.escape(value.strip())
    value = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", value)
    value = re.sub(r"(?<!\*)\*([^*]+)\*(?!\*)", r"<i>\1</i>", value)
    value = re.sub(r"`(.+?)`", r"<font color='#1559C7'>\1</font>", value)
    return value


def table_widths(headers: list[str]) -> list[float]:
    n = len(headers)
    if headers == ["Cấp", "Tiêu đề", "Trang"]:
        ratios = [0.11, 0.79, 0.10]
    elif n == 4 and headers[0] == "Giai đoạn" and headers[1] == "Gói công việc cần thực hiện":
        ratios = [0.09, 0.30, 0.43, 0.18]
    elif n == 4 and headers[0] == "Giai đoạn":
        ratios = [0.17, 0.42, 0.17, 0.24]
    elif n == 4:
        ratios = [0.19, 0.35, 0.22, 0.24]
    elif n == 5:
        ratios = [0.19, 0.20, 0.18, 0.22, 0.21]
    elif n == 3:
        ratios = [0.23, 0.35, 0.42]
    elif n == 2:
        ratios = [0.28, 0.72]
    else:
        ratios = [1 / n] * n
    return [CONTENT_W * ratio for ratio in ratios]


def make_table(raw_rows: list[list[str]]) -> Table:
    headers = raw_rows[0]
    is_toc = headers == ["Cấp", "Tiêu đề", "Trang"]
    data = []
    for index, row in enumerate(raw_rows):
        if is_toc and index:
            title_style = styles["tocMain"] if row[0] in ("Tiêu đề", "Chính") else styles["tocCell"]
            data.append([
                Paragraph(inline_markup(row[0]), styles["tocCell"]),
                Paragraph(inline_markup(row[1]), title_style),
                Paragraph(inline_markup(row[2]), styles["tocPage"]),
            ])
        else:
            style = styles["tableHead"] if index == 0 else styles["tableCell"]
            data.append([Paragraph(inline_markup(cell), style) for cell in row])
    table = Table(data, colWidths=table_widths(headers), repeatRows=1, hAlign="LEFT")
    if is_toc:
        table.setStyle(
            TableStyle(
                [
                    ("BACKGROUND", (0, 0), (-1, 0), PALE),
                    ("LINEBELOW", (0, 0), (-1, 0), 0.7, LINE),
                    ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                    ("LEFTPADDING", (0, 0), (-1, -1), 6),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 6),
                    ("TOPPADDING", (0, 0), (-1, -1), 3),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 3),
                ]
            )
        )
        return table
    table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), PALE),
                ("GRID", (0, 0), (-1, -1), 0.35, LINE),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 7),
                ("RIGHTPADDING", (0, 0), (-1, -1), 7),
                ("TOPPADDING", (0, 0), (-1, -1), 6),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [PAPER, colors.HexColor("#F8FAFD")]),
            ]
        )
    )
    return table


def make_image(line: str, caption: str | None):
    match = re.match(r"!\[(.+?)\]\((.+?)\)", line)
    if not match:
        return []
    path = ROOT / match.group(2)
    with PILImage.open(path) as source:
        width, height = source.size
    scale = min(CONTENT_W / width, 342 / height, 1.0)
    picture = Image(str(path), width=width * scale, height=height * scale, hAlign="CENTER")
    group = [Spacer(1, 6), picture]
    if caption:
        group.append(Paragraph(inline_markup(caption.strip("*")), styles["caption"]))
    return [KeepTogether(group)]


def page_frame(canvas, doc):
    canvas.saveState()
    canvas.setFillColor(NAVY)
    canvas.rect(0, PAGE_H - 8, PAGE_W, 8, fill=1, stroke=0)
    canvas.setStrokeColor(LINE)
    canvas.line(MARGIN_X, PAGE_H - 32, PAGE_W - MARGIN_X, PAGE_H - 32)
    canvas.line(MARGIN_X, 29, PAGE_W - MARGIN_X, 29)
    canvas.setFont("TahomaVN-Bold", 7.3)
    canvas.setFillColor(MID)
    canvas.drawString(MARGIN_X, PAGE_H - 26, "THỦ THIÊM WORKFLOW  /  LỘ TRÌNH CÔNG TRÌNH")
    canvas.setFont("TahomaVN", 7.3)
    canvas.drawRightString(PAGE_W - MARGIN_X, PAGE_H - 26, "25.09.2026")
    canvas.drawString(MARGIN_X, 18, "Ảnh chụp từ môi trường mẫu · Kiểm tra trạng thái công trình trước khi áp dụng")
    canvas.drawRightString(PAGE_W - MARGIN_X, 18, f"Trang {doc.page}")
    canvas.restoreState()


def build():
    lines = SOURCE.read_text(encoding="utf-8").splitlines()
    story = []
    index = 0
    while index < len(lines):
        line = lines[index].strip()
        if not line:
            index += 1
            continue
        if line == "<!-- page-break -->":
            story.append(PageBreak())
        elif line.startswith("# "):
            story.append(Paragraph(inline_markup(line[2:]), styles["title"]))
            story.append(HRFlowable(width="100%", thickness=2.2, color=BLUE, spaceAfter=14))
        elif line.startswith("## "):
            story.append(Paragraph(inline_markup(line[3:]), styles["h2"]))
        elif line.startswith("### "):
            story.append(Paragraph(inline_markup(line[4:]), styles["h3"]))
        elif line.startswith("|"):
            rows = []
            while index < len(lines) and lines[index].strip().startswith("|"):
                row = [cell.strip() for cell in lines[index].strip().strip("|").split("|")]
                if not all(re.fullmatch(r":?-{3,}:?", cell) for cell in row):
                    rows.append(row)
                index += 1
            if rows:
                story.append(make_table(rows))
                story.append(Spacer(1, 8))
            continue
        elif line.startswith("!["):
            caption = None
            if (
                index + 2 < len(lines)
                and not lines[index + 1].strip()
                and lines[index + 2].strip().startswith("*")
                and lines[index + 2].strip().endswith("*")
                and not lines[index + 2].strip().startswith("**")
            ):
                caption = lines[index + 2].strip()
                index += 2
            story.extend(make_image(line, caption))
        elif re.match(r"^\d+\. ", line) or line.startswith("- "):
            story.append(Paragraph(inline_markup(line), styles["list"]))
        elif line.startswith("*") and line.endswith("*") and not line.startswith("**"):
            story.append(Paragraph(inline_markup(line.strip("*")), styles["caption"]))
        elif line.startswith("**Thủ Thiêm Workflow"):
            story.append(Paragraph(inline_markup(line), styles["meta"]))
        else:
            story.append(Paragraph(inline_markup(line), styles["body"]))
        index += 1

    doc = SimpleDocTemplate(
        str(OUTPUT),
        pagesize=(PAGE_W, PAGE_H),
        leftMargin=MARGIN_X,
        rightMargin=MARGIN_X,
        topMargin=46,
        bottomMargin=43,
        title="Lộ trình một công trình từ đầu đến bàn giao - Thủ Thiêm Workflow",
        author="Thủ Thiêm Workflow",
    )
    doc.build(story, onFirstPage=page_frame, onLaterPages=page_frame)
    print(OUTPUT)


if __name__ == "__main__":
    build()
