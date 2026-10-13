# BM03/BM08 — tích hợp development, 2026-10-09

**Giới hạn:** chưa nghiệm thu trọn workflow chính thức, chốt/ký phiếu, nghiệm thu cuối hoặc hoàn tất bàn giao. BM08 Create UI đã từ chối đúng vì chưa có nghiệm thu cuối hợp lệ; kiểm tra đánh giá/xuất dùng hồ sơ nháp được tạo qua API chuẩn. Chưa kiểm tra Microsoft Excel, in giấy, dữ liệu client thật hay tải tối đa. Không coi kết quả này là nghiệm thu toàn bộ tám mẫu.

**Đã kiểm chứng:** chính sách chọn phiên bản BM03, Docker development chuẩn ở V157, Rev10 published và công trình mới nhận đúng phiếu client v3. Hai phiếu BM03 và hồ sơ BM08 đã lưu/tải lại qua màn hình gốc. Ba file cuối tải bằng **Xuất báo cáo**, chuyển nguyên trạng vào Google Sheets và đọc lại toàn bộ vùng có dữ liệu: đúng dữ liệu nguồn, không có lỗi ô. Hiển thị Google ở 100% đã được kiểm tra. Trạng thái: IMPLEMENTED / TECHNICALLY VERIFIED / BOUNDED DEVELOPMENT BROWSER + GOOGLE ACCEPTANCE.

## Phạm vi duyệt và nguyên nhân

- `Approve 1.` duyệt chuẩn hóa development, sửa mặc định BM03, cấu hình workflow mới bất biến, tạo công trình thử qua luồng chuẩn, rồi nghiệm thu lưu nháp/xuất. Ứng dụng đang development; các bản ghi đã có là dữ liệu thử. Giữ dữ liệu đó.
- `Duyệt sửa Rev10` duyệt riêng sửa quyền giai đoạn cuối và phát hành bản nháp Rev10. Không thay Rev9, từng quyền Đầu việc hoặc dự án đã có. Không reset, commit/push, production hay đóng kế hoạch.
- Nguyên nhân đã xác minh: catalog cho chọn phiên bản native mới nhưng mặc định loại toàn bộ `operational_capture_v3`; bước trong workflow published cũng đang pin phiên bản explicit trước. Chỉ sửa default không tự đổi pin đó.
- Chủ sở hữu: `repository_workflow_forms.go` giữ chính sách catalog/default; `defaultWorkflowFormCode` vẫn route tên nghiệm thu thầu phụ sang builtin. Client-package dùng pin explicit trong revision workflow mới. Frontend `pipelineBuilderDomain.ts` đọc `default_eligible`, không có một quy tắc mặc định thứ hai.
- Mapping áp dụng: workflow-orchestration, quality-acceptance-handover; central/back/frontend/triage maps. Tìm theo `workflowFormDefaultEligibilitySQL`, `DefaultWorkflowFormVersionID`, `default_eligible`, `incoming.subcontractor_package`, `handover_finance`.

## Thay đổi đã triển khai

1. Một predicate SQL dùng chung cho catalog và default resolver. Chỉ hai mẫu BM03 được tự chọn khi published và đủ cả marker native, số biên bản, hai kết luận, tiêu chuẩn/bản vẽ/yêu cầu kỹ thuật, khối lượng thiết kế/thực tế và kết quả/nhận xét. Phiên bản thiếu contract và các mẫu native khác không tự được kích hoạt.
2. Builtin tự chọn v4. Client-package v3 được đánh dấu eligible; workflow chính thức được pin rõ vào v3 qua clone/configure/publish. Không đổi phiên bản của instance đã tạo.
3. Sau backup/restore-test, `make up` chạy đúng `down -> build -> run`. Custom AUTO_MIGRATE áp dụng sáu migration additive V152–V157 lên DB development `consflow`. Không sửa migration đã áp dụng.
4. Clone Rev9 thành Rev10; chỉ đổi pin `incoming.subcontractor_package` và quyền giai đoạn cuối đã duyệt. Không thay topology, roles, approval/source/exit gates hoặc quyền từng Đầu việc.

## Source/build/môi trường đã kiểm tra

