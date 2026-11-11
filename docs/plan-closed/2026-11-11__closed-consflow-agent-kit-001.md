# [Closed] CONSFLOW-AGENT-KIT-001 — workflow thu hẹp, duyệt 2026-11-11

**Mục tiêu/phạm vi:** Chỉ áp dụng planning, bảng WHAT, bảng Skills thực sự đã dùng và biên bản bàn giao/nghiệm thu từ Agent Kit tại commit `8161bfe28aacfee1915c50bca0f9274add0a1899`. Một lần bàn giao, không thuộc roadmap. Chỉ sửa `AGENTS.md`, thêm `docs/agent-kit/workflow.md` và cập nhật mục này. `docs/current-states.md` là nguồn trạng thái hiện hành duy nhất.

**Xác nhận:** Người dùng yêu cầu không áp dụng Claude, giữ nguyên mọi thành phần Consflow, sau đó xác nhận “Duyệt package thu hẹp này, triển khai.” Quyền hiện tại chỉ bao phủ thay đổi tài liệu local và kiểm tra đúng phạm vi; không gồm commit/push/deploy.

**Đã làm:** Bổ sung mục workflow tại `AGENTS.md` và hướng dẫn/mẫu tại `docs/agent-kit/workflow.md`: planning và phân loại một lần bàn giao/roadmap, WHAT trực tiếp trong chat, Skills thực sự dùng, biên bản có visual dựa trên nguồn/checklist/bằng chứng/quyết định nghiệm thu, điểm dừng roadmap và quyền cho bước tiếp theo. Giữ attribution, nguồn Kit đã ghim và thông báo MIT ngay trong tài liệu mới. Không nhập skills, agents, adapters, lịch sử hoặc nhật ký từ Kit.

**Kiểm chứng:** IMPLEMENTED / TECHNICALLY VERIFIED ở mức tài liệu local trên working tree dựa trên `05274ec4b2bcab360361991d4007b037c10a7683`, chưa commit. Python kiểm tra hashes và inventory của 63 file `.agents/**`/`.codex/**`: PASS; nội dung `AGENTS.md` cũ và toàn bộ các plan cũ nguyên vẹn: PASS; chỉ ba đường dẫn đã duyệt thay đổi: PASS; liên kết tương đối mới, các mục mẫu, upstream pin và attribution: PASS; `git diff --check` và kiểm tra whitespace cho cả file mới: PASS. Tự review nội dung theo phạm vi đã duyệt, không có reviewer độc lập. Build/tests ứng dụng, browser, runtime discovery và DB/migrations không chạy vì đây là thay đổi tài liệu; không có bằng chứng về hiệu lực tự động ở phiên mới hoặc hiệu quả workflow.

**Skills thực sự dùng trong implementation:** `consflow-module-memory` (Repo): bootstrap mapping, xác định owner agent workflow và ngoại lệ giữ nguyên mapping; đóng góp ước lượng 100% trong các skills được áp dụng, không phải số đo thời gian/token/hiệu quả. Không dùng skills Kit và không tạo nhật ký riêng.

**Đóng plan — 2026-11-11:** Người dùng yêu cầu “Đóng plan, commit, push.” Đóng phạm vi tài liệu và cho phép commit/push; không cấp quyền deploy. Việc đóng plan không chứng minh hành vi tuân thủ ở phiên mới; runtime acceptance vẫn chưa kiểm chứng. Các kết quả kiểm tra bên trên mô tả snapshot trước khi đóng plan; bản đóng bổ sung file lịch sử này theo yêu cầu mới.

**Ranh giới mapping:** Mapping có liên quan vì đây là agent workflow. Theo ngoại lệ phạm vi do người dùng vừa duyệt, giữ nguyên cả `.agents/**`, bao gồm mọi mapping; đường dẫn workflow mới được ghi trực tiếp trong `AGENTS.md`. Không đổi ownership, contracts hoặc invariants ứng dụng.
