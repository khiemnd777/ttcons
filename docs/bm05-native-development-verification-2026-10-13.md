# BM05 — Nghiệm thu phiếu đánh giá rủi ro, 13-10-2026

**Kết quả hiện tại:** UX ACCEPTED trong phạm vi đã review: user explicit `Chốt nghiệm thu BM05`. Native risk v4 saved-draft một dòng, sửaB5K5/actualXuấtbáocáo/nativeGoogle/165ôkhớp/0#VALUE/row33pt/fullname/dữliệulịchsửgiữnguyên được chốt. Bằng chứng trước sửa và các phạm vi NOT_RUN vẫn được giữ; không suy ra submitted/fullworkflow/roles/Excel/print acceptance. BM01 cũng đã được chốt trong phạm vi đã review.

## Bản sửa được duyệt và nghiệm thu lại

- Approval: user explicit1–3/mockup/B5K5/nofields/history/Development/browser+Google/screenshots. SolMedium owns renderer+focusedtest; SolHigh read-only independentreview; root maps/runtime/UAT/docs. Existing before-file/Google/screenshot evidence below remains intact as history and is superseded only for current B5 finding.
- Source delta:5lines BM-GS-05 branch in `bm_gs_workbook.go::bmgsPopulate`, `MergeCell(B5,K5)` before existing merge-aware fitting. New `bm_gs_risk_header_test.go`; global fit, sources/APIs/frontend/schema/template assets remain unchanged. Central/backend/triage/Constructiondomain maps updated for this behavior.
- RED reopened-workbook tests fail missingmerge/short65.4pt/long347.55pt. GREEN fullfrozennames/row33pt-short35.7pt-long/sourceJSONnonmutation/all11columnwidthsheaders/riskdatesstatus/signatures plus7other-template merge geometry. Exact scoped command from backend: `GOCACHE=/private/tmp/consflow-bmgs-go-cache GOMODCACHE=/private/tmp/consflow-bm02-diagnosis-modcache go test ./internal/core -run 'TestBMGS|TestNativeBMReuse' -count=1` PASS1.980s. Independent HighfocusedPASS0.569s/no newP0–P2 in source delta, recorded `header-fix/source-review.md`.
- Build/update: running Development uses exact Compose envfile `/private/tmp/consflow-bm02-development-20261012.env`; configquietPASS. `docker compose --env-file … build backend` PASS, then `up -d --no-deps backend` PASS. Backend image `sha256:f1aa879da541bb1636b7f8e9490f08a5a9b7374b012a9b1d70ea34c7e7254ee8`, started11:58:17ICT13October. Only backend recreated; admin/postgres/redis/redis-cache container IDs remain identical. APP_ENVdevelopment/AUTO_MIGRATEtrue; databaseV158 unchanged/no new migration. `/api/ready` and frontend HTTP200.979current source hashes match built manifest; exactly one existing renderer changed and newtest added. Frontend source unchanged, retained frontend build10:56:48ICT remains relevant; no fresh frontend build/fullsuite claimed. Make/Dockerconfigs unchanged.
- Setup limits: initial default build denied writing Buildx activity outsideworkspace; scoped escalation for exactly approved backendbuild succeeded, original failure log retained. Worker default Go cache denied and one temporaryfpdfcache incomplete; used existing complete taskmodcache/writabletaskGOCACHE, no global/dependency/source bypass. Browser reload reached login; ordinary reauthentication through autofilled existing same-account form then exact WI completed UAT. Cause/session endurance not diagnosed or attributed to this repair.
- Actual browserdownload `/Users/khiemnguyen/Downloads/BM-GS-05 (9).xlsx`; byteequal `header-fix/BM05.xlsx`,10,821bytes,SHA256 `2afa0cb845183d47c0d9e743bd76c305b888b55c2e65a88885a7259a36c80bad`. Original before-file hash/data remain intact. All cell values including complete projectname/risk row/signatures match before; only newmergeB5K5 and fittedheight33pt, all risk-column XML geometry preserved.
- New private nativeGoogle ID `1cA_IV__R9KFaDnb9hs6Gng28YUx6s6yuFZYBkJIU-RQ`,gid936714466, exact same tab. Nativeconversion/owneronly confirmed; metadata→boundedA1K15 gives165cells/33populated matchingXLSX/errors0/formulas0. Namebox navigation toB5 confirms actual merged selection B5:K5 and fullprojectvalue. Normal Sheets100% screenshots show one readable name line for this fixture and all risk/footer content. No Googlecell/style/share edit; Google navigation changes selection only.
- `header-fix/verify.py`24/24 evidence assertions PASS:2,893oldunique rows unchanged/0lost,149storagefiles andV158 preserved; only1native_source_exported auditadded. Exact formpin/answers/status/draftR0 same; no field/schema/history rewrite. Verifier initial sheet1part assumption failed read-only; corrected by resolving actual workbook relationships, no weakened assertion or file modification.
- Screenshots: `header-fix/browser-source-desktop.jpg` source/button1280×720; `google-main-100pct.jpg` full populated sheet; `google-merged-cell-100pct.jpg` also exposes selectedB5K5 in namebox. SolHigh final independent audit24/24import-onlyPASS and3screenshots inspected; P2B5 resolved in testedfixture/no newP0–P2 or evidence overclaim, `header-fix/final-review.md`. Reviewer independently checked source/audit/container snapshots, did not rerun browser/runtime/Google or mutate data. Finalgofmt-l anddiffcheck clean. ImageCreated11:57:23ICT,backendStarted11:58:17ICT13October; current979hashes match that build.
- Limits remain: one nativev4 draftR0/one riskrow/Admin/desktop/nativeGoogle, user sign-off pending. Submitted/resolved/fullRev10/contextual/multirow/capacity/mobile/otherroles/Excel/print NOT_RUN. Pure tests cover short/long frozen-name presentation and other-template geometry, not entire runtime workflows. No Production/Gitdelivery/planclosure.

