import { AppLink, EmptyState, Panel } from '../workbench/ui'
import { ReportDetailPage } from './ReportDetailPage'
import { ReportListPage } from './ReportListPage'
import { ReportReviewPage } from './ReportReviewPage'
import { ReportRulesPage } from './ReportRulesPage'
import { ReportUploadPage } from './ReportUploadPage'

export function ReportsPage({ path }: { path: string }) {
  if (path === '/reports') return <ReportListPage />
  if (path === '/reports/upload') return <ReportUploadPage />
  if (path === '/reports/review') return <ReportReviewPage />
  if (path === '/reports/rules') return <ReportRulesPage />
  if (path.startsWith('/reports/')) {
    const id = path.slice('/reports/'.length)
    if (!id || id.includes('/')) return <Panel><EmptyState title="报告档案不存在"><p>请检查链接或返回列表。</p><AppLink className="tic-button" to="/reports">返回报告列表</AppLink></EmptyState></Panel>
    return <ReportDetailPage id={id} />
  }
  return <Panel><EmptyState title="页面未找到"><p>没有对应的报告档案页面。</p><AppLink className="tic-button" to="/reports">返回报告列表</AppLink></EmptyState></Panel>
}
