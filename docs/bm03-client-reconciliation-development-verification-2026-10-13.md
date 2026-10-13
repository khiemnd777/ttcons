# BM03 gói client — Kiểm chứng Development ngày 13-10-2026

**Trạng thái: UX ACCEPTED trong phạm vi BM03 đã review.** Anh xác nhận `Chốt nghiệm thu BM03` ngày13-10-2026. Các blocker thuộc phạm vi đã duyệt đã được sửa. Công trình thử dùng đầy đủ Rev10/engine4 đã qua nguồn hợp lệ tới gói client BM03, chốt phiếu, gửi đối soát R1/R2 và hoàn tất đúng Đầu việc trên trình duyệt. Hai file tải nguyên từ nút Xuất báo cáo mở trên Google Sheets với ảnh hiển thị,900 ô đối chiếu không có lỗi hoặc sai giá trị. Các giới hạn dưới đây vẫn giữ nguyên; không tự chốt toàn công trình hoặc BM khác.

Giới hạn: đây là dữ liệu giả lập Development với actor Admin. Chưa kiểm người dùng nghiệp vụ thông thường, chất lượng công trình thật, giấy/chữ ký pháp lý, Microsoft Excel/in thực tế, toàn bộ quy trình công trình hoặc hồ sơ tài chính. Giai đoạn incoming_acceptance còn Đầu việc vật tư riêng; không tuyên bố hoàn tất giai đoạn hoặc công trình.

## Phạm vi đã duyệt

- Đối soát từ checklist hiện có ngay trong gói client; đổi ràng buộc theo phiên bản bằng V158; giữ lịch sử, không thêm cột/field/form mới.
- Tái dùng WorkflowFormPanel, SubcontractorReconciliationPanel, API/toast và bộ xem minh chứng chuẩn. Giữ nút Xuất báo cáo.
- Gói client chỉ đủ nguồn khi đối soát của chính phiếu/phiên bản hiện tại đã gửi. Dữ liệu quantity_items độc lập; design/measured/quality của BM03 không lấy từ khối lượng thương mại.
- Sau khi phát hiện blocker hợp đồng, anh đã duyệt riêng việc nhận chữ ký từ gói Rev10 hiện có bên cạnh bước legacy; giữ giao diện ký và kiểm tra nguồn/người ký/phiên bản.
- Anh đã duyệt riêng1–3 sửa nested checkbox của phiếu bàn giao kế hoạch, giữ minh chứng bắt buộc và lịch sử; không thêm field/schema/UI.
- Không Production, Git delivery, sửa lịch sử, early-start, waiver, SQL ghi trạng thái hoặc xuất từ template rút gọn trong official family.

## Kết quả implementation và ảnh hưởng

| Khu vực | Chủ sở hữu hiện có | Kiểm tra va chạm | Kết quả |
|---|---|---|---|
| Đối soát và BM03 | service/repository_reconciliations; native projection | Checklist, bảng khối lượng riêng, dữ liệu đo thực tế | Chọn đúng checklist; mapping vật lý không đổi |
| Phiên bản và lịch sử | WFI snapshot/revision; reconciliation ledger | Submitted R1, R2 mới, request stale và ghi đồng thời | Tests giữ hash cũ; một ledger/item/audit/activity; fail closed |
| UI và minh chứng | WorkItemPrimarySurface; panel/viewer chuẩn | Observer/closed/pending, dirty/busy, lịch sử, responsive | Controlled tests và browser chỉ xem/pending đạt trong phạm vi nêu dưới |
| Nguồn workflow | repository_pipeline/execution target/sample consumers | Không nhận nguồn khác hoặc revision cũ | Actual R1 đã gửi → ready; R2 chưa gửi → chưa ready; R2 đã gửi → ready/hoàn tất; sample không fan-out |
| Hợp đồng Rev10 | ContractDetail + App-sign mutation | Exact step/form/pin/project/revision/actor, legacy, duplicates | Shared predicate, tests và actual UI ký thành công |
| Checkbox bàn giao | Shared workflow validator + exact handoff defaults | Boolean thật, sai kiểu, optional false, required và minh chứng | Lưu/chốt/kích hoạt actual phiếu thành công; evidence gate giữ nguyên |

