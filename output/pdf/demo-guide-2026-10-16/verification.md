# Kiểm tra guideline quy trình bổ sung BMGS - 16-10-2026

**Trạng thái: TECHNICALLY VERIFIED cho tài liệu; chờ người dùng review nội dung/bố cục.** Không chạy lại hoặc mở rộng nghiệm thu ứng dụng. Ảnh được tái sử dụng từ các phạm vi Development đã chốt; ảnh không chứng minh Production, mọi vai trò hoặc mọi nhánh công trình.

## Phạm vi đã thực hiện

1. Bổ sung bảng tra cứu tám mẫu, đường dẫn/nút xuất, điều kiện lưu/quyền, phạm vi bộ lọc và lần gửi, nội dung cần đối chiếu tại các chặng quy trình tương ứng. Giữ toàn bộ nội dung cũ; đổi số các mục BOQ/Hợp đồng/Mua sắm/Lấy vật tư để dành mục cho Khảo sát/Rủi ro/Minh chứng.
2. Giữ 69 ảnh tham chiếu cũ, thêm 26 ảnh đã nghiệm thu; ghi rõ công trình thử và môi trường, phân biệt Rev9/Rev10. File `image-sources.json` lưu nguồn và SHA256; không chỉnh ảnh gốc, workbook hoặc Google Sheets.
3. Tạo bản Markdown/PDF mới. Script kết xuất sao chép từ bản cũ, chỉ cập nhật ngày, phân trang/bảng hướng dẫn mới và tự tính mục lục/liên kết. Giữ nguyên thư mục `output/pdf/demo-guide-2026-09-25/`.

## Kết quả kiểm tra bản cuối

- Nền mã nguồn: `eff150e`; working tree chỉ bổ sung bộ tài liệu mới. Không sửa frontend/backend, config, field, API, schema, dữ liệu hoặc lịch sử.
- PDF kết xuất lúc 09:22:22 ICT ngày 16-10-2026; 129 trang A4 ngang, 17.053 MiB. Số trang ổn định sau hai lượt kết xuất.
- `verify_guide.py`: **715/715 kiểm tra đạt**, gồm 31 mục lục trùng tiêu đề/trang thực tế, 62 liên kết nội bộ hợp lệ, 95/95 tham chiếu ảnh có file và được nhúng, 26/26 ảnh nghiệm thu khớp byte nguồn, font tiếng Việt, số trang/footer và giới hạn hình/chữ.
- Render toàn bộ 129 trang bằng `pdftoppm`; xem chín ảnh tổng quan bao phủ toàn bộ trang và xem kỹ các trang bảng tra cứu, vị trí nút, dữ liệu/lần gửi, ảnh và bàn giao. Không phát hiện ảnh bị kéo giãn, chữ tràn hoặc bảng bị cắt ở các vùng đã xem.
- Đối chiếu 110 file trong thư mục cũ với baseline trước thao tác: **0 file thay đổi/mất**. Nội dung thân bài cũ không mất dòng; bốn tiêu đề mục được đánh số lại trong bản mới.
- SHA256 Markdown: `5efcfc6b43c6bef600ec303557a8b89098820de5c6303e20ba1581f3bdc4f377`.
- SHA256 PDF: `eefc1c9e514589bece8e2063e202f05bd0c3d93c9ba9e279057a0864fc291d53`.
- `verification.json` lưu kết quả kiểm tự động; `build-metadata.json` lưu vị trí tiêu đề thực tế.

## Trang để review

| Nội dung | Trang PDF |
|---|---|
| Bảng tra cứu tám mẫu | 5 |
| BM01 - khảo sát/đo hiện trạng | 51 |
| BM05 - rủi ro | 54 |
| BM07 - sổ minh chứng và ảnh trong ô | 57 |
| BM02 - các điểm xuất báo cáo ngày | 98 |
| BM02 - đúng lần gửi cũ và ảnh | 104 |
| BM04 - danh sách/chi tiết Cần xử lý | 107 |
| BM06 - kế hoạch/chỉ dẫn hiện trường | 111 |
| BM03 - gói nghiệm thu thầu phụ | 117 |
| BM08 - hồ sơ bàn giao | 123 |

## Tạo lại và kiểm tra

Dùng Python có `reportlab`, `Pillow`, `pypdf`, `pdfplumber`; máy kết xuất này dùng bộ Python đi kèm Codex. Script dùng font Tahoma của macOS như bản gốc. Từ repository:

```sh
python3 output/pdf/demo-guide-2026-10-16/build_pdf.py
python3 output/pdf/demo-guide-2026-10-16/verify_guide.py
pdftoppm -scale-to 1000 -jpeg output/pdf/demo-guide-2026-10-16/huong-dan-demo-cong-trinh.pdf /tmp/bmgs-guide-review
```

**Không chạy trong lượt này:** trình duyệt, UAT ứng dụng, Google Sheets, Excel desktop, in giấy, build/test ứng dụng, Docker, database/migration, deployment hoặc Git delivery. Các kết quả ứng dụng trước đây giữ đúng phạm vi tại `docs/bm-gs-acceptance-summary-2026-10-13.md`.

**Mapping:** đã dùng central/frontend mapping và đối chiếu các owner hiện có (WorkflowFormPanel, Operations, WorkReports, EvidencePanel, HandoverPage và native export services). Không cập nhật mapping vì chỉ thêm tài liệu hướng dẫn, không đổi hành vi, quyền, contract hoặc chủ sở hữu tính năng.
