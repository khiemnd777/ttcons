export async function openProtectedBlob(loadBlob: () => Promise<Blob>) {
  const viewer = window.open('about:blank', '_blank');
  if (!viewer) {
    throw new Error('Trình duyệt đã chặn cửa sổ mở tệp. Hãy cho phép pop-up cho trang này rồi thử lại.');
  }

  viewer.opener = null;
  viewer.document.title = 'Đang mở tệp…';
  viewer.document.body.textContent = 'Đang tải tệp…';

  try {
    const blob = await loadBlob();
    const objectUrl = URL.createObjectURL(blob);
    viewer.location.href = objectUrl;
    window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
  } catch (error) {
    viewer.close();
    throw error;
  }
}
