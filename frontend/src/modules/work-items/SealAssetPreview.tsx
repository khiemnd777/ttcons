import { useEffect, useState } from 'react';
import { ImageOff, Loader2 } from 'lucide-react';
import clsx from 'clsx';
import { downloadBlob } from '../../lib/api/client';

export function SealAssetPreview({ path, alt, className }: { path: string; alt: string; className?: string }) {
  const [source, setSource] = useState<string>();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    let objectURL = '';
    setSource(undefined);
    setFailed(false);
    downloadBlob(path)
      .then((blob) => {
        if (!active) return;
        objectURL = URL.createObjectURL(blob);
        setSource(objectURL);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [path]);

  return (
    <div className={clsx('grid place-items-center overflow-hidden rounded-md border border-line bg-white', className)}>
      {source ? <img src={source} alt={alt} className="h-full w-full object-contain p-1" /> : null}
      {!source && !failed ? <Loader2 className="h-4 w-4 animate-spin text-slate-400" aria-label="Đang tải ảnh con dấu" /> : null}
      {failed ? <ImageOff className="h-4 w-4 text-rose-500" aria-label="Không tải được ảnh con dấu" /> : null}
    </div>
  );
}