- Working tree chưa commit trên `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`; giữ mọi thay đổi BMGS đã có.
- [Manifest ứng dụng](acceptance-evidence/bm-gs-native-2026-10-09/development/tested-source-state.json): 716 file source/config/migration, fingerprint `ee6c5811f30e22f9fcfdcd3b6311f1b451e6aa55087620e4fcfcfb6d61d113e3`, capture `2026-10-09T10:03:10.896708Z`. Không file ứng dụng nào có mtime sau khi tạo container chuẩn. Chỉ test, mapping, docs và bằng chứng thay đổi sau build.
- Backend Docker build: binary server/migrate/storage-migrate/bootstrap-admin thành công. Container `/ttcons-backend-1` tạo `08:30:40.746Z`, chạy `08:30:47.415Z`; image `sha256:9973c3f2108af2e719b50a0c86353407c94a03e06d80f4b10ff4b9c0b059e64e`.
- Admin container `/admin` tạo `08:30:40.815Z`, chạy `08:30:47.519Z`; image `sha256:ae079927e9fffaf044651b4765dde5a6ef3f1682964ddcc904932fbd9972610e`. Admin dùng source bind/Vite development; frontend TypeScript/Vite build riêng thành công (4.54s).
- Normal Compose `ttcons`: UI `127.0.0.1:5173`, API `127.0.0.1:8080/api`, PostgreSQL16 DB `consflow`, Redis thường và cache; background workers/startup chuẩn. Health/ready/UI HTTP200; migration count157/max numeric157. Không dùng task launcher làm bằng chứng normal development.
- [Thông tin runtime](acceptance-evidence/bm-gs-native-2026-10-09/development/runtime-images.json), [health](acceptance-evidence/bm-gs-native-2026-10-09/development/runtime-health.json). Log build chuẩn giữ riêng tại `/private/tmp/consflow-development-bm0308-20261009/make-up.log`.

## Backup và dữ liệu giữ nguyên

- Backup V151: 1,064,944 bytes, SHA256 `2fc266f7720e01256274727091181a193c0fd0af676a503e3d9e85181063ffd1`. Dump/record snapshots giữ riêng, không đưa lên repo hoặc Google.
- Khôi phục dump vào DB tạm mới, so snapshot bằng nhau, sau đó chỉ loại DB restore tạm. [Kết quả backup](acceptance-evidence/bm-gs-native-2026-10-09/development/backup-verification.json).
- [Đối chiếu sau chạy](acceptance-evidence/bm-gs-native-2026-10-09/development/retained-record-verification.json): một dự án, 13 template, 162 template node và 507 template step đã có bằng chính xác JSON trước migration/cấu hình. Trước đó không có run/form instance. Rev9 giữ nguyên. Không xóa, reset hoặc cập nhật hồi tố dữ liệu thử đã có.

## Rev10 và công trình mới

| Đối tượng | Kết quả |
|---|---|
| Rev9 | Published `40000000-0000-0000-0000-000000000028`, nguyên trạng |
| Rev10 | Published `60e28e04-1183-4d1f-a357-0c2e821442bf`; 15 node, 21 step, 16 edge giữ nguyên |
| Pin BM03 | `incoming.subcontractor_package` → published v3 `0845e4bc-a008-48bc-be5e-6d873dad8d43` |
| Final node | `required_permission=handover.view`; `permissions=[handover.view,handover.approve,accounting_handoff.view,accounting_handoff.acknowledge]` |
| Quyền từng Đầu việc | Giữ `handover.approve`, `accounting_handoff.acknowledge` |
| Công trình mới | `DEV-REV10-20261009`, `c0acb592-b160-43c9-b8b2-370f386d4de0`, tạo/assign PM/start qua API chuẩn |
| Runtime | Run `99f704fc-a392-46ce-866e-a5e53bb2a851`; phiếu `2f10e39a-d2ee-412e-a343-36e3d0f8cee6` nhận đúng client v3 |

Array `permissions` là gate runtime ANY-permission cho thao tác node, không chỉ danh sách hiển thị. Các kiểm tra actor/source và quyền từng Đầu việc độc lập giữ nguyên. Review này không chứng nhận toàn bộ ma trận quyền của từng vai trò.

[Bằng chứng cấu hình](acceptance-evidence/bm-gs-native-2026-10-09/development/workflow-configuration-result.json), [công trình/runtime](acceptance-evidence/bm-gs-native-2026-10-09/development/official-project-state.json), [màn hình runtime](acceptance-evidence/bm-gs-native-2026-10-09/development/rev10-runtime-form.jpg). Browser hiển thị v3 và chỉ cho xem vì nhánh upstream chưa chốt. Không đẩy thẳng workflow tới nghiệm thu bằng SQL hoặc bỏ gate.

## Kiểm tra từ hồ sơ gốc tới file thật

Mục đích chính: ghi dữ liệu một lần tại phiếu/hồ sơ đang làm, sau đó xuất dữ liệu đã lưu.