**Việc tiếp theo:** BM05 đã chốt; user yêu cầu lập kế hoạch BM06/BM07. Không tự bắt đầu implementation/browser của kế hoạch mới hoặc đóng plan/Git delivery.

## Bằng chứng trước sửa — được giữ nguyên

## Nguồn, môi trường và phạm vi

- Development `http://127.0.0.1:5173`, Admin; backend8081 `/api/ready` HTTP200, PostgreSQL `consflow`,158 migrations. HEAD `75dc44707785d399ed2408959bf784928a544e96` cùng working tree trước đó giữ nguyên.978 source hashes khớp retained build Medium `npm run build` PASS/Vite4.42s, completion observed10:56:48ICT13October. Không có application edit/rebuild/restart/Production/Git delivery trong UAT này.
- Project `DEV-BM-REUSE-20261009`, ID `d2398f6e-83db-4171-a729-fc2e2e1557d2`; Đầu việc `BM05 — Rủi ro synthetic`, ID `0e59ca7e-d8d3-4ded-b28d-67253cfda970`, `in_progress`.
- Form `dde65884-a1da-46ac-968d-6e1f83084185`, code `client_site_risk_assessment`, exact pinned v4 ID `96f6a564-c6f4-4ef1-96a3-4dc45a3fe9b0`, draftR0. Một risk row đã lưu đầy đủ; không nhập/sửa/chốt nguồn. Native export yêu cầu linked WI/quyền xem/project active/exact version và không cancelled/deleted; không bắt submitted. User được thông báo saved-draft boundary trước export.
- Đã đọc TOC/BM-GS-05 trực tiếp từ `/Users/khiemnguyen/Works/ttcons/documents/Bo_Bieu_Mau_Giam_Sat_Thu_Thiem.xlsx`:11cột A7:K7,3chỗ ký12–13,không có ô ảnh. Không tạo ảnh hoặc appendix cho nguồn0evidence. Không suy luận nghiệm thu rủi ro đã xử lý, hồ sơ đã chốt hoặc fullRev10.
- NOT_RUN: submitted/resolved source, contextual survey/daily BM05, multirow/capacity, otherroles/mobile/physicaldevices/Excel desktop/print. Bounded readback không có formula; không claim đã kiểm formula recalculation.

