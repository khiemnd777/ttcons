# BM03/BM08 — sửa dữ liệu nguồn và nghiệm thu, 2026-10-09

**Phạm vi của lần UAT này:** thời điểm kiểm tra dưới đây chưa áp dụng V157 vào DB bình thường/production. Sau đó người dùng duyệt `Approve 1.`; V157, default policy và workflow Rev10 đã được tích hợp trên development chuẩn, với bằng chứng riêng tại [Development verification](bm-gs-03-08-development-verification-2026-10-09.md). Chưa nghiệm thu client thật, Microsoft Excel, in thực tế, tải tối đa hoặc toàn bộ workflow chốt/ký/bàn giao. Kết quả này không đóng toàn bộ kế hoạch tám biểu mẫu. Không commit/push/deploy.

**Kết quả trong phạm vi được duyệt:** IMPLEMENTED / TECHNICALLY VERIFIED / BOUNDED SYNTHETIC BROWSER + NATIVE GOOGLE ACCEPTANCE PASS. Người dùng duyệt `Approve 1-3`, sau đó duyệt mockup `Duyệt bố trí này`. Đã lưu tại hồ sơ gốc, tải từ nút hiện có và chuyển nguyên trạng bốn file cuối sang Google Sheets. Không có bộ chọn biểu mẫu, nguồn hoặc ảnh lúc xuất.

## Lỗi, nguyên nhân và sửa chữa đã kiểm chứng

| Mẫu | Lỗi trước sửa | Nguyên nhân | Hành vi đã sửa |
|---|---|---|---|
| BM03 | Thiếu số biên bản; kết quả là trạng thái việc/phản hồi đối soát; mất hai lựa chọn kết luận | Schema nguồn không có các sự kiện nghiệm thu; projection dùng sai ý nghĩa dữ liệu; renderer chỉ giữ prefix kết luận | Phiên bản form mới ghi số biên bản, kết quả/nhận xét từng dòng, kết luận. Export đọc answers đã lưu; Done không suy ra Đạt. Hai lựa chọn client luôn hiện và chỉ quyết định đã lưu được đánh dấu |
| BM08 | Thiếu giai đoạn/kết luận; không thêm hạng mục thực tế; không đánh giá thực tế dòng tự động; mất hai lựa chọn kết luận | Dossier thiếu header; thao tác cũ có trách nhiệm xác minh nguồn và từ chối dòng tự động; renderer mất lựa chọn | Ghi header ngay hồ sơ; thêm dòng thực tế; Đánh giá ghi observation trên mọi loại dòng có quyền. Xác minh nguồn/readiness độc lập. Hai lựa chọn client giữ nguyên |
| BM08 | Lưu đánh giá dòng qua browser báo HTTP500 | `repository.go::safeTable` thiếu `handover_checklist_items`, làm handler không resolve được project trước khi vào service | Thêm đúng bảng vào allowlist; regression chạy handler thật chứng minh save/create hợp lệ và từ chối project khác với403 |
| BM03 | Google hiển thị `flooring`, `24. m2` | Projection chưa đọc nhãn của options pinned; number format có dấu thập phân khi số nguyên | Đọc options_json đúng phiên bản form; dùng format theo precision giá trị thực: `Sàn`, `24 m2`, `23.5 m2` |
| BM08 | Ngày chưa có bằng chứng tương thích | Fixture trước chưa lưu ngày; automation fill chỉ đổi DOM property, chưa cập nhật React state | Thao tác bàn phím thật, lưu/reload và DB xác nhận2026-10-12. Export là numeric DATE; Google đọc đúng46307/yyyy-mm-dd. Đây không phải thay nguồn bằng hardcode hoặc sửa file sau tải |

High reviewer đã rút lại kết luận sạch trước đó vì service-only review bỏ sót allowlist. Audit cuối trên delta đã sửa không còn phát hiện actionable P0–P2; đó là kết quả source review, không thay nghiệm thu browser.

