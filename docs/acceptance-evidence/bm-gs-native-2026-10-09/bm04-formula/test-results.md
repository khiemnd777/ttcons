# BM-GS-04 formula regression — captured output

Captured from observed tool output on 2026-10-09. Tests were not rerun to create this record. Source was not changed when writing this record.

Environment: cwd `/Users/khiemnguyen/Works/ttcons/ttcons/backend`; Go `/usr/local/go/bin/go`; `GOPATH=/private/tmp/consflow-gopath`, `GOMODCACHE=/private/tmp/consflow-gomod`, `GOCACHE=/private/tmp/consflow-bmgs-go-cache`.

## Pre-fix FAIL — preserved verbatim

Command:

```sh
GOPATH=/private/tmp/consflow-gopath GOMODCACHE=/private/tmp/consflow-gomod GOCACHE=/private/tmp/consflow-bmgs-go-cache /usr/local/go/bin/go test ./internal/core -run '^TestBMGS04SummaryCountsExportedActions$' -count=1 -v
```

Exit code: 1. Captured command output, preserved verbatim:

```text
=== RUN   TestBMGS04SummaryCountsExportedActions
=== RUN   TestBMGS04SummaryCountsExportedActions/blank_code_with_unused_template_rows
    bm_gs_workbook_test.go:158: calculated exported summary: got "TỔNG HỢP: Tổng lỗi: 0 | Đã khắc phục: 0 | Đang xử lý: 1", want "TỔNG HỢP: Tổng lỗi: 1 | Đã khắc phục: 0 | Đang xử lý: 1"
=== RUN   TestBMGS04SummaryCountsExportedActions/all_codes_blank
    bm_gs_workbook_test.go:158: calculated exported summary: got "TỔNG HỢP: Tổng lỗi: 0 | Đã khắc phục: 1 | Đang xử lý: 2", want "TỔNG HỢP: Tổng lỗi: 3 | Đã khắc phục: 1 | Đang xử lý: 2"
=== RUN   TestBMGS04SummaryCountsExportedActions/mixed_codes_and_dynamic_rows
    bm_gs_workbook_test.go:158: calculated exported summary: got "TỔNG HỢP: Tổng lỗi: 3 | Đã khắc phục: 2 | Đang xử lý: 3", want "TỔNG HỢP: Tổng lỗi: 5 | Đã khắc phục: 2 | Đang xử lý: 3"
=== RUN   TestBMGS04SummaryCountsExportedActions/empty_register
--- FAIL: TestBMGS04SummaryCountsExportedActions (0.04s)
    --- FAIL: TestBMGS04SummaryCountsExportedActions/blank_code_with_unused_template_rows (0.02s)
    --- FAIL: TestBMGS04SummaryCountsExportedActions/all_codes_blank (0.01s)
    --- FAIL: TestBMGS04SummaryCountsExportedActions/mixed_codes_and_dynamic_rows (0.01s)
    --- PASS: TestBMGS04SummaryCountsExportedActions/empty_register (0.01s)
FAIL
FAIL	consflow/backend/internal/core	0.783s
FAIL
```

## Post-fix focused PASS — preserved verbatim

Command: same focused command shown above, executed after the formula fix and scoped gofmt.

Exit code: 0. Captured command output, preserved verbatim:

```text
=== RUN   TestBMGS04SummaryCountsExportedActions
=== RUN   TestBMGS04SummaryCountsExportedActions/blank_code_with_unused_template_rows
=== RUN   TestBMGS04SummaryCountsExportedActions/all_codes_blank
=== RUN   TestBMGS04SummaryCountsExportedActions/mixed_codes_and_dynamic_rows
=== RUN   TestBMGS04SummaryCountsExportedActions/empty_register
--- PASS: TestBMGS04SummaryCountsExportedActions (0.03s)
    --- PASS: TestBMGS04SummaryCountsExportedActions/blank_code_with_unused_template_rows (0.01s)
    --- PASS: TestBMGS04SummaryCountsExportedActions/all_codes_blank (0.01s)
    --- PASS: TestBMGS04SummaryCountsExportedActions/mixed_codes_and_dynamic_rows (0.01s)
    --- PASS: TestBMGS04SummaryCountsExportedActions/empty_register (0.01s)
PASS
ok  	consflow/backend/internal/core	0.461s
```

## Post-fix broader TestBMGS run — summary, not complete raw output

Command:

```sh
GOPATH=/private/tmp/consflow-gopath GOMODCACHE=/private/tmp/consflow-gomod GOCACHE=/private/tmp/consflow-bmgs-go-cache /usr/local/go/bin/go test ./internal/core -run '^TestBMGS' -count=1 -v
```

Observed exit code: 0. The tool truncated the middle of the output (305 lines; reported original token count 5921). This section is a summary; it does not reconstruct missing lines. The observed BMGS tests passed. The opt-in `TestBMGSQAFixtures` was skipped because `BMGS_QA_DIR` was not requested. Final observed result: package passed in 1.773s.

## Tested source identity and limits

HEAD: `75dc44707785d399ed2408959bf784928a544e96`, with existing tracked and untracked worktree changes. SHA256 observed after the post-fix broader run and before the focused rerun:

```text
1580fcd4e86e066023bf7eca5fd65aac32983f0db6967db75d676b02c20ede46  backend/internal/core/bm_gs_workbook.go
e6412fce7e785312e646dab0f8e74ab305655e4e7d3fe8b5f501dc0119f4157a  backend/internal/core/bm_gs_workbook_test.go
8828f0772f451edad19019f70918bc0f2a4425eb36f7afb7a3de6891ab9e8b84  backend/internal/core/bm_gs_native_projection_test.go
```

These are pure renderer/projection tests. The regression reopens generated XLSX bytes and calculates the exported summary with Excelize `CalcCellValue`. This record does not establish browser, native Excel/Google Sheets, database, migration or UX acceptance. No source/docs/maps were edited while capturing this record.
