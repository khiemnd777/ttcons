import fs from "node:fs/promises";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputDir = "/Users/khiemnguyen/Works/ttcons/ttcons/outputs/vietnix-purchase-request";
const outputFile = `${outputDir}/De_nghi_mua_Vietnix_VPS_NVMe4_S3_Basic_12_thang.xlsx`;
const previewSummary = `${outputDir}/preview_de_nghi_mua.png`;
const previewDetails = `${outputDir}/preview_chi_tiet_ky_thuat.png`;

await fs.mkdir(outputDir, { recursive: true });

const workbook = Workbook.create();
const summary = workbook.worksheets.add("Đề nghị mua");
const details = workbook.worksheets.add("Chi tiết kỹ thuật");

const COLORS = {
  blue: "#0B57D0",
  lightBlue: "#E8F0FE",
  paleBlue: "#F5F8FE",
  dark: "#202124",
  gray: "#5F6368",
  lightGray: "#F1F3F4",
  border: "#DADCE0",
  green: "#137333",
  paleGreen: "#E6F4EA",
  amber: "#B06000",
  paleAmber: "#FEF7E0",
  input: "#FFF8D6",
  white: "#FFFFFF",
};

function styleSectionHeader(sheet, rangeAddress) {
  const range = sheet.getRange(rangeAddress);
  range.format = {
    fill: COLORS.lightGray,
    font: { bold: true, color: COLORS.dark },
    verticalAlignment: "center",
    borders: { preset: "outside", style: "thin", color: COLORS.border },
  };
}

function styleTableHeader(sheet, rangeAddress) {
  const range = sheet.getRange(rangeAddress);
  range.format = {
    fill: COLORS.lightBlue,
    font: { bold: true, color: COLORS.dark },
    horizontalAlignment: "center",
    verticalAlignment: "center",
    wrapText: true,
    borders: { preset: "all", style: "thin", color: COLORS.border },
  };
}

summary.showGridLines = false;
summary.mergeCells("A1:K1");
summary.getRange("A1").values = [["ĐỀ NGHỊ MUA DỊCH VỤ HẠ TẦNG VIETNIX"]];
summary.getRange("A1:K1").format = {
  fill: COLORS.lightBlue,
  font: { bold: true, color: COLORS.blue, size: 16 },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};
summary.getRange("A1:K1").format.rowHeight = 34;

summary.mergeCells("A2:K2");
summary.getRange("A2").values = [["VPS NVMe 4 + S3 Basic 100GB | Chu kỳ 12 tháng | Phục vụ triển khai Consflow"]];
summary.getRange("A2:K2").format = {
  font: { color: COLORS.gray, italic: true, size: 10 },
  horizontalAlignment: "center",
  verticalAlignment: "center",
};
summary.getRange("A2:K2").format.rowHeight = 22;

summary.getRange("A4:C8").values = [
  ["Nhà cung cấp", "", "Vietnix"],
  ["Ngày lập", "", new Date("2026-07-18T00:00:00+07:00")],
  ["Mục đích", "", "Triển khai hệ thống Consflow"],
  ["Trạng thái", "", "Chờ kế toán đặt mua"],
  ["Thuế VAT dự kiến", "", 0.1],
];
for (let row = 4; row <= 8; row += 1) summary.mergeCells(`A${row}:B${row}`);
summary.getRange("A4:B8").format = {
  fill: COLORS.lightGray,
  font: { bold: true, color: COLORS.dark },
  borders: { preset: "inside", style: "thin", color: COLORS.border },
};
summary.getRange("C4:C8").format = {
  fill: COLORS.white,
  wrapText: true,
  verticalAlignment: "center",
  borders: { preset: "inside", style: "thin", color: COLORS.border },
};
summary.getRange("C5").format.numberFormat = "yyyy-mm-dd";
summary.getRange("C8").format.numberFormat = "0%";
summary.getRange("A4:C8").format.rowHeight = 24;
summary.getRange("A6:C6").format.rowHeight = 34;