## Source, build và môi trường

- Working tree chưa commit trên `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`; nhiều thay đổi BMGS từ các bước trước vẫn được giữ nguyên.
- [Manifest cuối](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/final-tested-source-state.json):114 file ứng dụng, fingerprint `b5ae32da905be9706ee4652921da25dcb149e2c81d7dbb8677035961c7a2f085`, freeze04:14:31UTC. Kiểm tra hash cuối xác nhận cả114 file còn khớp; sau build chỉ cập nhật bằng chứng/docs/maps.
- `npm run build` (tsc/Vite) PASS4.46s; frontend dist mtime03:57:49UTC. Hai executable normal-entrypoint và guarded task API build PASS, mtime04:13:50UTC và04:13:49UTC. Build normal-entrypoint không phải deploy production.
- UI `127.0.0.1:15174`, API `127.0.0.1:18082`; API khởi động mới từ guarded executable cuối trước khi tải bốn file. Browser dùng Admin tổng hợp, core routes/JWT/project-scope/storage thật; launcher cô lập không chạy normal background workers/observability.
- PostgreSQL16 task container `consflow-bmgs-uat-pg`, DB `bmgs_native_browser_20261008`,157 custom migrations/max **numeric**157. V157 chỉ chạy task DB. Redis/task protected storage giữ môi trường tổng hợp. Backup trước migration nằm riêng trong private tmp, không đưa vào repo/Google.
- Integration dùng DB riêng `consflow_bm0308_regression` và schema cô lập; DB đã bị loại sau tests. Không chạy test fixtures trên DB ứng dụng bình thường.

## Kiểm tra kỹ thuật

| Kiểm tra | Kết quả thực tế và phạm vi |
|---|---|
| Go focused capture/migration/HTTP | PASS13.552s; omission/clear, enum/type/date/length, audit, quyền project, trạng thái terminal và cạnh tranh completion; immutable forms/defaults/history |
| Handler regression sau browser500 | PASS3.096s: handler thật save/update/create và foreign-project403 |
| Final capture/pinned options/HTTP | PASS8.159s; pinned field label không lấy từ catalog mới hơn |
| `go test ./internal/core -run '^TestBMGS'` | PASS1.901s; reopened-XLSX mapping/conclusions/units/dates/dynamic rows; BM04/BM07 regressions giữ hành vi. Opt-in fixture export không thay acceptance file thật |
| Frontend focused tests |21/21 PASS: assessment/source presentation/eligibility/native saved-export lifecycle |
| `npm run build`; `go vet ./internal/core`; fresh API builds |PASS/exit0 trên manifest cuối |
| GPT-6.1 Sol High độc lập |114 hashes khớp, scoped source audit không còn actionable P0–P2; pure BMGS/handover validation/safeTable PASS1.902s, focused FE18/18. Không tự chạy DB/browser/Google |
| `git diff --check` và final hash check |PASS; không stage/commit hoặc sửa lịch sử |

Go dùng `/usr/local/go/bin/go`, GOPATH/GOMODCACHE/GOCACHE trong private tmp do caches mặc định read-only. Chi tiết regression owners: `native_handover_assessment_test.go`, `native_subcontractor_assessment_test.go`, `bm_gs_acceptance_renderer_test.go`, `handoverAssessment.test.ts` và source/export lifecycle tests.

## Nghiệm thu hồ sơ nguồn qua browser

Mục đích chính: nhập một lần trong phiếu/hồ sơ đang làm, sau đó xuất dữ liệu đã lưu.

