import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputDir = path.resolve("../../outputs/roadmap-sheet");
const outputPath = path.join(outputDir, "lo-trinh-trien-khai-tinh-nang-consflow.xlsx");

const phases = [
  {
    phase: "Phase 0",
    focus: "Xây dựng lõi hệ thống",
    dependency: "Không",
    defaultPriority: "Bắt buộc",
    goal: "Nền tảng kỹ thuật, dữ liệu, phân quyền, module lõi và workflow core.",
    completion: "Đăng nhập/phân quyền, dữ liệu lõi, admin UI và workflow core sẵn sàng mở rộng.",
    items: [
      ["Nền tảng hệ thống", "Nền tảng", "Thiết lập khung hệ thống API, database, cache, Docker và cấu trúc module.", "Nền kỹ thuật ổn định để mở rộng các module nghiệp vụ."],
      ["Dữ liệu lõi", "Dữ liệu lõi", "Định nghĩa dữ liệu nền tảng cho dự án, nhân sự, đối tác, vật tư và danh mục dùng chung.", "Dữ liệu dùng chung có cấu trúc thống nhất và sẵn sàng tái sử dụng."],
      ["Auth / RBAC", "Phân quyền Auth / RBAC", "Xây dựng đăng nhập, vai trò, quyền hạn và bảo vệ route/API.", "Người dùng chỉ thao tác đúng phạm vi quyền được cấp."],
      ["Lõi module nghiệp vụ", "Lõi module nghiệp vụ", "Chuẩn hoá cách khai báo module, route, service, repository và contract FE/API.", "Các module mới có khuôn triển khai nhất quán."],
      ["Admin UX", "Chuẩn hoá trải nghiệm Admin", "Thiết lập layout, navigation, feedback, loading, empty và error state cho admin.", "Trải nghiệm vận hành nhất quán trên toàn bộ admin."],
      ["Shared UI", "Thành phần giao diện dùng chung", "Xây dựng bảng, modal, form, toast, validation, filter và action pattern dùng chung.", "Giảm lặp lại UI và giữ hành vi thao tác đồng nhất."],
      ["Business UI", "Thành phần giao diện nghiệp vụ", "Tạo các component phục vụ workflow, checklist, báo cáo, chữ ký, hình ảnh và chứng từ.", "Có bộ UI nghiệp vụ sẵn sàng dùng lại ở các phase sau.", "Cao"],
      ["Workflow Core", "Cơ chế chuyển hoá Workflow thành nghiệp vụ", "Ánh xạ workflow template/run thành các nghiệp vụ cụ thể và trạng thái vận hành.", "Workflow có thể trở thành đơn vị nghiệp vụ có thể thao tác, theo dõi và audit."],
    ],
  },
  {
    phase: "Phase 1",
    focus: "Nghiệp vụ kế hoạch",
    dependency: "Phase 0",
    defaultPriority: "Cao",
    goal: "Số hoá nghiệp vụ kế hoạch, báo cáo ngày và các form minh chứng.",
    completion: "Người dùng tạo và quản lý được nghiệp vụ kế hoạch qua giao diện.",
    items: [
      ["Workflow nghiệp vụ", "Giao diện hoá các loại nghiệp vụ", "Hiển thị và thao tác các nghiệp vụ kế hoạch qua giao diện quản trị.", "Người dùng xử lý nghiệp vụ kế hoạch mà không cần cấu hình thô."],
      ["Workflow nghiệp vụ", "Hệ quy đổi Workflow thành nghiệp vụ", "Chuyển đổi bước workflow thành loại form, quy tắc và trạng thái nghiệp vụ.", "Mỗi workflow có đầu ra nghiệp vụ rõ ràng."],
      ["Báo cáo hằng ngày", "Cơ chế Báo cáo theo ngày (Đã và Sẽ Làm)", "Ghi nhận công việc đã làm và sẽ làm theo ngày cho giai đoạn kế hoạch.", "Báo cáo tiến độ ngày có thể lọc theo dự án, người phụ trách và ngày."],
      ["Form nghiệp vụ", "Form Biên Bản Họp", "Tạo form ghi nhận nội dung họp, thành phần, kết luận và việc cần làm.", "Biên bản họp được lưu, tìm kiếm và liên kết với dự án/workflow.", "Trung bình"],
      ["Form nghiệp vụ", "Form Báo cáo nghiệp vụ dạng Checklist", "Tạo form checklist cho các đầu việc nghiệp vụ kế hoạch.", "Checklist có trạng thái, người phụ trách và bằng chứng hoàn thành."],
      ["Form nghiệp vụ", "Form Báo cáo nghiệp vụ dạng Chữ ký ảo", "Cho phép xác nhận/ký số nội bộ trên báo cáo nghiệp vụ.", "Báo cáo có dấu xác nhận, actor và thời điểm rõ ràng.", "Trung bình"],
      ["Form nghiệp vụ", "Form Báo cáo nghiệp vụ dạng Hình ảnh", "Cho phép đính kèm hình ảnh minh chứng cho báo cáo nghiệp vụ.", "Hình ảnh được gắn với đúng báo cáo, người gửi và thời điểm.", "Trung bình"],
      ["Đơn giá", "Form Báo cáo nghiệp vụ dạng Đơn Giá", "Quản lý đơn giá theo các trạng thái: dự kiến, chốt định mức, chốt đơn.", "Theo dõi được vòng đời đơn giá từ dự kiến đến chốt chính thức."],
    ],
  },
  {
    phase: "Phase 2",
    focus: "Nghiệp vụ thi công",
    dependency: "Phase 1",
    defaultPriority: "Cao",
    goal: "Vận hành thi công: checklist, đối soát, vật tư, kho, nghiệm thu, hoá đơn và thanh toán.",
    completion: "Theo dõi được tiến độ, vật tư, nghiệm thu, hoá đơn và thanh toán hai chiều.",
    items: [
      ["Checklist thi công", "Form Checklist Thầu Phụ", "Tạo checklist thi công dành cho thầu phụ.", "Thầu phụ có đầu mục công việc, trạng thái và bằng chứng thực hiện."],
      ["Checklist thi công", "Form Checklist Chủ Đầu Tư", "Tạo checklist xác nhận với chủ đầu tư.", "Chủ đầu tư có checklist nghiệm thu/xác nhận riêng."],
      ["Báo cáo hằng ngày", "Cơ chế báo cáo mỗi ngày (Đã và Sẽ Làm)", "Ghi nhận tiến độ thi công mỗi ngày theo công trình và đội thi công.", "Báo cáo ngày phản ánh đúng việc đã làm và kế hoạch tiếp theo."],
      ["Đối soát checklist", "Cơ chế đối soát Checklist Thầu Phụ", "So sánh checklist thầu phụ với khối lượng, thực tế và xác nhận nội bộ.", "Phát hiện chênh lệch và trạng thái cần xử lý."],
      ["Đối soát checklist", "Cơ chế đối soát Checklist Chủ Đầu Tư", "So sánh checklist chủ đầu tư với tiến độ và hồ sơ nội bộ.", "Có căn cứ đối soát đầu ra với chủ đầu tư."],
      ["Vật tư và kho", "Form Báo cáo Vật Tư (trước và sau)", "Ghi nhận vật tư trước thi công và sau thi công.", "Theo dõi chênh lệch vật tư trước/sau theo hạng mục."],
      ["Vật tư và kho", "Form Lưu Kho Vật Tư Tồn", "Quản lý vật tư tồn và lưu kho sau thi công.", "Vật tư tồn có vị trí, số lượng và lịch sử ghi nhận."],
      ["Nghiệm thu", "Form Nghiệm thu Đầu Vào", "Ghi nhận nghiệm thu đầu vào từ nhà cung cấp/thầu phụ.", "Nghiệm thu đầu vào có trạng thái, bằng chứng và người xác nhận."],
      ["Nghiệm thu", "Form Nghiệm thu Đầu Ra", "Ghi nhận nghiệm thu đầu ra với chủ đầu tư.", "Nghiệm thu đầu ra có trạng thái, bằng chứng và người xác nhận."],
      ["Hoá đơn PDF", "Import / Export PDF hoá đơn cho Đầu Vào", "Nhập và xuất PDF hoá đơn đầu vào gắn với nghiệp vụ tương ứng.", "Hoá đơn đầu vào được quản lý như chứng từ có liên kết.", "Trung bình"],
      ["Hoá đơn PDF", "Import / Export PDF hoá đơn cho Đầu Ra", "Nhập và xuất PDF hoá đơn đầu ra gắn với nghiệp vụ tương ứng.", "Hoá đơn đầu ra được quản lý như chứng từ có liên kết.", "Trung bình"],
      ["Thanh toán", "Form Thanh toán theo Kỳ / Ứng trước cho Đầu Vào", "Theo dõi thanh toán theo đợt và ứng trước cho đầu vào.", "Kiểm soát được giá trị đã ứng, đã thanh toán và còn lại."],
      ["Thanh toán", "Form Thanh toán theo Kỳ / Ứng trước cho Đầu Ra", "Theo dõi thanh toán theo đợt và ứng trước cho đầu ra.", "Kiểm soát được giá trị đã thu, đã xuất và còn lại."],
    ],
  },
  {
    phase: "Phase 3",
    focus: "Kế toán",
    dependency: "Phase 2",
    defaultPriority: "Cao",
    goal: "Tổng hợp công nợ, dòng tiền, thuế và lợi nhuận công trình.",
    completion: "Báo cáo tài chính công trình có số liệu từ các phase trước.",
    items: [
      ["Công nợ", "Form Báo cáo Công / Nợ Đầu Vào", "Báo cáo công nợ phải trả cho đầu vào.", "Theo dõi công nợ đầu vào theo công trình, đối tác và kỳ thanh toán."],
      ["Công nợ", "Form Báo cáo Công / Nợ Đầu Ra", "Báo cáo công nợ phải thu cho đầu ra.", "Theo dõi công nợ đầu ra theo công trình, chủ đầu tư và kỳ thanh toán."],
      ["Dòng tiền", "Form Báo cáo dòng tiền (Thu / Chi)", "Tổng hợp dòng tiền thu chi theo công trình và thời gian.", "Quản trị được dòng tiền thực tế của từng công trình."],
      ["Thuế", "Form Báo cáo thuế", "Tổng hợp số liệu phục vụ báo cáo thuế.", "Dữ liệu thuế được tổng hợp theo đúng kỳ và nguồn chứng từ.", "Trung bình"],
      ["Lợi nhuận", "Form Báo cáo lợi nhuận sau khi trừ chi phí công trình", "Tính lợi nhuận sau khi trừ chi phí trực tiếp và chi phí liên quan công trình.", "Có báo cáo lợi nhuận cuối cùng phục vụ quản trị."],
    ],
  },
];

