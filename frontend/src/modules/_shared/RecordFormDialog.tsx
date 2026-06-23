import type { DialogSize } from '../../components/Dialog';
import { Dialog } from '../../components/Dialog';
import { SimpleForm, type FieldDef, type FormLayout } from './SimpleForm';

type FormValues = Record<string, any>;

export function RecordFormDialog({
  open,
  title,
  description,
  fields,
  submitLabel = 'Lưu',
  cancelLabel,
  layout = 'single',
  resetOnSubmit = false,
  onSubmit,
  onClose,
  size = 'lg',
}: {
  open: boolean;
  title: string;
  description?: string;
  fields: FieldDef[];
  submitLabel?: string;
  cancelLabel?: string;
  layout?: FormLayout;
  resetOnSubmit?: boolean;
  onSubmit: (values: FormValues) => Promise<unknown> | unknown;
  onClose: () => void;
  size?: DialogSize;
}) {
  return (
    <Dialog open={open} title={title} description={description} onClose={onClose} size={size}>
      <SimpleForm
        fields={fields}
        submitLabel={submitLabel}
        cancelLabel={cancelLabel}
        layout={layout}
        resetOnSubmit={resetOnSubmit}
        onSubmit={onSubmit}
        onCancel={onClose}
      />
    </Dialog>
  );
}
