# BM05 header — Independent High source review, 2026-10-13

**Giới hạn:** actual Development download/native Google sau sửa chưa được reviewer audit. Source/test review không đóng UX acceptance. Nguồn UAT vẫn là saved risk v4 draftR0; không chứng minh submitted/resolved/full Rev10. Reviewer không sửa application/maps, gọi DB/API, Docker hoặc browser; chỉ ghi review note này trong phạm vi được giao.

**Kết luận:** không có P0–P2 mới trong delta đã ổn định. So với `bm_gs_workbook-before.go.txt`, renderer chỉ thêm case BM-GS-05 với `MergeCell(name,"B5","K5")` trước write và existing merge-aware fit. Helper fit, source projection/service, frontend, fields/schema/history và các template khác không đổi.

## Evidence và kiểm tra

- Mandatory central/backend/triage và Construction Operations/Reporting maps đã đọc; owner `bm_gs_workbook.go::bmgsPopulate/bmgsFitRowText`, focused regression `bm_gs_risk_header_test.go`. First searches `BM-GS-05|MergeCell|bmgsFitRowText|ProjectHeaderMergedWorkbook`. Root sở hữu mapping maintenance và runtime/UAT.
- B5 chứa đúng project code/name qua selector hiện có. C5:K5 trống trong client/output baseline, không có metadata khác bị merge ghi đè. Merge không đổi A5 label hoặc11 risk column widths/headers. Điều kiện chỉ BM-GS-05; current/contextual/history BM05 đều đi qua cùng renderer, giữ selector/source semantics hiện có.
- Regression có RED trước sửa: thiếu merge; short65.4pt/long347.55pt. GREEN và independent rerun giữ tên ngắn/dài đầy đủ, row5 lần lượt33pt/35.7pt. Reopened XLSX checks xác nhận frozen historical project identity không bị live rename thay thế và input JSON không bị mutation.
- Các checks giữ11headers/column widths, risk code có số0 đầu, risk cells/status, numeric detected/due dates và ba chữ ký. Test thứ hai giữ merge geometry của bảy template còn lại. Đây là workbook behavior assertions, không chỉ tìm class/chuỗi source.
- Lệnh độc lập tại `backend/`: `GOCACHE=/private/tmp/consflow-bm05-review-go-cache GOMODCACHE=/private/tmp/consflow-bm02-diagnosis-modcache go test ./internal/core -run 'TestBMGS05(ProjectHeaderMergedWorkbook|HeaderRepairPreservesOtherTemplateMerges)$' -count=1 -v`.
- Kết quả thực tế: **PASS, 0.569s**; hai top-level tests, hai short/long subcases và bảy other-template subcases. Pure Go/reopened workbook, không integration DB hoặc browser. Cache task-specific, không sửa default/global cache.
- Đối chiếu978 existing source baseline: chỉ `backend/internal/core/bm_gs_workbook.go` đổi; thêm focused regression file mới. Existing dirty changes ngoài delta không được quy cho bản sửa này.
- Tested SHA256: renderer `f2eac21ee9ddd991182ac780f74949b413d9ed60babb306c6812ab1811adeff1`; regression `a9d2426394647099757dd97c182f8f0c3c4d82ab939cb11bfa78d3d284b30f00`.

**Next:** root build/update đúng Development source, kiểm actual download merge/row-height và bảo toàn source/data/history/file baselines; import nguyên file vào Google để đọc toàn bộ cells và review header/footer ở100%. User quyết định UX acceptance sau bằng chứng đó; reviewer không triển khai runtime.