const headers = [
  "Phase",
  "Trọng tâm",
  "Nhóm hạng mục",
  "Tính năng / hạng mục",
  "Mô tả triển khai",
  "Kết quả đầu ra",
  "Ưu tiên",
  "Trạng thái",
  "Phụ thuộc",
  "Owner",
  "Thời gian dự kiến",
  "Ghi chú",
];

const roadmapRows = phases.flatMap((phase) =>
  phase.items.map(([group, item, scope, output, priority]) => [
    phase.phase,
    phase.focus,
    group,
    item,
    scope,
    output,
    priority ?? phase.defaultPriority,
    "Chưa bắt đầu",
    phase.dependency,
    "",
    "",
    "",
  ]),
);

const summaryRows = phases.map((phase) => {
  const total = phase.items.length;
  return [phase.phase, phase.focus, total, 0, 0, "Chưa bắt đầu", phase.goal, phase.completion];
});

function setValues(sheet, startCell, rows) {
  if (!rows.length) return;
  const startCol = startCell.match(/[A-Z]+/)[0];
  const startRow = Number(startCell.match(/\d+/)[0]);
  const width = rows[0].length;
  const height = rows.length;
  const endCol = columnName(columnIndex(startCol) + width - 1);
  const endRow = startRow + height - 1;
  sheet.getRange(`${startCell}:${endCol}${endRow}`).values = rows;
}

