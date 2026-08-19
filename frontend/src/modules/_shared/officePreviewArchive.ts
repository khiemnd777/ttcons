export const MAX_OFFICE_PREVIEW_ARCHIVE_ENTRIES = 4_096;
export const MAX_OFFICE_PREVIEW_UNCOMPRESSED_BYTES = 100 * 1024 * 1024;
export const MAX_OFFICE_PREVIEW_ENTRY_BYTES = 50 * 1024 * 1024;

const ZIP_END_OF_CENTRAL_DIRECTORY_SIGNATURE = 0x06054b50;
const ZIP_CENTRAL_DIRECTORY_SIGNATURE = 0x02014b50;
const ZIP_MINIMUM_END_RECORD_BYTES = 22;
const ZIP_MAXIMUM_COMMENT_BYTES = 65_535;

export type OfficePreviewArchiveInfo = {
  entryCount: number;
  uncompressedBytes: number;
};

export function inspectOfficePreviewArchive(buffer: ArrayBuffer): OfficePreviewArchiveInfo {
  const view = new DataView(buffer);
  const endOffset = findEndOfCentralDirectory(view);
  if (endOffset < 0) throw new Error('Tệp Office không có cấu trúc ZIP hợp lệ.');

  const diskNumber = view.getUint16(endOffset + 4, true);
  const centralDirectoryDisk = view.getUint16(endOffset + 6, true);
  const diskEntryCount = view.getUint16(endOffset + 8, true);
  const entryCount = view.getUint16(endOffset + 10, true);
  const centralDirectorySize = view.getUint32(endOffset + 12, true);
  const centralDirectoryOffset = view.getUint32(endOffset + 16, true);
  if (diskNumber !== 0 || centralDirectoryDisk !== 0 || diskEntryCount !== entryCount) {
    throw new Error('Trình đọc không hỗ trợ tệp Office ZIP nhiều phân đoạn.');
  }
  if (entryCount === 0xffff || centralDirectorySize === 0xffffffff || centralDirectoryOffset === 0xffffffff) {
    throw new Error('Trình đọc không hỗ trợ tệp Office ZIP64.');
  }
  if (entryCount === 0 || entryCount > MAX_OFFICE_PREVIEW_ARCHIVE_ENTRIES) {
    throw new Error(`Tệp Office có quá nhiều thành phần để xem trước (tối đa ${MAX_OFFICE_PREVIEW_ARCHIVE_ENTRIES.toLocaleString('vi-VN')}).`);
  }
  const centralDirectoryEnd = centralDirectoryOffset + centralDirectorySize;
  if (centralDirectoryOffset < 0 || centralDirectoryEnd > endOffset || centralDirectoryEnd > buffer.byteLength) {
    throw new Error('Thư mục trung tâm của tệp Office không hợp lệ.');
  }

  let cursor = centralDirectoryOffset;
  let uncompressedBytes = 0;
  for (let index = 0; index < entryCount; index += 1) {
    if (cursor + 46 > centralDirectoryEnd || view.getUint32(cursor, true) !== ZIP_CENTRAL_DIRECTORY_SIGNATURE) {
      throw new Error('Thành phần ZIP của tệp Office không hợp lệ.');
    }
    const flags = view.getUint16(cursor + 8, true);
    const entryUncompressedBytes = view.getUint32(cursor + 24, true);
    const filenameLength = view.getUint16(cursor + 28, true);
    const extraLength = view.getUint16(cursor + 30, true);
    const commentLength = view.getUint16(cursor + 32, true);
    if ((flags & 0x0001) !== 0) throw new Error('Trình đọc không hỗ trợ tệp Office được mã hóa.');
    if (entryUncompressedBytes === 0xffffffff) throw new Error('Trình đọc không hỗ trợ thành phần ZIP64.');
    if (entryUncompressedBytes > MAX_OFFICE_PREVIEW_ENTRY_BYTES) {
      throw new Error('Một thành phần trong tệp Office quá lớn để xem trước an toàn.');
    }
    uncompressedBytes += entryUncompressedBytes;
    if (uncompressedBytes > MAX_OFFICE_PREVIEW_UNCOMPRESSED_BYTES) {
      throw new Error('Nội dung giải nén của tệp Office vượt quá giới hạn 100 MB của trình đọc.');
    }
    cursor += 46 + filenameLength + extraLength + commentLength;
  }
  if (cursor !== centralDirectoryEnd) throw new Error('Kích thước thư mục trung tâm của tệp Office không hợp lệ.');
  return { entryCount, uncompressedBytes };
}

function findEndOfCentralDirectory(view: DataView) {
  const latestStart = view.byteLength - ZIP_MINIMUM_END_RECORD_BYTES;
  const earliestStart = Math.max(0, view.byteLength - ZIP_MINIMUM_END_RECORD_BYTES - ZIP_MAXIMUM_COMMENT_BYTES);
  for (let offset = latestStart; offset >= earliestStart; offset -= 1) {
    if (view.getUint32(offset, true) !== ZIP_END_OF_CENTRAL_DIRECTORY_SIGNATURE) continue;
    const commentLength = view.getUint16(offset + 20, true);
    if (offset + ZIP_MINIMUM_END_RECORD_BYTES + commentLength === view.byteLength) return offset;
  }
  return -1;
}