V158 chỉ thay hai ràng buộc unique bằng các khóa trên ID/cột phiên bản hiện có. Không backfill, thêm cột hoặc cập nhật row cũ. Không hứa khôi phục global uniqueness sau khi đã có nhiều revision bằng cách xóa lịch sử.

Nguồn client đã chốt đọc answers_snapshot_json cùng current_revision. Submitted ledger cũ chỉ đọc, không thay bằng answers/BOQ mới. If-Match là metadata transport, không phải field capture hoặc token lưu trong database. Client không thể ghi khi Đầu việc đóng hoặc giai đoạn không còn hợp lệ; builtin giữ hành vi legacy.

## Source và kiểm thử

HEAD `75dc44707785d399ed2408959bf784928a544e96` cùng dirty tree đã có; không reset/stage/commit/push. Manifest cuối có976 application/test/migration files, khớp toàn bộ source đã build ở lượt cập nhật checkbox. Các mapping central/backend/frontend/triage/Commercial/Workflow đã cập nhật. Checkbox không thay UI nên không cần thêm mapping frontend cho riêng validator.

GPT-6.1 Sol Medium triển khai riêng backend/frontend; High review độc lập:

- Backend worker: client/revision, execution target và sample completed/rollback — PASS30.891s.
- Root: `TestClientSubcontractor|TestWorkItemExecutionTarget|TestNativeSubcontractor|TestSubcontractor` — PASS37.131s; core vet PASS.
- High: sáu `TestClientSubcontractor` — PASS23.642s. Migration có legacy client revision0/header/items trước V158; hash không đổi. Actual amend R1→R2, stale interleaving, inactive node và concurrent single-effect có assertions.
- Frontend worker:25 focused controlled tests; build cuối PASS, Vite4.63s. High suite mục tiêu8/8 PASS243.3ms. Có readonly copy, anchor, dirty preservation/412, history no live fallback và standard-component checks.
- App-sign: RED trước sửa; source matrix GREEN6.217s, High6.696s. High chạy contract lifecycle + BM03 regressions PASS21.369s. Guards legacy/Rev10, wrong project/step/pin/code/purpose, stale/unsubmitted, duplicate và immutable signature hash.
- Checkbox: RED trước sửa có actual isolated handoff draft400; worker combined checkbox/handoff/BM03/App-sign PASS34.553s và assertions omission/null cuối PASS9.089s. High combined PASS39.446s và cuối PASS6.628s. Giữ optional nested false, từ chối sai kiểu, không mutate input hoặc source khi request thất bại. Required nested false/missing bị từ chối cả draft theo hành vi hiện có; top-level required check vẫn submit-only. Handoff false/null/omitted thành true, không che sai kiểu. Existing activation/completion evidence regressions vẫn pass.
- Các Go DB tests dùng database kiểm thử riêng `consflow_bm02_receipt_test_20261012` và schema cô lập/custom migrations. Không dùng dữ liệu Development làm unit fixture.
- Core vet sau checkbox PASS. `git diff --check` PASS. Không phát hiện P0–P2 chưa giải quyết trong source review của các phần implementation đã duyệt.

## Development và lịch sử

UI5173/API8081, database consflow158 migrations. Ba lần standard make restart giữ down→build→run, dùng task env `/private/tmp/consflow-bm02-development-20261012.env`. Lần đầu sandbox không cho Docker ghi buildx cache; scoped escalation đã được duyệt tự động và retry exit0. Lượt update hợp đồng và checkbox đều exit0. Không đổi env/ports/Makefile/Compose; Java8080 giữ nguyên.