function columnIndex(name) {
  return name.split("").reduce((acc, char) => acc * 26 + char.charCodeAt(0) - 64, 0);
}

function columnName(index) {
  let name = "";
  while (index > 0) {
    const mod = (index - 1) % 26;
    name = String.fromCharCode(65 + mod) + name;
    index = Math.floor((index - mod) / 26);
  }
  return name;
}

function tryFormat(fn) {
  try {
    fn();
  } catch (error) {
    console.warn(`format skipped: ${error.message}`);
  }
}

async function saveRender(workbook, sheetName, range) {
  try {
    const rendered = await workbook.render({ sheetName, range, scale: 1 });
    const out = path.join(outputDir, `${sheetName.replaceAll(" ", "-").toLowerCase()}.png`);
    if (Buffer.isBuffer(rendered) || rendered instanceof Uint8Array) {
      await fs.writeFile(out, rendered);
    } else if (rendered?.data && (Buffer.isBuffer(rendered.data) || rendered.data instanceof Uint8Array)) {
      await fs.writeFile(out, rendered.data);
    } else if (typeof rendered?.arrayBuffer === "function") {
      await fs.writeFile(out, Buffer.from(await rendered.arrayBuffer()));
    } else {
      console.warn(`render completed for ${sheetName}, but returned unsupported output shape`);
    }
  } catch (error) {
    console.warn(`render skipped for ${sheetName}: ${error.message}`);
  }
}

