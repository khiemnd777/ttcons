#!/usr/bin/env python3
"""Generate synthetic, reviewable PDF fixtures for the Golden Project seed."""

from __future__ import annotations

import hashlib
import json
import shutil
import sys
from pathlib import Path

if "--check" not in sys.argv:
    from reportlab.lib import colors
    from reportlab.lib.enums import TA_CENTER
    from reportlab.lib.pagesizes import A4
    from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
    from reportlab.lib.units import mm
    from reportlab.pdfbase import pdfmetrics
    from reportlab.pdfbase.ttfonts import TTFont
    from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle


ROOT = Path(__file__).resolve().parent
UPLOADS = ROOT / "uploads"
PDF_DIR = UPLOADS / "workflow-form-pdfs" / "golden-project"
FONT = Path("/System/Library/Fonts/Supplemental/Verdana.ttf")
FONT_BOLD = Path("/System/Library/Fonts/Supplemental/Verdana Bold.ttf")

DOCUMENTS = {
    "client_site_survey_minutes": (
        "BIÊN BẢN KHẢO SÁT HIỆN TRẠNG",
        [
            ("Phạm vi", "Tầng 12 - khu làm việc, pantry, sảnh và phòng họp"),
            ("Kích thước", "Diện tích hoàn thiện 1.280 m²; cao độ trần 2.700 mm"),
            ("Hiện trạng", "Mặt bằng đã bàn giao; MEP chờ điều chỉnh theo layout mới"),
            ("Rủi ro", "Thi công ngoài giờ; kiểm soát bụi, tiếng ồn và công việc che khuất"),
            ("Kết luận", "Đủ dữ liệu chuyển sang thiết kế và bóc tách BOQ"),
        ],
    ),
    "client_procurement_contract_advance_package": (
        "HỒ SƠ HỢP ĐỒNG VÀ TẠM ỨNG ĐẦU VÀO",
        [
            ("Nhà cung cấp", "Cadivi, Dulux và các nhà cung cấp đã qualify"),
            ("Thầu phụ", "Thầu phụ điện An Tâm - gói MEP"),
            ("Nguồn giá", "Báo giá được chọn, BOQ final lock và PO-GOLD-001/002"),
            ("Tạm ứng", "20% cho thầu phụ; có lịch thanh toán và quyết toán"),
            ("Phê duyệt", "QS, PM, Kế toán và Giám đốc đã kiểm tra"),
        ],
    ),
    "client_owner_contract_package": (
        "HỢP ĐỒNG CHỦ ĐẦU TƯ - REV.02",
        [
            ("Số hợp đồng", "HD-GOLD-CDT-001"),
            ("Giá trị", "4.680.000.000 VND"),
            ("Phạm vi", "MEP, trần, sơn, sàn, pantry và đèn trang trí"),
            ("Thanh toán", "30% tạm ứng - 45% tiến độ - 25% quyết toán"),
            ("Bảo hành", "365 ngày; giữ lại theo milestone và giải phóng có lịch sử"),
        ],
    ),
    "builtin_meeting_minutes": (
        "BIÊN BẢN HỌP PHÂN CÔNG THI CÔNG",
        [
            ("Chủ trì", "Nguyễn Minh PM"),
            ("Thành phần", "PM, QS, Thiết kế, QA/QC, Giám sát và thầu phụ"),
            ("Quyết định", "Thi công theo ITP Rev.02; hold point trước đóng trần"),
            ("An toàn", "Toolbox meeting hằng ngày; kiểm soát ngoài giờ"),
            ("Báo cáo", "Nhật ký ngày, RFI theo lô và QA/QC hằng tuần"),
        ],
    ),
    "builtin_subcontractor_checklist": (
        "ĐỐI CHIẾU KHỐI LƯỢNG THẦU PHỤ",
        [
            ("Gói công việc", "MEP khu A và B"),
            ("Khối lượng BOQ", "2.400 m dây điện"),
            ("Khối lượng xác nhận", "2.388 m"),
            ("Chênh lệch", "-12 m theo đo hoàn công"),
            ("Kết luận", "Hai bên chấp thuận điều chỉnh và quyết toán"),
        ],
    ),
    "builtin_owner_client_checklist": (
        "BIÊN BẢN NGHIỆM THU VÀ XÁC NHẬN CHỦ ĐẦU TƯ",
        [
            ("Nghiệm thu", "OA-GOLD-FINAL - kết quả PASS"),
            ("MEP", "Test vận hành và bản vẽ hoàn công đạt"),
            ("Hoàn thiện", "Trần, sơn, sàn đạt; NCR-GOLD-001 đã đóng"),
            ("Punch list", "1 hạng mục đã xử lý và tái kiểm"),
            ("Kết luận", "Đủ điều kiện chuyển bàn giao"),
        ],
    ),
    "client_output_settlement_package": (
        "HỒ SƠ THANH TOÁN VÀ QUYẾT TOÁN",
        [
            ("Hợp đồng active", "HD-GOLD-CDT-001 Rev.02 - 4.680.000.000 VND"),
            ("Tạm ứng", "1.404.000.000 VND - đã thu và đối trừ"),
            ("Tiến độ", "2.000.700.000 VND - đã thu"),
            ("Quyết toán", "1.111.500.000 VND - đã thu"),
            ("Giữ lại", "163.800.000 VND - đã giải phóng và thu đủ"),
        ],
    ),
    "client_output_invoice_closure": (
        "BIÊN BẢN ĐÓNG TÀI CHÍNH VÀ BÀN GIAO",
        [
            ("Công nợ đầu vào", "AP remaining = 0"),
            ("Công nợ đầu ra", "AR remaining = 0"),
            ("Thanh toán chờ duyệt", "0"),
            ("Kho và vật tư dư", "Đã hoàn trả, site balance = 0"),
            ("Bàn giao", "HO-GOLD-001 - 8/8 checklist verified"),
        ],
    ),
}


