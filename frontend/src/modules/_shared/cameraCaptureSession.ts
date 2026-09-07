/** A canvas callback may finish after close, camera switch, or a newer capture. */
export function createCameraCaptureSession() {
  let generation = 0;
  let pending: number | null = null;
  return {
    begin() {
      if (pending !== null) return null;
      pending = ++generation;
      return pending;
    },
    finish(ticket: number) {
      if (pending !== ticket) return false;
      pending = null;
      return true;
    },
    invalidate() {
      generation++;
      pending = null;
    },
  };
}
