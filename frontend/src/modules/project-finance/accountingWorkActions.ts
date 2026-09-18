import { useState } from 'react';
import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';
import type { AccountingWorkItem } from '../../lib/types/domain';
import type { AccountingHandoffDialog } from './AccountingHandoffActionDialog';

export async function invalidateAccountingWork(queryClient: QueryClient, projectId: string) {
  await Promise.all([
    ...['my-accounting-work', 'my-work-items', 'my-work-scope-summary', 'my-work-navigation-summary', 'project-finance', 'accounting-handoff-detail']
      .map((key) => queryClient.invalidateQueries({ queryKey: [key] })),
    queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] }),
  ]);
}

export function useAccountingWorkActions() {
  const [dialog, setDialog] = useState<AccountingHandoffDialog<AccountingWorkItem>>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const mutation = useMutation({
    mutationFn: ({ action, item, note, returnReason, externalReference }: {
      action: 'acknowledge' | 'return'; item: AccountingWorkItem;
      note: string; returnReason: string; externalReference: string;
    }) => post(`/accounting-handoffs/${item.handoff_id}/${action}`, {
      note, return_reason: returnReason, external_reference: externalReference,
    }),
    onSuccess: async (_result, variables) => {
      setDialog(null);
      await invalidateAccountingWork(queryClient, variables.item.project_id);
      toast.success(variables.action === 'acknowledge' ? 'Đã xác nhận tiếp nhận hồ sơ.' : 'Đã trả hồ sơ để bộ phận nguồn bổ sung.');
    },
    onError: async (error, variables) => {
      toast.error('Không cập nhật được hồ sơ kế toán', error);
      // Another accountant may have handled this shared record. Refresh both views.
      await invalidateAccountingWork(queryClient, variables.item.project_id);
    },
  });
  return { dialog, setDialog, mutation };
}