summary.mergeCells("D4:K4");
summary.getRange("D4").values = [["Phạm vi dự toán"]];
styleSectionHeader(summary, "D4:K4");
summary.mergeCells("D5:K8");
summary.getRange("D5").values = [[
  "Dự toán gồm đúng 2 dịch vụ: VPS NVMe 4 và S3 Basic 100GB, cùng chu kỳ 12 tháng. " +
  "Tổng ngân sách chưa trừ mã DE08; kế toán kiểm tra mã tại bước checkout và thanh toán theo số thực tế, không vượt mức ngân sách tối đa trong bảng."
]];
summary.getRange("D5:K8").format = {
  fill: COLORS.paleBlue,
  font: { color: COLORS.dark },
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};

summary.getRange("A10:K10").values = [[
  "STT",
  "Dịch vụ",
  "Gói",
  "Cấu hình chính",
  "Chu kỳ\n(tháng)",
  "Đơn giá/tháng\ntrước VAT",
  "Ưu đãi niêm yết",
  "Thành tiền\ntrước VAT",
  "VAT",
  "Tổng thanh toán",
  "Liên kết đặt mua",
]];
styleTableHeader(summary, "A10:K10");
summary.getRange("A10:K10").format.rowHeight = 40;

summary.getRange("A11:G12").values = [
  [1, "Cloud VPS", "VPS NVMe 4", "4 CPU AMD EPYC; RAM 8GB; NVMe 50GB", 12, 680000, "DE08: giảm thêm 8%*"],
  [2, "Object Storage", "S3 Basic", "100GB; S3 API; Network 100Mbps", 12, 90000, "Giá chu kỳ 1 năm"],
];
summary.getRange("H11:H12").formulas = [["=E11*F11"], ["=E12*F12"]];
summary.getRange("I11:I12").formulas = [["=H11*$C$8"], ["=H12*$C$8"]];
summary.getRange("J11:J12").formulas = [["=H11+I11"], ["=H12+I12"]];
summary.getRange("K11:K12").values = [
  ["https://vietnix.vn/vps-nvme/"],
  ["https://vietnix.vn/s3-object-storage/"],
];
summary.getRange("A11:K12").format = {
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "all", style: "thin", color: COLORS.border },
};
summary.getRange("A11:A12").format.horizontalAlignment = "center";
summary.getRange("E11:E12").format.horizontalAlignment = "center";
summary.getRange("A11:K12").format.rowHeight = 38;
summary.getRange("F11:J12").format.numberFormat = '#,##0 "đ"';

summary.getRange("A13:G13").merge();
summary.getRange("A13").values = [["TỔNG DỰ TOÁN"]];
summary.getRange("H13").formulas = [["=SUM(H11:H12)"]];
summary.getRange("I13").formulas = [["=SUM(I11:I12)"]];
summary.getRange("J13").formulas = [["=SUM(J11:J12)"]];
summary.getRange("K13").values = [["Ngân sách tối đa"]];
summary.getRange("A13:K13").format = {
  fill: COLORS.lightGray,
  font: { bold: true, color: COLORS.dark },
  verticalAlignment: "center",
  borders: { preset: "all", style: "thin", color: COLORS.border },
};
summary.getRange("H13:J13").format.numberFormat = '#,##0 "đ"';

summary.getRange("A15:C15").merge();
summary.getRange("A15").values = [["NGÂN SÁCH TỐI ĐA (GỒM VAT)"]];
summary.getRange("A16:C17").merge();
summary.getRange("A16").formulas = [["=J13"]];
summary.getRange("A15:C17").format = {
  fill: COLORS.paleBlue,
  font: { bold: true, color: COLORS.blue },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};
summary.getRange("A16:C17").format.font = { bold: true, color: COLORS.blue, size: 16 };
summary.getRange("A16:C17").format.numberFormat = '#,##0 "đ"';

summary.getRange("E15:G15").merge();
summary.getRange("E15").values = [["DỰ KIẾN SAU MÃ DE08*"]];
summary.getRange("E16:G17").merge();
summary.getRange("E16").formulas = [["=C24"]];
summary.getRange("E15:G17").format = {
  fill: COLORS.paleGreen,
  font: { bold: true, color: COLORS.green },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};
summary.getRange("E16:G17").format.font = { bold: true, color: COLORS.green, size: 16 };
summary.getRange("E16:G17").format.numberFormat = '#,##0 "đ"';

