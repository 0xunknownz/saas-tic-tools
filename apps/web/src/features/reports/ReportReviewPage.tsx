import { useSession } from '@0xunknownz/foundation-tenant-ui'
import { AppLink, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import { ArchiveListFilter, archiveStatusLabels, listReports, statusTone, useArchiveFeed } from './api'
import { useState } from 'react'

export function ReportReviewPage() {
  const session = useSession()
  const tenantId = session.session?.tenant?.id ?? ''
  const admin = session.session?.role === 'tenant_admin'
  const [page, setPage] = useState(1)
  const filter: ArchiveListFilter = { scope: 'review', page, pageSize: 20 }
  const feed = useArchiveFeed(signal => listReports(filter, signal), [page, tenantId], tenantId)
  if (!admin) return <Panel><EmptyState title="无权访问待处理队列"><p>只有租户管理员可以处理异常档案。</p><AppLink className="tic-button" to="/reports">返回报告列表</AppLink></EmptyState></Panel>
  const items = feed.data?.items ?? []
  return <>
    <PageHeading title="待处理报告" description="展示处理阶段、原因与管理员操作入口。"
      action={<AppLink className="tic-button secondary" to="/reports">返回报告列表</AppLink>} />
    <Panel>
      {feed.error ? <div className="tic-error" role="alert">加载失败：{feed.error.message}<button className="tic-button secondary" onClick={feed.reload}>重试</button></div> : null}
      {items.length === 0 && !feed.loading ? <EmptyState title="没有待处理档案"><p>所有报告均已归档或尚无上传记录。</p></EmptyState> : null}
      {items.length > 0 ? <div className="tic-table-wrap" role="region" aria-label="待处理报告列表，可横向滚动" tabIndex={0}>
        <table><thead><tr><th scope="col">报告编号</th><th scope="col">报告名称</th><th scope="col">阶段</th><th scope="col">原因</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
          <tbody>{items.map(item => <tr key={item.id}>
            <td>{item.reportNumber || '待识别'}</td>
            <td>{item.reportName || '未命名档案'}</td>
            <td>{item.reviewStage || '—'}</td>
            <td>{item.reviewReason || '—'}</td>
            <td><span className={statusTone(item.archiveStatus)}>{archiveStatusLabels[item.archiveStatus] ?? item.archiveStatus}</span></td>
            <td><AppLink to={'/reports/' + item.id}>打开处理页</AppLink></td>
          </tr>)}</tbody></table>
      </div> : null}
      {feed.data ? <Pagination page={feed.data.page} total={feed.data.total} pageSize={feed.data.pageSize} onChange={setPage} /> : null}
    </Panel>
  </>
}
