import { CalendarDays } from 'lucide-react';
import { BannerLabel, useBannerWordless } from '../../components/Wordless';
import type { ConstructionDailyReportSource } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { constructionReportWeatherLabel } from './constructionWeatherPresentation';

/** Source identity is shared; actual evidence reuse remains owned by each caller. */
export function SourceReportContextBanner({ report, variant }: { report: ConstructionDailyReportSource; variant: 'check' | 'action' }) {
  const wordless = useBannerWordless('operations');
  const evidenceText = variant === 'check' ? `Dùng lại ${report.evidence.length} minh chứng hiện trạng` : `Có ${report.evidence.length} minh chứng có thể tái sử dụng`;
  return <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4"><span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-white text-blue-600"><CalendarDays className="h-5 w-5" /></span><div>
    {!wordless ? <p className="text-xs font-semibold uppercase tracking-wide text-blue-600">Nguồn đã liên kết</p> : null}
    <p className="mt-1 font-semibold text-blue-900"><BannerLabel surface="operations" text={`Báo cáo ngày ${dateText(report.report_date)}`} help={<>{constructionReportWeatherLabel(report.weather)} · {variant === 'check' ? 'Kiểm tra' : 'Việc cần xử lý'} được truy vết về báo cáo hiện trường nguồn.</>} /></p>
    {wordless ? report.evidence.length > 0 ? <p className="mt-1 text-xs text-blue-700">{evidenceText}</p> : null : <p className="mt-1 text-xs text-blue-700">{constructionReportWeatherLabel(report.weather)}{report.evidence.length ? ` · ${evidenceText}` : ''}</p>}
  </div></div>;
}