summary.getRange("I15:K15").merge();
summary.getRange("I15").values = [["TIẾT KIỆM DỰ KIẾN"]];
summary.getRange("I16:K17").merge();
summary.getRange("I16").formulas = [["=J13-C24"]];
summary.getRange("I15:K17").format = {
  fill: COLORS.paleAmber,
  font: { bold: true, color: COLORS.amber },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};
summary.getRange("I16:K17").format.font = { bold: true, color: COLORS.amber, size: 16 };
summary.getRange("I16:K17").format.numberFormat = '#,##0 "đ"';

summary.mergeCells("A19:K19");
summary.getRange("A19").values = [["Kịch bản mã DE08 — chỉ áp dụng nếu hệ thống Vietnix chấp nhận tại checkout"]];
styleSectionHeader(summary, "A19:K19");

summary.getRange("A20:C24").values = [
  ["Tỷ lệ giảm VPS", "", 0.08],
  ["VPS sau giảm (trước VAT)", "", null],
  ["Tổng 2 dịch vụ sau giảm (trước VAT)", "", null],
  ["VAT dự kiến", "", null],
  ["Tổng thanh toán dự kiến", "", null],
];
for (let row = 20; row <= 24; row += 1) summary.mergeCells(`A${row}:B${row}`);
summary.getRange("C21").formulas = [["=H11*(1-C20)"]];
summary.getRange("C22").formulas = [["=C21+H12"]];
summary.getRange("C23").formulas = [["=C22*$C$8"]];
summary.getRange("C24").formulas = [["=C22+C23"]];
summary.getRange("A20:C24").format = {
  borders: { preset: "all", style: "thin", color: COLORS.border },
  verticalAlignment: "center",
};
summary.getRange("A20:B24").format = {
  fill: COLORS.paleBlue,
  font: { bold: true, color: COLORS.dark },
  wrapText: true,
  verticalAlignment: "center",
  borders: { preset: "all", style: "thin", color: COLORS.border },
};
summary.getRange("A20:C24").format.rowHeight = 24;
summary.getRange("A22:C22").format.rowHeight = 36;
summary.getRange("C20").format = { fill: COLORS.input, numberFormat: "0%", borders: { preset: "all", style: "thin", color: COLORS.border } };
summary.getRange("C21:C24").format.numberFormat = '#,##0 "đ"';
summary.getRange("C24").format = {
  fill: COLORS.paleGreen,
  font: { bold: true, color: COLORS.green },
  numberFormat: '#,##0 "đ"',
  borders: { preset: "all", style: "thin", color: COLORS.border },
};

summary.mergeCells("D20:K24");
summary.getRange("D20").values = [[
  "* Mã DE08 và mức giảm 8% lấy từ ảnh VPS anh cung cấp. Không dùng số sau giảm làm hạn mức thanh toán chính thức trước khi mã được hệ thống chấp nhận. " +
  "Nếu mã không hợp lệ, tổng ngân sách tối đa gồm VAT là 10.164.000đ. Nếu VAT trên hóa đơn khác 10%, cập nhật ô C8 để bảng tự tính lại."
]];
summary.getRange("D20:K24").format = {
  fill: COLORS.paleAmber,
  font: { color: COLORS.dark },
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};

summary.mergeCells("A26:K26");
summary.getRange("A26").values = [["THÔNG TIN XUẤT HÓA ĐƠN — KẾ TOÁN ĐIỀN"]];
styleSectionHeader(summary, "A26:K26");

summary.getRange("A27:K29").values = [
  ["Tên công ty", "", "", "", "", "Mã số thuế", "", "", "", "", ""],
  ["Địa chỉ hóa đơn", "", "", "", "", "", "", "", "", "", ""],
  ["Email nhận hóa đơn", "", "", "", "", "Người phê duyệt", "", "", "", "", ""],
];
summary.mergeCells("A27:B27");
summary.mergeCells("C27:E27");
summary.mergeCells("F27:G27");
summary.mergeCells("H27:K27");
summary.mergeCells("A28:B28");
summary.mergeCells("C28:K28");
summary.mergeCells("A29:B29");
summary.mergeCells("C29:E29");
summary.mergeCells("F29:G29");
summary.mergeCells("H29:K29");
summary.getRange("A27:K29").format = {
  verticalAlignment: "center",
  borders: { preset: "all", style: "thin", color: COLORS.border },
};
summary.getRange("A27:B29").format = { fill: COLORS.lightGray, font: { bold: true }, borders: { preset: "all", style: "thin", color: COLORS.border } };
summary.getRange("F27:G29").format = { fill: COLORS.lightGray, font: { bold: true }, borders: { preset: "all", style: "thin", color: COLORS.border } };
summary.getRange("C27:E27").format.fill = COLORS.input;
summary.getRange("H27:K27").format.fill = COLORS.input;
summary.getRange("C28:K28").format.fill = COLORS.input;
summary.getRange("C29:E29").format.fill = COLORS.input;
summary.getRange("H29:K29").format.fill = COLORS.input;