## File tải và dữ liệu

Actual browser mở exact WI và click **Xuất báo cáo** của WorkflowFormPanel, thấy toast `Đã tải biểu mẫu Excel`. Download `/Users/khiemnguyen/Downloads/BM-GS-05 (8).xlsx`; byte-equal copy `docs/acceptance-evidence/bm05-native-development-2026-10-13/BM05.xlsx`,10,824bytes,SHA256 `f94f422ac80d1dfdcea6a7a40095a6f12979416328703dd43908b0b1cd10e45f`.

Import nguyên bản vào private native Google Sheets ID `1kvOkszci_Qi11XKYm6zvmC51F3C8cwko2TytjZ0q1yY`, gid1792178762, tab `BM-GS-05-dde65884_01`. Confirm `converted:true`,nativeMIME,`shared:false`,owner-only. Metadata trước rồi đọc bounded `'BM-GS-05-dde65884_01'!A1:K15` gồm values/effective/errors/formats.165cells/33populated khớp XLSX,0effective errors,0#VALUE,0formula. Không sửa Google cell/style/sharing.

| Ô | Đối chiếu nguồn đã lưu |
|---|---|
| B5 | Project code + name đúng nguồn |
| A8,B8,C8 | STT1,SYN-RR-05,detected_date09-10-2026 |
| D8,E8,F8 | risk,cause,impact đúng saved row |
| G8 | medium→Trung bình, khớp pinned severity option |
| H8,I8,J8 | mitigation,owner,deadline10-10-2026 |
| K8 | Đang theo dõi synthetic; giữ nguyên trạng thái risk row, không thay bằng form draft hoặc WI in_progress |
| A7:K7,12–13 | 11headers và3signature placeholders đúng client; sample row9 được xóa nội dung |

Bundled Python `PYTHONDONTWRITEBYTECODE=1 …/python3 docs/acceptance-evidence/bm05-native-development-2026-10-13/verify.py`:23/23 evidence assertions PASS. Đây là kiểm tra mapping/file/baseline, không phải UX approval hoặc ứng dụng regression suite.2,892unique old business/workflow/construction/audit rows giữ nguyên/0lost; chỉ1native_source_exported audit mới;149storagefiles/978sourcehashes/V158 không đổi. Không thêm field/migration hoặc sửa lịch sử.

Screenshots `browser-source-desktop.jpg`1280×720 cho thấy nút/phiếu draftv4 và các risk fields bên trái; AX/sourceJSON xác nhận các cột ngoài scrollport. `google-main-100pct.jpg` hiển thị toàn bộ populated tab/footer ở Sheets100%, đồng thời chứng minh vấn đề B5. GPT-6.1 SolHigh đã chạy import-only verify23/23 và xem hai ảnh, kiểm thêm exact pin/published active/required/types/enums/dates/same-project/audit source+revision0+10824bytes. High xác nhận P2B5 và phạm vi DATA TECHNICALLY VERIFIED/NOT UX ACCEPTED; report `independent-review.md`. Reviewer không tự rerun browser/Google hoặc sửa ứng dụng/workbook/data.

## Review package: bố cục B5

**Mục đích chính:** người đọc nhận diện công trình và đọc sổ rủi ro11cột nhanh, đủ nội dung.

**Hiện tại:** B5 có đủ code/tên nhưng chỉ rộng12 Excel width units. C5:K5 trống; tên xuống nhiều dòng giữa từ, khiến hàng5 cao169.35pt. Risk row8 vẫn vừa ô46.5pt, không bị #VALUE hoặc mất dữ liệu.