Sau lượt build đầu, chỉ copy readonly của FE và test đổi; build FE mới và active bind/HMR được kiểm trên browser. Lượt build hợp đồng bao gồm source975, lượt checkbox bao gồm976. Sau lượt cuối, `/api/health`, `/api/ready` và UI đều200; container/source metadata lưu trong evidence. Hai probe nhầm `/health`/`ready` ngoài `/api` trả404, đã kiểm lại đúng routes; đó không phải lỗi runtime. Không có app edit sau lượt build cuối.

Baseline trước mutation:36 bảng,2299 rows và133 tệp storage. Sau nghiệm thu:2477 rows và143 storage files; mọi ID/hash cũ còn nguyên. V158 chỉ tăng migration count; không thêm migration cho App-sign/checkbox. Có backup private trước migration. Verifier dùng snapshot/hash, không coi đó là restore-test của backup.

## Bằng chứng giới hạn từ phiếu đã đóng trước đó

**Đây là bounded export/read-only proof, chưa phải active reconciliation trong full Rev10.**

1. Phiếu client đã chốt từ lần thử engine2 trước: backend trước sửa available=false, sau sửa available=true/can_write=false vì Đầu việc completed. Browser hiện đúng checklist23/20/21, Chưa lưu đối soát và chỉ xem; Lưu nháp/Gửi đối soát disabled. Không tạo ledger để sửa ngược hồ sơ đã đóng.
2. Desktop và mobile390×844: panel nằm sau phiếu; dòng responsive; document/content width đều390, không tràn ngang. Phiếu Rev10 còn pending hiển thị lý do cần chốt trước đối soát, không có thao tác ghi.
3. Click actual Xuất báo cáo tải `/Users/khiemnguyen/Downloads/BM-GS-03 (12).xlsx`. Evidence copy `BM03-client-closed-after-fix.xlsx` bằng từng byte. ZIP hợp lệ.60 source cells bằng file trước sửa: design25; measured24.8/25/24.6; phần commercial23/20/21 riêng. Đây là facts của phiếu lưu, không chứng minh client ledger đã gửi.
4. Import nguyên file, không sửa workbook hoặc sửa Google: https://docs.google.com/spreadsheets/d/1C-y-vcgJyDxT1JOH7tjRlKNq7O9lKV2Su5lpyEygS9M/edit . Private như mặc định. Metadata đọc trước cells; A1:G30 kiểm210 cells,60 có dữ liệu khớp, không effectiveValue.errorValue/#VALUE. Top/footer nhìn tại100%.

## Công trình Rev10 mới và phần mở rộng hợp đồng

Project432115cb-bd5b-4fae-9edc-74ce031eacf3, run d94e292e-7349-4d48-b741-0aa821241e68 từ full published Rev10/template60e28e04-1183-4d1f-a357-0c2e821442bf, engine4/version10. Membership/PM/start dùng API hiện có. Actor Admin trong công trình thử; chưa nghiệm thu người dùng nghiệp vụ thông thường.

Canonical progression đã thực hiện:

