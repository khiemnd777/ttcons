# Nhập kết quả MISA để đối soát hồ sơ Consflow

Mục đích: lưu bản dữ liệu từ MISA có nguồn gốc và phân bổ kết quả thu/chi vào đề nghị công trình. Không tạo bút toán, phiếu thu/chi hoặc công nợ chính thức trong Consflow.

## Đề nghị theo đợt

Mở **Hồ sơ bàn giao kế toán → Đề nghị theo đợt & kết quả MISA**. Chi tiết bàn giao và Bàn làm việc cũng mở cùng dialog. Kế toán xác nhận căn cứ bằng nguồn có đối tác, giá trị và chứng từ. Giá trị dự kiến của yêu cầu mua không tự trở thành căn cứ đã xác nhận.

Nguồn hỗ trợ: yêu cầu mua đã đặt hàng, mốc thu hợp đồng khách hàng và mốc chi hợp đồng thầu phụ. Mỗi nguồn có một căn cứ xác nhận bất biến. Người lập chọn căn cứ, tên đợt, loại đề nghị, hạn, giá trị đợt, thu hồi tạm ứng, giữ lại và chứng từ; lưu nháp rồi gửi Kế toán. Tiếp nhận chỉ xác nhận hồ sơ đã được nhận. Hồ sơ trả bổ sung có thể sửa và gửi lại; mỗi lần giữ một phiên bản, không sửa lịch sử.

Thu hồi phải chọn đợt tạm ứng đã tiếp nhận của cùng căn cứ; giải phóng giữ lại phải chọn đợt có giữ lại đã tiếp nhận. Máy chủ kiểm tra tổng các đợt cùng tham chiếu để tránh thu hồi hoặc giải phóng vượt giá trị. Đây là số liệu đề nghị, chưa chứng minh tạm ứng đã được trả bằng tiền.

Phiếu xuất kho đã ghi nhận của công trình được xem/liên kết riêng, không tự tạo đề nghị chi, định giá kho hoặc công nợ. Tệp bổ sung được tải bằng uploader chuẩn vào Đầu việc do người dùng chọn rõ ràng; quyền/trạng thái Đầu việc vẫn do API nguồn kiểm tra. Nếu tải tệp thành công nhưng lưu đề nghị thất bại, tệp đã tồn tại ở Đầu việc nguồn và lần thử lại dùng receipt đã tải.

## Mẫu CSV

Trong tab **Kết quả MISA**, chọn **Nhập dữ liệu CSV → Tải mẫu CSV**. Đây là mẫu Consflow, không phải bộ đọc mọi định dạng xuất sẵn của MISA. Mỗi file thuộc công trình đang mở và một mã đơn vị kế toán; phải chuyển đổi/ánh xạ dữ liệu xuất từ MISA sang mẫu trước khi nhập.

```csv
external_id,line_key,kind,direction,partner_id,currency,amount,record_date,replaces_id
```

| Cột | Nội dung |
|---|---|
| external_id | Mã định danh ổn định của chứng từ hoặc dòng số dư tại nguồn; không chỉ dùng tên đối tác |
| line_key | Mã dòng ổn định trong chứng từ; phân biệt các dòng cùng external_id |
| kind | `transaction`: giao dịch thu/chi; `balance`: số dư công nợ tại ngày chốt |
| direction | `inbound`: thu/phải thu; `outbound`: chi/phải trả |
| partner_id | UUID đối tác Consflow đã ánh xạ đúng từ MISA; không phải tên đối tác |
| currency | Mã tiền tệ ISO viết hoa, phải khớp đề nghị khi phân bổ |
| amount | Số không dấu phân cách hàng nghìn, dấu chấm thập phân; VND nguyên, tiền tệ khác theo độ chính xác ISO trong giới hạn 3 chữ số thập phân của dữ liệu nhập; giao dịch dương, số dư có thể bằng 0 |
| record_date | Ngày giao dịch hoặc ngày chốt số dư, `YYYY-MM-DD` |
| replaces_id | Để trống khi nhập mới; khi điều chỉnh, dùng ID dòng Consflow hiện hành hiển thị tại “Mã dòng để đối chiếu/thay thế” |

Giới hạn: 2 MB, 2000 dòng/đợt. **Kiểm tra file** chỉ lưu đợt chờ xác nhận; **Xác nhận nhập** mới đưa dòng vào kết quả. File gốc được giữ nguyên cùng digest, người nhập và thời gian, tải qua endpoint có kiểm tra quyền công trình. Lịch sử hiển thị cả đợt chờ xác nhận và đã nhập. Có thể chọn lại cùng file và đơn vị để tiếp tục đợt chờ xác nhận; file đã nhập không tạo đợt trùng.

Danh tính dòng gồm công trình, đơn vị kế toán, external_id, line_key và kind. Gửi lại dòng đã tồn tại bằng một file khác phải chỉ rõ phiên bản thay thế. Dòng cũ được giữ lại. Dòng có phân bổ đang hiệu lực không được thay thế trước khi thu hồi các phân bổ và ghi lý do.

## Phân bổ và xác nhận đối soát

Chọn đề nghị đã tiếp nhận. Danh sách giao dịch chỉ gồm dòng hiện hành cùng đối tác, hướng và tiền tệ. Nhập số tiền phân bổ, lưu, rồi xác nhận đối soát kèm ghi chú. Một giao dịch có thể chia cho nhiều đề nghị; một đề nghị có thể nhận nhiều giao dịch. Máy chủ khóa giao dịch xử lý và kiểm tra cả hai giới hạn; retry cùng mã thao tác không tạo phân bổ lặp.

Số dư tại ngày chốt được trình bày riêng, không đem phân bổ như tiền thu/chi và không cộng với giao dịch. Phần chưa được bao phủ chỉ so với giao dịch đã nhập; không khẳng định chưa thanh toán trên MISA. Có thể xác nhận đã kiểm tra một phần, nhưng màn hình vẫn ghi còn chênh lệch. Khi phân bổ đổi hoặc bị thu hồi, xác nhận đối soát cũ không còn là xác nhận cho trạng thái hiện tại.

Không thay đổi điều kiện kết thúc quy trình, không tự đánh dấu mốc hợp đồng đã thanh toán, không thay đổi dữ liệu MISA. Chưa có tích hợp API MISA hoặc kiểm chứng bằng file MISA thực tế.
