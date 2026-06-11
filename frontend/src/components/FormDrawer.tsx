import { ReactNode } from 'react';
import { Dialog } from './Dialog';

export function FormDrawer({
  open,
  title,
  children,
  onClose,
}: {
  open: boolean;
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  return (
    <Dialog open={open} title={title} onClose={onClose} size="lg">
      {children}
    </Dialog>
  );
}