1. Intake client_boq + PDF nguồn test đúng Đầu việc → edge chính thức bỏ khảo sát/thiết kế.
2. BOQ draft→submit→norm lock→approve; quotation submit→qualify→select khóa giá cuối. Gói BOQ chốt và đủ document; automation tạo procurement.
3. Gói hợp đồng pin client_hardcopy_dispatch chốt bằng nút trên browser. Existing Ký phiên bản mở shared signature dialog; UAT ink và ghi chú nội bộ, approved_by/current revision. Ký và lưu thành công, một signature-link hiện hành.
4. PDF native có chữ ký UAT thực tế/checksum tải nguyên byte, upload thành tài liệu trả lại nội bộ; canonical dispatch receive→activate. UI cho4/4 conditions. Đây là mô phỏng Development, không có gửi giấy, chữ ký pháp lý hoặc thanh toán thật.
5. Takeoff/sourcing/negotiation dùng đúng BOQ release và selected quote; procurement approved và supplier/payable contract version approved. Complete/advance hợp lệ tới construction_planning.
6. Shared checkbox fix cho phép actual draft/submit handoff. Kích hoạt tạo đúng hạng mục/check có minh chứng bắt buộc. Lịch thử ban đầu09:00 là giả định fixture không cần thiết; Start bị guard từ chối, không dùng override. Chỉnh sửa nguồn qua Amendment R2 bỏ lịch optional, giữ R1; canonical activation chỉ thay hạng mục chưa bắt đầu/chưa có evidence. Snapshot cũ vẫn còn. Ảnh của chính hạng mục mới được upload/mở trong viewer chuẩn, quan sát đủ bốn góc và nhãn SYNTHETIC; canonical check pass hoàn tất hạng mục.
7. Báo cáo ngày đúng pin được submit/complete. Advance bình thường tới incoming_acceptance; gói BM03 client được start và chuẩn bị nguồn đúng Đầu việc, không mượn ảnh từ check hoặc công trình khác.

Không dùng sample-data generator (ghi trực tiếp), SQL state updates, bỏ gate, borrowed source hoặc nguồn từ công trình khác.

## Nghiệm thu gói BM03 client và lịch sử

Đầu việc `778be55c-f1aa-423a-b888-40807c11c2d5`, phiếu `f5dbac7f-86f4-4b16-a0a1-80092e8592db`, exact client package v3. Existing form/panel/API/toast/evidence viewer được tái dùng; ảnh client `99d24495-827c-42b6-b575-dea587da6b52` thuộc đúng Đầu việc.

1. Actual UI Chốt hồ sơ R1, chọn BOQ hiện có và Lưu nháp/Gửi đối soát. Ledger R1 có3 dòng; dòng1 claim23, xác nhận thương mại22/Điều chỉnh KL. Nguồn phiếu vẫn thiết kế25/thực đo24.8, confirmed trong phiếu23. Readiness chuyển true đúng ledger R1.
2. Actual UI Chỉnh sửa→lý do→Bắt đầu chỉnh sửa→Lưu chỉnh sửa tạo R2. Số biên bản R2, thực đo24.2, claim/confirmed trong checklist24; quantity_items riêng vẫn23/20/21 như R1. R2 phải có ledger riêng; readiness false khi mới amendment, không nhận ledger R1 thay thế.
3. Mở Xem lịch sử R1 trên UI:3 dòng, quyết định cũ, xác nhận22, BOQ/payload ảnh đúng; toàn bộ header/items JSON bằng snapshot trước R2, các controls chỉ xem. R2 Lưu nháp/Gửi đối soát riêng xác nhận24,3 Chấp nhận; readiness true. Actual Hoàn tất đưa chính Đầu việc client sang completed lúc07:38 +07, incoming_acceptance vẫn còn việc vật tư riêng. Các ghi chú không sửa vẫn giữ nguyên nguồn; ghi chú thử tổng quát của R2 còn cụm Rev10 R1 do chỉ sửa số biên bản và checklist, không tự đổi nội dung ghi chú.
4. Desktop và mobile390: panel cùng phiếu; trên mobile các controls xếp dọc, body/document width390, không tràn ngang. Ảnh chuẩn có thể mở từ dòng đối soát; không tạo uploader/viewer riêng.

Hai lần tải bằng đúng nút Xuất báo cáo của phiếu, bản copy bằng từng byte:

| File | Nguồn | Thiết kế / thực đo dòng1 | SHA256 |
|---|---|---|---|
| BM03-Rev10-R1.xlsx | Downloads/BM-GS-03 (13).xlsx trước amendment |25 /24.8 m2 |613c351ac91c8786ee93dbdf69d1e191ad11a70c2ee878ad6e2baa2c18c32ed9 |
| BM03-Rev10-R2.xlsx | Downloads/BM-GS-03 (14).xlsx sau own R2 submit |25 /24.2 m2 |7c80ff4b7e97d65a85c1bc2607d85d2ac28d03dae47b8943ab10a4659006c4e1 |