**Nguyên nhân đã kiểm chứng:** template client B5 unmerged, width12 và không explicit wrap/height. `bmgsPopulate` ghi full project vào B5; loop5–7 gọi `bmgsFitRowText`. Helper546–618 cưỡng bức WrapText và tính height theo width của vùng merge hiện có; B5 không merge nên chỉ12. Đây là layout/wrap của XLSX trước import, không phải Google đổi dữ liệu.

**Đề xuất:** chỉ trong BM05 export mới, merge B5:K5 trước existing row-fit; dùng helper hiện có để tính height theo toàn vùng. Giữ nhãn A5,11cột risk/widths/headers,source/date/status và3chỗ ký. Không viết tên rút gọn/hardcode hoặc sửa Google sau tải. Code đề xuất chưa được duyệt.

```text
┌─────────────────────────────────────────────────────────────────────────┐
│ CÔNG TY TNHH ĐẦU TƯ XÂY DỰNG THỦ THIÊM                                   │
│ SỔ QUẢN LÝ VẤN ĐỀ, RỦI RO VÀ PHÁT SINH                                   │
├─────────┬───────────────────────────────────────────────────────────────┤
│ Dự án:  │ [DEV-BM-REUSE-20261009] [Synthetic Development] Nghiệm thu…     │
│ A5      │ B5:K5 — đủ tên, tự xuống dòng theo vùng rộng khi cần          │
├─────────┴───────────────────────────────────────────────────────────────┤
│ STT | Mã | Ngày phát hiện | Mô tả | … | Thời hạn | Trạng thái             │
│ 1   | SYN-RR-05 | 2026-10-09 | … | 2026-10-10 | Đang theo dõi synthetic  │
├─────────────────────────────────────────────────────────────────────────┤
│ Giám sát công trình | Phòng QS / Bộ phận liên quan | Quản lý dự án (PM)   │
└─────────────────────────────────────────────────────────────────────────┘
```

Ellipsis trong mockup chỉ thể hiện không gian minh họa; implementation phải giữ đủ nguyên tên. File đang review chưa merge/chỉnh height. Workbook không có loading/empty/error UI; source button loading/error/toast và source guards giữ nguyên. Narrow spreadsheet viewport vẫn dùng scrolling hiện có, không thu nhỏ font hoặc risk columns.

**Files dự kiến:** `backend/internal/core/bm_gs_workbook.go`, focused reopened-workbook regression trong `bm_gs_workbook_test.go`/`bm_gs_acceptance_renderer_test.go` theo owner hiện có; maps central/backend/triage và domain export được cập nhật nếu approved change diễn ra. Frontend/API/schema/data không cần đổi.

**Kế hoạch đề xuất1–3:**

1. Sửa vùng tên dự án B5:K5 riêng BM05 và dùng existing height-fit; giữ toàn bộ mapping/dữ liệu/lịch sử.
2. Regression với tên ngắn/dài: đủ nguyên tên, rowheight tính theo mergedwidth,11headers/riskcells/dates/status/signatures không đổi. Chạy focused backendtests và review SolHigh, cập nhật mapping; không test bằng class/string presence đơn thuần.
3. Cập nhật backend Development bằng pipeline hiện có phù hợp phạm vi; tải file mới từ đúng nút, import nguyên bản Google, đọc lại boundedvalues/errors, chụp100% layout và so baseline. Giữ cả file trước sửa và file mới để review; không patch workbook sau tải.

**Rủi ro/giới hạn:** đây là điều chỉnh khu metadata, cần anh duyệt mockup trước code. Tên dài hơn có thể cần nhiều dòng trong vùng rộng; không hứa mọi tên luôn một dòng. C5:K5 đang trống trong mẫu gốc; không có source field nào bị bỏ. Nếu layout của mẫu khác có metadata khác, không mở rộng merge tự động.

**Việc tiếp theo:** anh review file/hình và duyệt1–3 nếu chọn sửa B5. BM05 chưa chốt nghiệm thu hiển thị; BM06/BM07 còn chờ nghiệm thu riêng.