const workbook = Workbook.create();
const roadmap = workbook.worksheets.add("Roadmap");
const summary = workbook.worksheets.add("Phase Summary");

setValues(roadmap, "A1", [["Lộ trình triển khai tính năng Consflow"]]);
setValues(roadmap, "A3", [headers, ...roadmapRows]);

setValues(summary, "A1", [["Tổng quan phase triển khai"]]);
setValues(summary, "A3", [[
  "Phase",
  "Tên phase",
  "Tổng hạng mục",
  "Đã hoàn thành",
  "Tỷ lệ hoàn thành",
  "Trạng thái tổng",
  "Mục tiêu",
  "Điều kiện hoàn thành",
]]);
setValues(summary, "A4", summaryRows);

tryFormat(() => {
  roadmap.getRange("A1:L1").format.font.bold = true;
  roadmap.getRange("A1:L1").format.font.size = 14;
  roadmap.getRange("A3:L3").format.font.bold = true;
  roadmap.getRange(`A3:L${roadmapRows.length + 3}`).format.wrapText = true;
  summary.getRange("A1:H1").format.font.bold = true;
  summary.getRange("A1:H1").format.font.size = 14;
  summary.getRange("A3:H3").format.font.bold = true;
  summary.getRange("E4:E7").numberFormat = "0%";
});

await fs.mkdir(outputDir, { recursive: true });

const inspectRoadmap = await workbook.inspect({
  kind: "table",
  range: "Roadmap!A1:L12",
  include: "values,formulas",
  tableMaxRows: 12,
  tableMaxCols: 12,
});
console.log(inspectRoadmap.ndjson);

const formulaErrors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 50 },
  summary: "final formula error scan",
});
console.log(formulaErrors.ndjson);

await saveRender(workbook, "Roadmap", "A1:L18");
await saveRender(workbook, "Phase Summary", "A1:H8");

const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputPath);
console.log(`saved:${outputPath}`);