| Kịch bản | Bằng chứng và kết quả |
|---|---|
| BM03 `builtin_subcontractor_checklist` | Form26c01b3c-25e1-45c7-9158-e9858687a563/versionv4, Đầu việc54584f58-8d1f-4e41-a828-3c62f89f6d33. Lưu/reload NT-UAT-157-01, phạm viSàn,24/23.5m2, bốn dòng gồm Chưa đánh giá dù Done; kết luận accepted. Unsaved disable export, saved enable; download thật |
| BM03 `client_subcontractor_acceptance_package` | Formfd224aba-8416-499b-a2a5-f0a5cdf422d9/versionv3, Đầu việc29035eca-8c25-405d-900e-b853d9a99ce3. Lưu/reload NT-UAT-157-02, Mộc / nội thất,10/9.5m2, quality fail dù Done; kết luận rework_required. Mobile cards390×844 đọc/điều khiển được |
| BM08 hồ sơ liên kết | bb2ae20e-6899-4632-ba27-e802758f3756: header stage/eligible; dòng evidence pass, thử lưu rồi xóa explicit tồn tại/hạn. Reload và DB xác nhận đã xóa; source/status/verified actor/time giữ nguyên |
| BM08 hồ sơ canonical | d4d8d476-3d44-4fb6-8cbe-5476ce86d837: canonical service tạo sáu dòng; browser lưu manual as-built pass, automatic warehouse fail/tồn tại/hạn2026-10-12, thêm physical rowpass và header not_eligible. Bảy dòng vẫn pending/unverified; readiness chưa kiểm tra; dossier vẫn draft |
| Edge/error/mobile | Stage161 ký tự và tên dòng trống bị inline reject; HTTP500 giữ dialog/draft, retry sau sửa thành công. Header save/reload trên390×844; desktop quan sát action và state tách biệt |

[DB readback cuối](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/final-db-readback.json) ghi27 old pin/answer hashes không đổi, linked verification giống baseline, explicit clear/date/conclusion actor-time đã lưu, canonical readiness `{}`/documents_readyfalse/warehouse_closedfalse/completed_atnull, test DB đã loại.

Hai BM03 giữ nháp; gói thầu phụ còn điều kiện bắt buộc `Khối lượng thầu phụ`, chưa chốt/ký. BM08 canonical fixture được tạo qua service có guard task DB, không chứng minh Create dialog vượt điều kiện nghiệm thu cuối. Các giới hạn này không bị export sửa hoặc bypass.

Standard components: schema-rendered WorkflowFormPanel, Dialog/SimpleForm, NativeExcelExportButton, shared API client/ToastProvider; HandoverChecklistSourceDialog/EvidenceReferenceViewer giữ source linking/viewing. Không thêm uploader/previewer riêng.

## Bốn file tải từ nút Xuất báo cáo

[Actual download events](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/actual-download-events.json) và [workbook inspection](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/actual-workbook-inspection.json) ghi đường dẫn Downloads và SHA256 source/copy bằng nhau. Files không được sửa trước import; chỉ copy byte-for-byte để giữ bằng chứng.

