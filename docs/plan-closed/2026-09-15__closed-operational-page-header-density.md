# [Closed] Chuẩn hoá header vận hành ngoài không gian công trình

- Mục tiêu: đưa title/subtitle trên các trang cấp ứng dụng về density gọn đang dùng trong các trang chi tiết công trình, để ưu tiên không gian cho dữ liệu và thao tác.
- Đã làm: thống nhất `PageHeader` về title 18px/subtitle 12px và bỏ density lớn; đưa các header global tự dựng của Tài chính công trình, Hồ sơ bàn giao kế toán, Thống kê và Báo cáo về cùng hierarchy; cập nhật test presentation và frontend UI map.
- Xác minh kỹ thuật: `node --experimental-strip-types --test tests/projectWorkspaceCompactHeader.test.ts tests/projectFinanceProjectScope.test.ts tests/reportsResponsive.test.ts` đạt 9/9; `npm run build` đạt (TypeScript và Vite production build).
- Giới hạn còn lại: `npm test` đầy đủ còn 1 lỗi ngoài phạm vi tại `tests/projectEvidenceColumns.test.ts`, do test chờ cột `Giai đoạn` riêng nhưng `EvidenceGallery.tsx` hiện dùng nhãn `Nguồn / Giai đoạn`; không sửa vì không thuộc thay đổi header. Browser/UAT không được chạy, nên chưa UX accepted.
- Đóng plan: theo yêu cầu ngày 15-09-2026.
