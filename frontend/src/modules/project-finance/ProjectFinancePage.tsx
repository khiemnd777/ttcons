import { AccountingHandoffPage } from './AccountingHandoffPage';

export function ProjectFinancePage({ global = false }: { global?: boolean }) {
  return <AccountingHandoffPage global={global} />;
}
