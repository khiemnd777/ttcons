export type ProtectedImagePreviewState =
  | { status: 'loading' }
  | { status: 'ready'; url: string }
  | { status: 'error'; message: string; retryable: boolean };

export function protectedImageError(error: unknown): Extract<ProtectedImagePreviewState, { status: 'error' }> {
  const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
  if (status === 401 || status === 403) {
    return { status: 'error', message: 'Bạn không có quyền xem dấu ký này.', retryable: false };
  }
  if (status === 404) {
    return { status: 'error', message: 'Không tìm thấy ảnh dấu ký của bản ghi này.', retryable: false };
  }
  return { status: 'error', message: 'Không tải được dấu ký.', retryable: true };
}

// The caller owns one preview lease. Disposing it suppresses late responses and
// releases the object URL, including when a record or authenticated actor changes.
export function startProtectedImagePreview(
  load: () => Promise<Blob>,
  onState: (state: ProtectedImagePreviewState) => void,
  urls: Pick<typeof URL, 'createObjectURL' | 'revokeObjectURL'> = URL,
) {
  let active = true;
  let objectURL: string | undefined;
  onState({ status: 'loading' });
  void (async () => {
    try {
      const blob = await load();
      if (!active) return;
      objectURL = urls.createObjectURL(blob);
      onState({ status: 'ready', url: objectURL });
    } catch (error) {
      if (active) onState(protectedImageError(error));
    }
  })();
  return () => {
    active = false;
    if (objectURL) {
      urls.revokeObjectURL(objectURL);
      objectURL = undefined;
    }
  };
}