BM03 vẫn xuất từ pinned form answers. Phần thông tin commercial nguồn ở chân phiếu là confirmed23 của R1/24 của R2, không lấy ledger override22 để thay dữ liệu phiếu hoặc số đo. Đây là hành vi nguồn giữ nguyên trong scope sửa blocker; không tuyên bố file là bảng đối soát thương mại riêng. R1 là file đã tải được giữ nguyên, chưa kiểm tái tạo XLSX lịch sử bằng một endpoint khác.

Hai workbook có một sheet biểu mẫu và sheet Hình ảnh phụ lục, một PNG960×640 cùng hash `aa1cf046ba0f4bdffaf655c05670ee5a549de41fe57dd803baf5a2e14d155afe`. PNG đủ bốn góc và tỷ lệ1.5, đúng ảnh nguồn. Đây là drawing nhúng của phụ lục, không tuyên bố Microsoft Place in Cell.

Import nguyên hai file, không sửa Google hoặc workbook:

- R1: https://docs.google.com/spreadsheets/d/1ptpkllrh0k5_w0AVifCWY2AZtNLt8iQ2EMIT8oW7Yug/edit
- R2: https://docs.google.com/spreadsheets/d/1EEhBGBJpKIe6GACYKBLTCGOZQKAdbP3_7lqOaiESuyI/edit

Metadata đọc trước bounded cells. Mỗi file kiểm A1:G30 biểu mẫu và A1:H30 ảnh; tổng900 ô,122 ô populated khớp,0 effective errors/#VALUE. Report và ảnh hai phiên bản nhìn tại100%; R1 được mở lại sau R2, số đo24.8/ảnh vẫn nguyên. Footer R2 cũng được chụp: các phần nguồn/kết luận/ghi chú và ba ô ký còn đúng bố cục, không tự thêm chữ ký BM03. Không đổi sharing; native Google vẫn private. ZIP/XML kiểm đúng actual files, không phát hành workbook tái dựng hoặc sửa sau download.

## Kiểm chứng cuối và bước tiếp theo

- `uat.py verify`:2299 old rows/133 old files không đổi; bằng chứng phiếu đã đóng giữ nguyên.
- `uat.py verify-rev10`:28 assertions PASS; own-source readiness, R1 immutable/history, independent quantities, client completed, hai file/ảnh và900 Google cells đều đúng.976 source hashes khớp build cuối.
- Review source và audit bằng chứng độc lập High đã hoàn tất, không còn P0–P2 trong phạm vi này. Reviewer chạy lại hai verifier, kiểm976 hashes và11 screenshots; xác nhận15 giai đoạn/runtime form pins bằng full published Rev10. Ba events handoff gốc giữ nguyên sau canonical amendment R2. Read-only audit không tự ghi source/domain hoặc sửa Google.
- Anh đã chốt UX ACCEPTED phạm vi BM03 này ngày13-10-2026. Không cần thêm implementation trong scope đã kiểm; không đóng plan hoặc tự triển khai giai đoạn còn lại. Tiếp theo review trạng thái các mẫu còn lại, ưu tiên BM08 nếu anh duyệt phạm vi nghiệm thu tiếp.

Evidence root: `docs/acceptance-evidence/bm03-client-reconciliation-development-2026-10-13/`. Ảnh review chính: rev10-client-completed-export-button.jpg, rev10-client-r1-submitted.jpg, rev10-client-r1-history-readonly.jpg, rev10-client-r2-submitted.jpg, rev10-client-completed-mobile-rows.jpg, rev10-google-r1-report.jpg, rev10-google-r2-report.jpg, rev10-google-r2-footer.jpg, rev10-google-r1-photo.jpg, rev10-google-r2-photo.jpg. Baseline/build/source/canonical response snapshots, local files, Google metadata/cells và verifier outputs lưu cùng folder.