| File nguyên trạng | SHA256 | Native Google Sheet |
|---|---|---|
| [BM03 đồng ý](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm03-accepted-final.xlsx) từ BM-GS-03 (4).xlsx | e587fafa909601c494c4fc8692b6c9b8f880e12ab2f594fda541ec7432bd6c86 | [Mở](https://docs.google.com/spreadsheets/d/18EK6WnmjqbKVwRgRQFBeeiMosbgSJAq7i9B3zofIcQw) |
| [BM03 cần sửa](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm03-rework-final.xlsx) từ BM-GS-03 (5).xlsx | 6a25ae756316c11308dda8f9173eba637da4dd7286ded691cfa428dfa8f20b02 | [Mở](https://docs.google.com/spreadsheets/d/1YTnYKo6RsBquTGirdp3KB9ZJjOI-96CAETWFni_6Rcw) |
| [BM08 đủ điều kiện](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-eligible-final.xlsx) từ BM-GS-08 (2).xlsx | fa27d87f723f79353fc20f90b48a136dc49872608b992c89d22f11c87da8000c | [Mở](https://docs.google.com/spreadsheets/d/1Ya-GGJDMD8--cJxPMKsTMSuyIOjRM8VcCIKvpthVYTs) |
| [BM08 chưa đủ điều kiện, có hạn](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-not-eligible-final.xlsx) từ BM-GS-08 (3).xlsx | 3554f75364444f1469e88b0c6de6762f4c56d3307f049fbdbc16821ba5b685af | [Mở](https://docs.google.com/spreadsheets/d/17IbvFTDUaSMAy8gYMY-bHqs2iDrXmMLmP_IWXTzuV7Y) |

Tất cả là native Google Sheets, private (`shared=false`) trong thư mục ChatGPT đã có. Không thay quyền chia sẻ, chỉnh cells/style, chèn ảnh hoặc transform file. Google locale/timezone vẫn mặc định; ngày typed46307 vẫn hiển thị2026-10-12.

Readback chính: [Google evidence](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/google-readback.json); [dated BM08 final evidence](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/google-final-date-readback.json). Negative Sheet cũ1LkHtNyTnP6WtNMLeYgVGgwHVimYrq0RM_uaNUu4RXk8/BM-GS-08 (1).xlsx **SUPERSEDED** vì ngày chưa lưu. Chỉ Sheet17Ibv… chứng minh ngày của file cuối.

Trong vùng báo cáo được đọc (BM03 A1:G30, BM08 A1:F30), không có effectiveValue.errorValue/`#VALUE!`. BM03 accepted B5=NT-UAT-157-01/B6=Sàn/E9=24m2/F9=23.5m2; G10 giữ Chưa đánh giá; A14 đánh dấu đồng ý. Rework giữ Không đạt/A13 đánh dấu cần sửa. BM08 đủ điều kiện giữ header/đánh giá đúng; bản bảy dòng giữ D9/D14 Đạt, D11 Không đạt, ô chưa đánh giá trống, F11 numeric46307 DATE yyyy-mm-dd, A16 đánh dấu chưa đủ điều kiện. Chữ ký BM08 dịch xuống A28:D29; đủ bốn vai trò và chỉ dẫn ký. Kiểm tra lỗi này giới hạn vùng báo cáo, không tuyên bố rà mọi cell/appendix.

Browser Edge ở100% đã quan sát bốn bản kết luận, kết quả, ngày, wrapped text và chữ ký. Ảnh chụp gốc:

- [BM03 đồng ý](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm03-google-final-accepted.jpg), [BM03 cần sửa](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm03-google-final-rework.jpg).
- [BM08 đủ điều kiện](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-google-final-eligible.jpg), [BM08 ngày/kết luận](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-google-final-not-eligible.jpg), [BM08 chữ ký](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-google-final-signatures.jpg).
- [BM03 mobile đánh giá](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm03-mobile-assessment.jpg), [BM08 mobile header](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-mobile-editor.jpg), [BM08 desktop](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-desktop-saved.jpg), [BM08 date saved/reloaded](acceptance-evidence/bm-gs-native-2026-10-09/bm03-bm08/bm08-date-saved-reloaded.jpg).

## Việc tiếp theo

1. Áp dụng rollout được duyệt cho V157 trên môi trường chính; cấu hình phiên bản form thầu phụ mới qua dropdown hiện có cho workflow tương lai. Không tự nâng pin/hồi tố hồ sơ cũ.
2. Nghiệm thu bằng hồ sơ client đủ dữ liệu và các template chưa có đầy đủ kiểm chứng Google; tách nghiệm thu chốt/ký/bàn giao khỏi kiểm chứng xuất nháp.
3. Thực hiện Microsoft Excel/in/tải tối đa khi phạm vi này được giao. Chỉ đóng parent plan sau khi các tiêu chí còn lại đạt.

Các bước trên là việc còn lại ngoài đợt sửa đã duyệt; không phải đã thực hiện hoặc một yêu cầu phê duyệt gộp. Mapping central/backend/frontend/triage và Quality/Workflow/Reporting đã cập nhật cùng đợt.
