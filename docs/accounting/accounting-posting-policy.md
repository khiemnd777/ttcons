# Chính sách hạch toán Kế toán công trình

## Phạm vi

- Chế độ kế toán định hướng theo Thông tư 200.
- Đồng tiền hạch toán và lập báo cáo: VND.
- Công nợ, hóa đơn, phiếu thu, phiếu chi và tạm ứng là chứng từ gốc.
- Sổ Nhật ký chung và Sổ Cái là nguồn số liệu kế toán chính thức sau khi chứng từ được ghi sổ.
- Dòng tiền tiếp tục căn cứ vào phiếu thu/phiếu chi đã duyệt; doanh thu, chi phí và thuế căn cứ vào bút toán đã ghi sổ.

## Nguyên tắc kiểm soát

1. Mỗi sự kiện chứng từ chỉ được ghi sổ một lần.
2. Tổng phát sinh Nợ phải bằng tổng phát sinh Có trước khi ghi sổ.
3. Chứng từ đã ghi sổ không được sửa hoặc xóa; sai sót được xử lý bằng bút toán đảo hoặc bút toán điều chỉnh.
4. Không được ghi sổ vào kỳ kế toán đã khóa.
5. Công trình, đối tượng, chứng từ nguồn và bút toán phải cùng phạm vi sổ kế toán.
6. Nhật ký kiểm soát phải lưu người lập, người ghi sổ, người đảo, thời điểm và lý do.

## Ma trận định khoản chuẩn

| Sự kiện nghiệp vụ | Nợ | Có | Tác động ngân sách |
|---|---|---|---|
| Duyệt khoản phải trả | Không ghi Sổ Cái | Không ghi Sổ Cái | Tăng giá trị đã cam kết |
| Xác minh hóa đơn đầu vào | 154/621/622/623/627/642 và 1331 | 331 | Giảm cam kết, tăng chi phí thực tế |
| Thanh toán khoản phải trả | 331 | 1111/1121 | Tăng số đã thanh toán, không tăng chi phí lần hai |
| Chi phí không qua công nợ | 154/621/622/623/627/642 | 1111/1121 | Tăng chi phí thực tế và đã thanh toán |
| Chi ứng trước nhà cung cấp | 331 (dư Nợ) | 1111/1121 | Tăng số đã thanh toán, chưa tăng chi phí thực tế |
| Phát hành hóa đơn đầu ra | 131 | 511 và 3331 | Không tác động ngân sách chi |
| Thu khoản phải thu | 1111/1121 | 131 | Không tác động ngân sách chi |
| Nhận ứng trước khách hàng | 1111/1121 | 131 (dư Có) | Không tác động ngân sách chi |
| Quyết toán tạm ứng | Theo chứng từ quyết toán | Theo tài khoản tạm ứng/công nợ | Chuyển sang chi phí thực tế khi đủ chứng từ |
| Sửa sai | Bút toán ngược với bút toán gốc | Bút toán ngược với bút toán gốc | Sinh biến động đảo tương ứng |

## Ngân sách

- `Ngân sách được duyệt`: hạn mức đã được phê duyệt.
- `Giá trị đã cam kết`: nghĩa vụ chi đã duyệt nhưng chưa đủ điều kiện ghi nhận chi phí thực tế.
- `Chi phí thực tế`: chi phí đã có căn cứ hạch toán.
- `Đã thanh toán`: dòng tiền đã chi.
- `Ngân sách còn khả dụng = Ngân sách được duyệt - Giá trị đã cam kết - Chi phí thực tế`.
- Các chỉ tiêu được tổng hợp từ sổ biến động ngân sách; trường tổng hợp tương thích không phải nguồn dữ liệu gốc.

## Thuế GTGT

- Thuế GTGT đầu vào chỉ ghi nhận khi hóa đơn đầu vào được xác minh.
- Thuế GTGT đầu ra ghi nhận khi hóa đơn đầu ra được phát hành.
- Phần thuế không được khấu trừ được hạch toán vào chi phí hoặc giá trị tài sản/công trình theo tài khoản chi phí của chứng từ.
- Sổ chi tiết thuế phải đối chiếu với tài khoản 1331 và 3331 theo từng kỳ thuế.
- Hóa đơn bị hủy hoặc điều chỉnh sau khi ghi sổ phải có bút toán đảo/điều chỉnh; không xóa lịch sử thuế.

## Chuyển đổi dữ liệu

- Backfill các chứng từ thuộc năm tài chính hiện tại bằng khóa chống ghi trùng theo chứng từ nguồn và sự kiện.
- Dữ liệu trước năm tài chính hiện tại được đưa vào số dư đầu kỳ sau khi đối chiếu.
- Báo cáo cũ và báo cáo từ Sổ Cái chạy song song cho đến khi công nợ, ngân sách, thuế và cân đối số phát sinh khớp.