summary.mergeCells("A31:K31");
summary.getRange("A31").values = [["Ghi chú cho kế toán"]];
styleSectionHeader(summary, "A31:K31");
summary.getRange("A32:A36").values = [[1], [2], [3], [4], [5]];
summary.getRange("B32:K36").merge(true);
summary.getRange("B32:B36").values = [
  ["Đặt VPS NVMe 4 và S3 Basic 100GB cùng chu kỳ 12 tháng."],
  ["Nhập mã DE08 khi thanh toán VPS; lưu ảnh checkout thể hiện mã được chấp nhận hoặc bị từ chối."],
  ["Đối chiếu tổng tiền với hạn mức 10.164.000đ gồm VAT trước khi thanh toán."],
  ["Yêu cầu hóa đơn VAT và gửi hóa đơn/biên nhận cho người phê duyệt."],
  ["Không ghi mật khẩu VPS, Access Key hoặc Secret Key S3 vào Google Sheet này."],
];
summary.getRange("A32:K36").format = {
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "inside", style: "thin", color: COLORS.border },
};
summary.getRange("A32:A36").format = { horizontalAlignment: "center", font: { bold: true, color: COLORS.blue } };

summary.getRange("A1:A36").format.columnWidth = 6;
summary.getRange("B1:B36").format.columnWidth = 20;
summary.getRange("C1:C36").format.columnWidth = 17;
summary.getRange("D1:D36").format.columnWidth = 29;
summary.getRange("E1:E36").format.columnWidth = 12;
summary.getRange("F1:F36").format.columnWidth = 18;
summary.getRange("G1:G36").format.columnWidth = 19;
summary.getRange("H1:H36").format.columnWidth = 18;
summary.getRange("I1:I36").format.columnWidth = 15;
summary.getRange("J1:J36").format.columnWidth = 19;
summary.getRange("K1:K36").format.columnWidth = 31;
summary.freezePanes.freezeRows(2);

details.showGridLines = false;
details.mergeCells("A1:E1");
details.getRange("A1").values = [["CHI TIẾT KỸ THUẬT VÀ NGUỒN GIÁ"]];
details.getRange("A1:E1").format = {
  fill: COLORS.lightBlue,
  font: { bold: true, color: COLORS.blue, size: 15 },
  horizontalAlignment: "center",
  verticalAlignment: "center",
  borders: { preset: "outside", style: "thin", color: COLORS.border },
};
details.getRange("A1:E1").format.rowHeight = 32;