Fixture riêng `DEV-BM0308-20261009` dùng workflow test có tên rõ, tạo/publish/start bằng APIs/services thật. Fixture này không đại diện cho tiến trình toàn bộ workflow Rev10. Browser dùng tài khoản Admin development hiện có, xác thực và backend chuẩn.

| Hồ sơ | Thao tác và kết quả đã quan sát |
|---|---|
| Checklist thầu phụ v4 | Nhập số biên bản, tiêu chuẩn/bản vẽ, thiết kế24m2/thực tế23.5m2, Done nhưng chất lượng Không đạt, kết luận Cần sửa chữa tồn tại; Bắt đầu & lưu nháp; mở lại giữ đúng; tải từ Xuất báo cáo |
| Gói nghiệm thu client v3 | Nhập số biên bản và dòng sơn40m2, Đạt, Đồng ý nghiệm thu; lưu nháp/reload/tải. Khối lượng thầu phụ riêng còn thiếu nên chưa chứng nhận Chốt hồ sơ |
| BM08 | Create UI từ chối đúng khi chưa có nghiệm thu cuối; API chuẩn cho phép tạo draft không final source. UI lưu giai đoạn/kết luận, thêm dòng thực tế Không đạt/tồn tại/hạn11-10-2026; reload giữ đúng; tải từ Xuất báo cáo |
| BM08 chưa đánh giá | Sáu dòng nguồn bắt buộc giữ pending và đánh giá trống; không biến trạng thái hồ sơ thành kết quả Đạt/Không đạt |
| Thay đổi chưa lưu | BM03 khóa nút xuất và báo phải lưu; nút mở lại sau Save Draft |

Automation `fill` ngày ban đầu chỉ đổi DOM property, chưa đổi state của controlled input. Bản BM08 đầu không có deadline, được giữ là bằng chứng đã supersede. Thao tác bàn phím cập nhật state; lưu/reload/API xác nhận `2026-10-11T00:00:00Z`; file tải mới và Google đều đúng ngày. Không sửa workbook sau tải, không sửa ứng dụng dựa trên lỗi thao tác automation này.

[Nguồn đã lưu](acceptance-evidence/bm-gs-native-2026-10-09/development/saved-source-readback.json), [BM03 sau reload](acceptance-evidence/bm-gs-native-2026-10-09/development/bm03-package-reloaded.jpg), [BM08 hộp thoại sau reload](acceptance-evidence/bm-gs-native-2026-10-09/development/bm08-assessment-reloaded.jpg).

## Google Sheets — file tải nguyên trạng

