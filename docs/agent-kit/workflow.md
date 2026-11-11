# Planning và bàn giao/nghiệm thu

Tài liệu này quy định phần workflow Agent Kit được áp dụng có chọn lọc cho Consflow. Chỉ áp dụng planning, bảng WHAT, bảng Skills đã sử dụng và biên bản bàn giao/nghiệm thu. [AGENTS.md](../../AGENTS.md) tiếp tục quy định nghiệp vụ, mapping, quyền, review và kiểm chứng của Consflow.

## Phạm vi và nguồn

- Nguồn được ghim: Agent Kit commit `8161bfe28aacfee1915c50bca0f9274add0a1899`: [operating core](https://github.com/khiemnd777/agent-kit/blob/8161bfe28aacfee1915c50bca0f9274add0a1899/AGENTS.md), [handoff templates](https://github.com/khiemnd777/agent-kit/blob/8161bfe28aacfee1915c50bca0f9274add0a1899/docs/handoff-templates.md), [MIT license](https://github.com/khiemnd777/agent-kit/blob/8161bfe28aacfee1915c50bca0f9274add0a1899/LICENSE).
- Đây là bản điều chỉnh cho Consflow, không phải cài đặt toàn bộ Kit. Không nhập skills/agents Kit, Claude adapters, lịch sử task, approval hoặc nhật ký Skills riêng. Không đổi model, permissions, thứ tự viết/chạy tests hoặc quy tắc nghiệp vụ hiện có.
- [docs/current-states.md](../current-states.md) là nguồn trạng thái hiện hành duy nhất. Tài liệu này chứa quy tắc và mẫu, không chứa tiến độ task hoặc quyết định nghiệm thu thực tế. Giữ quy trình đóng plan hiện có; không tự đóng plan khi bàn giao.
- Dùng tiếng Việt trong chat, kế hoạch, bảng và biên bản theo yêu cầu hiện hành của Khiêm; chỉ đổi sang tiếng Anh khi Khiêm yêu cầu. Giữ identifiers và thuật ngữ đã được duyệt.
- Người phụ trách phiên chính tích hợp kết quả và cập nhật trạng thái trong phạm vi được phép. Một tên vai trò hoặc mẫu biểu không yêu cầu tạo thêm agent.

## Planning

Mọi task mới đều cần planning và xác nhận rõ trước triển khai, kể cả thay đổi nhỏ. Độ dài kế hoạch theo phạm vi/rủi ro; task nhỏ có thể dùng kế hoạch ngắn trong chat, không bắt buộc tạo file task.

Ở đầu kế hoạch, ghi:

```text
Kiểu bàn giao: Một lần bàn giao / Roadmap
Lý do: [Có thể nghiệm thu cùng nhau hoặc cần các vòng nghiệm thu riêng]
Roadmap cha: [Tên/ID và gói hiện tại, hoặc Không có]
```

Chọn kiểu theo thứ tự:

1. Nếu task thuộc roadmap đã thống nhất, giữ roadmap và các điểm dừng của roadmap, kể cả khi gói hiện tại nhỏ.
2. Nếu các kết quả cần nghiệm thu riêng hoặc quyết định ở gói trước ảnh hưởng gói sau, dùng roadmap. Xác định trước ID/tên/thứ tự/phụ thuộc và tiêu chí nghiệm thu của các gói có ý nghĩa.
3. Nếu kết quả có phạm vi rõ và có thể đánh giá đầy đủ cùng lúc, dùng một lần bàn giao. Không chia gói theo từng file hoặc lớp UI/API/database chỉ để đủ mẫu.

Số file, số dòng và thời lượng không quyết định kiểu bàn giao. Rủi ro quyết định độ sâu kiểm tra riêng; một thay đổi quyền nhỏ vẫn cần kiểm tra bảo mật phù hợp.

Kế hoạch trình trực tiếp trong chat phải có:

- Mục tiêu, phạm vi, kết quả mong đợi và phần ngoài phạm vi khi cần làm rõ quyền.
- Hiện trạng và bằng chứng; tiêu chí nghiệm thu quan sát được.
- Cách thực hiện, files/thành phần liên quan, rủi ro, câu hỏi chưa giải quyết và cách kiểm chứng.
- Review package theo Consflow; bugfix cần bằng chứng/nguyên nhân hoặc giới hạn chẩn đoán; UI cần Mockup as Text và xác nhận trước sửa.
- Bảng WHAT cho công việc dự kiến; bảng Skills chỉ phản ánh đóng góp thực tế trong planning. Skills dự kiến dùng sau này không được báo là đã dùng.
- Các điểm cần xác nhận đánh số riêng `1.`, `2.`, `3.`; lựa chọn bước tiếp theo kèm kết quả và quyền cần thiết.

Chờ xác nhận kế hoạch trước triển khai. Yêu cầu sửa lỗi hoặc câu “tiếp tục triển khai” không thay thế Human Review Gates. Khi gói đã được duyệt, tiếp tục phần việc, kiểm tra và sửa chữa đúng phạm vi mà không xin lại theo từng file. Thay đổi đáng kể về phạm vi, giải pháp, quyền hoặc rủi ro phải trình điều chỉnh và chờ duyệt lại. Approval hoặc trạng thái lịch sử không cấp quyền cho task khác.

## Bảng WHAT

Luôn hiển thị trực tiếp trong chat khi trình kế hoạch và báo cáo kết quả, kể cả task nhỏ. Link tới bảng trong file không thay thế bảng trong chat. Cập nhật tiến độ giữa chừng không cần lặp bảng.

| Tên | Mục đích | Vị trí | Trạng thái | Lý do chưa làm/bỏ qua |
| --- | --- | --- | --- | --- |
| [Tên hạng mục ổn định] | [Kết quả cần tạo] | [File/module/tài liệu] | [Trạng thái thực tế] | [Lý do hoặc —] |

Trong planning, ghi đúng trạng thái đề xuất/chờ duyệt/đã duyệt. Khi bàn giao, ghi kết quả thực tế và giải thích mọi mục chưa làm, bị chặn, hoãn hoặc bỏ qua. Giữ tên hạng mục ổn định trong task. Một dòng “đã làm” không tự chứng minh kiểm chứng hành vi, nghiệm thu hoặc phát hành.

## Bảng Skills đã sử dụng

Cuối planning và implementation, hiển thị:

| Skill | Nguồn | Đóng góp thực tế | Tỷ lệ đóng góp ước lượng |
| --- | --- | --- | --- |
| [Tên skill thực sự dùng] | [Repo / Personal / UNKNOWN] | [Kỹ thuật/kết quả cụ thể đã đóng góp] | [Ước lượng %] |

- Repo: skill thực sự dùng từ repo hiện tại. Personal: nguồn ngoài repo từ cá nhân/host/plugin. UNKNOWN: chưa xác minh được nguồn thực sự dùng. Nguồn mô tả vị trí áp dụng, không mô tả tác giả upstream.
- Chọn ít skills đủ dùng, đọc trước khi áp dụng. Đọc hoặc chọn một skill không tự tạo đóng góp; không nhập skills Kit chỉ để điền bảng.
- Chỉ ghi đóng góp thực tế của pha đang báo cáo. Tổng tỷ lệ là 100% trong các skills đã dùng; tỷ lệ do agent ước lượng, không phải số đo thời gian, token, chất lượng hoặc hiệu quả.
- Nếu không dùng skill, nói rõ “Không dùng skill trong pha này”; không tạo dòng giả để đạt 100%.
- Không tạo nhật ký Skills, telemetry hoặc tracker riêng. Đưa bằng chứng cần thiết vào biên bản hoặc mục task trong nguồn trạng thái hiện có; không tạo bản sao trạng thái.

## Bằng chứng và nguyên tắc bàn giao

Cả hai kiểu bàn giao đều trình trực tiếp trong chat. Liên kết tới bằng chứng chi tiết khi cần, không chép toàn bộ logs.

- Hiển thị ảnh kết quả thực tế hoặc sơ đồ kỹ thuật dựa trên nguồn của phần đã bàn giao, kể cả task tài liệu. Chú thích revision/working-tree snapshot, nguồn, trạng thái bằng chứng và giới hạn. Sơ đồ không phải bằng chứng runtime.
- Verified: có nguồn/quan sát phù hợp hỗ trợ. Proposed: đề xuất. Unknown: chưa xác minh. Nếu thiếu visual cần thiết, ghi UNAVAILABLE và lý do; không dùng mockup/ảnh AI làm bằng chứng kết quả thật. Giữ yêu cầu xin phép browser của Consflow.
- Checklist thường có 3–5 điểm cụ thể, điều chỉnh theo rủi ro; mỗi điểm ghi quan sát/thao tác và kết quả mong đợi. Không dùng ảnh để suy ra quyền, transaction hoặc tính toàn vẹn dữ liệu chưa kiểm tra.
- Ghi source revision hoặc trạng thái working tree, môi trường, lệnh/kịch bản, kết quả và giới hạn. Ghi build timing, database/migration state khi có liên quan; nếu không áp dụng, nói rõ.
- Tách PASS, FAIL và NOT_RUN. Phân biệt IMPLEMENTED, TECHNICALLY VERIFIED, UX ACCEPTED và phát hành. Kiểm chứng kỹ thuật không thay thế người dùng nghiệm thu; checks bắt buộc chưa chạy vẫn còn mở.
- Nêu blocker, checks bị bỏ qua và giới hạn trước kết quả thành công. Sửa chữa thuộc gói hiện tại tiếp tục trong quyền đã duyệt; yêu cầu nghiệp vụ mới quay lại planning.
- Kết thúc bằng 1–3 lựa chọn cụ thể, nêu kết quả mong đợi và điều kiện/quyền cần thiết. Lựa chọn không tự cấp quyền thực hiện.

## Mẫu một lần bàn giao

Dùng cho task độc lập có thể nghiệm thu cùng nhau. Các trường trong ngoặc vuông là placeholders, không phải bằng chứng hoặc quyết định thật.

```text
[Task ID/tên — revision hoặc working-tree snapshot]

Kết quả và phạm vi: [Đã thực hiện gì, thành phần liên quan]
Giới hạn/blocker: [FAIL, NOT_RUN, phần chưa xác minh và tác động]

Ảnh kết quả/sơ đồ kỹ thuật: [Hiển thị trực tiếp trong chat]
Chú thích: [Nguồn, revision, Verified/Proposed/Unknown và giới hạn]

Checklist nghiệm thu:
- [ ] [Quan sát/thao tác] → [Kết quả mong đợi]
- [ ] [Ngoại lệ quan trọng] → [Kết quả mong đợi]
- [ ] [Tích hợp/phạm vi cần giữ] → [Kết quả mong đợi]

Bằng chứng kỹ thuật: [Bảng môi trường/lệnh/kịch bản, kết quả và nguồn]
WHAT: [Bảng WHAT với kết quả thực tế]
Skills: [Bảng skills thực sự dùng hoặc nói rõ không dùng]

Nghiệm thu: [Chờ người dùng / người dùng chấp nhận / yêu cầu sửa / dừng]
Bước tiếp theo: [1–3 lựa chọn, kết quả và quyền cần thiết]
```

Agent chỉ ghi “người dùng chấp nhận” khi có xác nhận thật và chỉ cho phạm vi người dùng đã xác nhận. Bàn giao không tự cho phép commit/push, deploy hoặc bắt đầu task khác.

## Mẫu bàn giao theo roadmap

Dùng toàn bộ nội dung mẫu một lần bàn giao, cộng khối sau ở đầu. Thay ID task bằng ID/tên gói và revision. Tên/thứ tự gói phải lấy từ roadmap đã thống nhất, không tự tạo gói kế tiếp sau khi hoàn tất gói hiện tại.

```text
Roadmap: [Tên/ID — phiên bản kế hoạch]
Đã nghiệm thu:      [Tên các gói người dùng đã chấp nhận]
Chờ nghiệm thu:     [Tên gói hiện tại thực sự đang chờ]
Tiếp theo:         [Tên gói kế tiếp trong roadmap đã thống nhất]
Các gói sau:       [Các tên còn lại theo thứ tự]

[Biên bản đầy đủ như mẫu một lần bàn giao]

Điểm dừng: Chờ người dùng nghiệm thu gói hiện tại và cho phép mở gói tiếp theo.
```

Nếu gói còn đang làm hoặc bị chặn, ghi đúng trạng thái thay vì đưa vào “Chờ nghiệm thu”. Không suy phần trăm hoàn thành từ số lượng gói. Một chỉ dẫn rõ có thể vừa nghiệm thu vừa cho phép mở gói kế tiếp; im lặng hoặc duyệt roadmap tổng thể không thay thế điểm dừng này.

## Ghi nhận nguồn và giấy phép

Phần planning và các mẫu bàn giao ở đây được dịch/điều chỉnh từ hai tài liệu Agent Kit đã ghim ở đầu trang. Ngoại lệ Consflow: dùng nguồn trạng thái hiện có; không nhập nhật ký Skills, runtime adapters hoặc các quy tắc Kit ngoài phạm vi đã duyệt. Giữ thông báo MIT của nguồn dưới đây; thông báo này áp dụng cho phần được điều chỉnh từ Agent Kit, không thay đổi giấy phép của toàn repo Consflow.

```text
MIT License

Copyright (c) 2026 agent-kit contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