def register_fonts() -> None:
    pdfmetrics.registerFont(TTFont("GoldenSans", str(FONT)))
    pdfmetrics.registerFont(TTFont("GoldenSansBold", str(FONT_BOLD)))


def build_pdf(path: Path, title: str, rows: list[tuple[str, str]]) -> None:
    styles = getSampleStyleSheet()
    title_style = ParagraphStyle(
        "GoldenTitle",
        parent=styles["Title"],
        fontName="GoldenSansBold",
        fontSize=16,
        leading=21,
        alignment=TA_CENTER,
        textColor=colors.HexColor("#17324D"),
        spaceAfter=8 * mm,
    )
    body_style = ParagraphStyle(
        "GoldenBody",
        parent=styles["BodyText"],
        fontName="GoldenSans",
        fontSize=9,
        leading=13,
        textColor=colors.HexColor("#243447"),
    )
    label_style = ParagraphStyle(
        "GoldenLabel",
        parent=body_style,
        fontName="GoldenSansBold",
        textColor=colors.HexColor("#17324D"),
    )
    doc = SimpleDocTemplate(
        str(path),
        pagesize=A4,
        leftMargin=18 * mm,
        rightMargin=18 * mm,
        topMargin=18 * mm,
        bottomMargin=18 * mm,
        title=title,
        author="Consflow Golden Sample",
    )
    story = [
        Paragraph("CONSFLOW - GOLDEN PROJECT", label_style),
        Spacer(1, 3 * mm),
        Paragraph(title, title_style),
        Paragraph(
            "Công trình: Golden Project - Hoàn thiện Văn phòng An Phát<br/>"
            "Mã: GOLDEN-REAL-001 | Template: 40000000-0000-0000-0000-000000000020",
            body_style,
        ),
        Spacer(1, 7 * mm),
    ]
    table_data = [[Paragraph("Hạng mục", label_style), Paragraph("Nội dung xác nhận", label_style)]]
    table_data.extend([[Paragraph(label, label_style), Paragraph(value, body_style)] for label, value in rows])
    table = Table(table_data, colWidths=[48 * mm, 112 * mm], repeatRows=1)
    table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#E9F1F7")),
                ("GRID", (0, 0), (-1, -1), 0.5, colors.HexColor("#A9BBC8")),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 7),
                ("RIGHTPADDING", (0, 0), (-1, -1), 7),
                ("TOPPADDING", (0, 0), (-1, -1), 7),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
            ]
        )
    )
    story.extend(
        [
            table,
            Spacer(1, 12 * mm),
            Paragraph(
                "Xác nhận: Hồ sơ synthetic phục vụ review local. Các mã, số tiền và liên kết "
                "khớp với Golden Project seed; không chứa dữ liệu khách hàng thật.",
                body_style,
            ),
        ]
    )
    doc.build(story)


def main() -> None:
    if "--check" in sys.argv:
        manifest_path = ROOT / "golden_fixture_manifest.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        for code, expected in manifest.items():
            path = PDF_DIR / f"{code}.pdf"
            raw = path.read_bytes()
            actual = {"size": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}
            if actual != expected:
                raise SystemExit(f"Golden fixture mismatch for {code}: {actual} != {expected}")
        print(f"GOLDEN_FIXTURES_VALIDATION_PASSED ({len(manifest)} PDFs)")
        return

    register_fonts()
    PDF_DIR.mkdir(parents=True, exist_ok=True)
    manifest: dict[str, dict[str, int | str]] = {}
    for code, (title, rows) in DOCUMENTS.items():
        path = PDF_DIR / f"{code}.pdf"
        build_pdf(path, title, rows)
        raw = path.read_bytes()
        manifest[code] = {"size": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}

    input_source = UPLOADS / "input-invoices" / "30000000-0000-0000-0000-000000000010" / "81000000-0000-0000-0000-000000000010" / "demo-input-invoice.pdf"
    output_source = UPLOADS / "output-invoices" / "30000000-0000-0000-0000-000000000010" / "82000000-0000-0000-0000-000000000010" / "demo-output-invoice.pdf"
    input_target = UPLOADS / "input-invoices" / "golden-project" / "demo-input-invoice.pdf"
    output_target = UPLOADS / "output-invoices" / "golden-project" / "demo-output-invoice.pdf"
    input_target.parent.mkdir(parents=True, exist_ok=True)
    output_target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(input_source, input_target)
    shutil.copyfile(output_source, output_target)

    (ROOT / "golden_fixture_manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


if __name__ == "__main__":
    main()