| File cuối | Bytes | SHA256 | Google đã kiểm tra |
|---|---:|---|---|
| [BM03 builtin](acceptance-evidence/bm-gs-native-2026-10-09/development/bm03-builtin-actual.xlsx) |11230|`9971cd5c69586cc37f68e772c5c5d1a5164542a6bcede88129dc9b3e8113691e`|[BM03 Không đạt](https://docs.google.com/spreadsheets/d/1jBVLytRioWtM_6PUYkYhZ-hue6kA6XT8mw_054nlg6M/edit)|
| [BM03 client](acceptance-evidence/bm-gs-native-2026-10-09/development/bm03-client-package-actual.xlsx) |11192|`649980999bfea34b090f0df9924ba37d52a0dd6005a47f678d97e254f5b43fbf`|[BM03 Đạt](https://docs.google.com/spreadsheets/d/1SOumXdGnhJ-YwOnrZrRK6-oCpJIsr1KGA6NKWw34JAY/edit)|
| [BM08 cuối](acceptance-evidence/bm-gs-native-2026-10-09/development/bm08-final-actual.xlsx) |11873|`e681a17329e047b1a158af6fcdb9ac8f0098c9846e32e6240d26847f82f64f29`|[BM08 có hạn xử lý](https://docs.google.com/spreadsheets/d/1UGRpqU7HyLOBxHhE6kurM8k08--__TX8WrwQNT0iIS0/edit)|

- Browser download events ghi đúng đường dẫn Downloads; bản giữ trong repo bằng nguyên bytes. Không dựng lại hoặc chỉnh file trước import. Native conversion vào thư mục riêng của người dùng; metadata cả ba xác nhận `shared=false`. Không sửa cell/style/content trên Google.
- BM03 vùng `A1:I28`: số biên bản, nhãn pinned Sàn/Sơn, số khối lượng và hai kết luận đúng. BM08 `A1:H40`: giai đoạn, hạng mục thực tế, yêu cầu, Không đạt, tồn tại và hạn xử lý đúng. F14 là numeric date46306, formatted2026-10-11. Toàn bộ các vùng dùng không có effective error hoặc `#VALUE!`.
- Ảnh chụp Google100%: [BM03 builtin](acceptance-evidence/bm-gs-native-2026-10-09/development/bm03-google-100.jpg), [BM03 client](acceptance-evidence/bm-gs-native-2026-10-09/development/bm03-package-google-100.jpg), [BM08 cuối](acceptance-evidence/bm-gs-native-2026-10-09/development/bm08-google-final-100.jpg). Nội dung chính đọc được và không bị cắt trong ba fixture này. Đây là kiểm tra trực quan có giới hạn, không phải đánh giá usability với người dùng hoặc dữ liệu client thật.
- [Download manifest](acceptance-evidence/bm-gs-native-2026-10-09/development/actual-download-manifest.json), [Google CellData](acceptance-evidence/bm-gs-native-2026-10-09/development/google-cell-readback.json), [assertions](acceptance-evidence/bm-gs-native-2026-10-09/development/result.json).

## Tests và review độc lập

| Kiểm tra | Kết quả / giới hạn |
|---|---|
| Trước sửa: `TestNativeSubcontractorAssessmentDefaults` |FAIL3.035s: complete assessment không được default-eligible |
| Năm Go tests focused default/catalog/assessment/pinned labels |PASS13.125s, gồm tám incomplete-profile subtests |
| Compatibility v1 + hai default tests sau review |PASS8.244s; giữ required/evidence/quantity assertions v1, thêm latest-native default assertions riêng |
| FE `pipelineFormDefaults.test.ts` |6/6 PASS; High chạy độc lập6/6,96.641ms |
| `go vet ./internal/core` |exit0 |
| `npm run build` |TypeScript/Vite PASS4.54s; không frontend app source nào đổi sau build |
| `git diff --check` |exit0; không stage/commit/push |
| GPT-6.1 Sol High source/config audit |Không còn P0–P2 trong delta đã duyệt; tự đối chiếu Rev9/Rev10 topology/pin/permissions và runtime formJSON. Reviewer không chạy GoDB/build/browser/DB mutations |

Go tests dùng schema cô lập trên PostgreSQL disposable localhost55532, migrations V157 thật và checked cleanup. Không chạy test fixtures trong DB `consflow`. Lệnh focused:

```sh
go test ./internal/core -run '^(TestNativeSubcontractorAssessmentDefaults|TestNativeSubcontractorDefaultsRejectIncompleteProfilesAndPreservePins|TestNativeCaptureCatalogExplicitPipelineSelectionPreservesLegacyPins|TestNativeSubcontractorAssessmentRevisionAndCapture|TestNativeWorkflowExportSourceKeepsPinnedSelectOptions)$' -count=1 -v
go test ./internal/core -run '^(TestSubcontractorChecklistFormValidatesEvidenceAndQuantities|TestNativeSubcontractorAssessmentDefaults|TestNativeSubcontractorDefaultsRejectIncompleteProfilesAndPreservePins)$' -count=1 -v
```

Test stdout ở tool outputs, không có file log riêng được giữ. Không viện dẫn file log không tồn tại. GOPATH/GOMODCACHE/GOCACHE đặt trong private tmp, DSN cấp riêng và không ghi vào bằng chứng. Source/test review bundle SHA256 `10d47f265898f39adf037db51b3cad035042dd8813aaaef0a7f41fb8e3f5edc1`; app policy SHA256 `6392352b8b7c649464ed936c81becb5c524a713c56c82de9cfe815b74051137e`.

TechNote Level3 đã cập nhật BM03/BM08 development và giới hạn còn lại. Mock-DOM kiểm tra tám bước, bounds và khôi phục state PASS. Preview IAB thật kiểm tra Tiếp đến bước8, khóa Tiếp ở cuối, Trước về bước7, Xem lại về bước1 PASS; [ảnh bước cuối](acceptance-evidence/bm-gs-native-2026-10-09/development/technote-final.jpg). Đây là kiểm tra công cụ giải thích; không thay thế nghiệm thu ứng dụng. Không chạy lại responsive của TechNote trong lượt này.

## Bước kế tiếp

Nghiệm thu một công trình thử đi trọn workflow Rev10: nguồn nghiệm thu thầu phụ, Chốt hồ sơ/chữ ký, nghiệm thu cuối hợp lệ, Create Handover UI và hoàn tất bàn giao. Giữ riêng kết quả đó với các fixture lưu nháp/xuất đã kiểm tra ở đây. Phạm vi rộng này chưa chạy trong lượt này; không tự coi dữ liệu nháp là hồ sơ đã nghiệm thu.