details.getRange("A3:E3").values = [["Dịch vụ", "Thuộc tính", "Giá trị", "Ghi chú", "Nguồn"]];
styleTableHeader(details, "A3:E3");
details.getRange("A4:E17").values = [
  ["VPS NVMe 4", "CPU", "4 CPU AMD EPYC", "Theo ảnh lựa chọn", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "RAM", "8GB", "Theo ảnh lựa chọn", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "Ổ đĩa", "50GB NVMe", "Theo ảnh lựa chọn", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "Băng thông trong nước", "400 Mbps", "Không giới hạn Data Transfer", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "Băng thông quốc tế", "400Mbps Inbound / 50Mbps Outbound", "Theo ảnh lựa chọn", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "License", "DirectAdmin", "Được tặng kèm theo ảnh", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "Giá 12 tháng", "680.000đ/tháng", "Tổng trước VAT: 8.160.000đ", "https://vietnix.vn/vps-nvme/"],
  ["VPS NVMe 4", "Mã ưu đãi", "DE08 — giảm thêm 8%", "Chờ xác nhận tại checkout", "Ảnh người dùng cung cấp"],
  ["S3 Basic", "Dung lượng", "100GB", "Gói Basic", "https://vietnix.vn/s3-object-storage/"],
  ["S3 Basic", "Datacenter", "Mobifone", "Hạ tầng Việt Nam", "https://vietnix.vn/s3-object-storage/"],
  ["S3 Basic", "Tương thích", "S3 API", "Dùng cho ảnh/PDF của Consflow", "https://vietnix.vn/s3-object-storage/"],
  ["S3 Basic", "Network", "100Mbps", "Theo ảnh lựa chọn", "https://vietnix.vn/s3-object-storage/"],
  ["S3 Basic", "Data transfer / Request", "Unlimited / Unlimited", "Không tính thêm trong dự toán gói", "https://vietnix.vn/s3-object-storage/"],
  ["S3 Basic", "Giá 12 tháng", "90.000đ/tháng", "Tổng trước VAT: 1.080.000đ", "https://vietnix.vn/s3-object-storage/"],
];
details.getRange("A4:E17").format = {
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "all", style: "thin", color: COLORS.border },
};
details.getRange("A4:E17").format.rowHeight = 34;
details.getRange("A4:A11").format.fill = COLORS.paleBlue;
details.getRange("A12:A17").format.fill = COLORS.paleGreen;
details.getRange("A4:A17").format.font = { bold: true, color: COLORS.dark };

details.mergeCells("A19:E19");
details.getRange("A19").values = [["Lưu ý vận hành sau khi mua"]];
styleSectionHeader(details, "A19:E19");
details.getRange("A20:A23").values = [[1], [2], [3], [4]];
details.mergeCells("B20:E20");
details.mergeCells("B21:E21");
details.mergeCells("B22:E22");
details.mergeCells("B23:E23");
details.getRange("B20:B23").values = [
  ["Gửi IP VPS, thông tin tài khoản quản trị và endpoint S3 qua kênh bảo mật; không gửi bằng Sheet."],
  ["Bật backup dữ liệu ứng dụng và kiểm tra khôi phục định kỳ; không xem Object Storage là bản backup duy nhất."],
  ["Tạo bucket riêng cho ảnh/PDF và áp dụng quyền truy cập tối thiểu cho ứng dụng."],
  ["Theo dõi dung lượng S3 và tài nguyên VPS trong tháng đầu để quyết định nâng cấp nếu cần."],
];
details.getRange("A20:E23").format = {
  verticalAlignment: "center",
  wrapText: true,
  borders: { preset: "inside", style: "thin", color: COLORS.border },
};
details.getRange("A20:A23").format = { horizontalAlignment: "center", font: { bold: true, color: COLORS.blue } };

details.getRange("A1:A23").format.columnWidth = 18;
details.getRange("B1:B23").format.columnWidth = 25;
details.getRange("C1:C23").format.columnWidth = 34;
details.getRange("D1:D23").format.columnWidth = 35;
details.getRange("E1:E23").format.columnWidth = 39;
details.freezePanes.freezeRows(3);

const summaryCheck = await workbook.inspect({
  kind: "table",
  range: "'Đề nghị mua'!A1:K36",
  include: "values,formulas",
  tableMaxRows: 36,
  tableMaxCols: 11,
  maxChars: 12000,
});
console.log(summaryCheck.ndjson);

const formulaErrors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 100 },
  summary: "final formula error scan",
});
console.log(formulaErrors.ndjson);

const summaryPreview = await workbook.render({
  sheetName: "Đề nghị mua",
  range: "A1:K36",
  scale: 1.25,
  format: "png",
});
await fs.writeFile(previewSummary, new Uint8Array(await summaryPreview.arrayBuffer()));

const detailsPreview = await workbook.render({
  sheetName: "Chi tiết kỹ thuật",
  range: "A1:E23",
  scale: 1.25,
  format: "png",
});
await fs.writeFile(previewDetails, new Uint8Array(await detailsPreview.arrayBuffer()));

const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputFile);

console.log(JSON.stringify({ outputFile, previewSummary, previewDetails }, null, 2));
