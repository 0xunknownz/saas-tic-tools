import { FormEvent, useEffect, useState } from 'react'
import { useSession } from '@0xunknownz/foundation-tenant-ui'
import { AppLink, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import {
  ArchiveError, ArchiveListFilter, archiveStatusLabels, detectionLabels, formatBytes, listReports,
  statusTone, useArchiveFeed,
} from './api'

const scopes = [
  { value: 'current', label: '当前有效' },
  { value: 'review', label: '待处理与处理中' },
  { value: 'history', label: '历史版本' },
  { value: 'voided', label: '已作废' },
  { value: 'all', label: '全部' },
]
const statuses = [
  { value: '', label: '全部状态' },
  { value: 'pending_ocr', label: '待 OCR' },
  { value: 'ocr_running', label: 'OCR 中' },
  { value: 'parsing', label: '规则解析' },
  { value: 'needs_review', label: '待处理' },
  { value: 'archived', label: '已归档' },
  { value: 'voided', label: '已作废' },
]

export function ReportListPage() {
  const session = useSession()
  const tenantId = session.session?.tenant?.id ?? ''
  const role = session.session?.role ?? ''
  const [filter, setFilter] = useState<ArchiveListFilter>({ scope: 'current', page: 1, pageSize: 5 })
  const [draft, setDraft] = useState<ArchiveListFilter>({ scope: 'current' })
  const feed = useArchiveFeed(signal => listReports(filter, signal), [JSON.stringify(filter), tenantId], tenantId)
  const patch = (values: Partial<ArchiveListFilter>) => {
    setFilter(previous => ({ ...previous, ...values, page: values.page ?? 1 }))
    setDraft(previous => ({ ...previous, ...values }))
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    setFilter({ ...draft, page: 1, pageSize: filter.pageSize ?? 5 })
  }
  const processing = feed.data?.items.some(item => ['pending_ocr', 'ocr_running', 'parsing'].includes(item.archiveStatus)) ?? false
  usePolling(processing, feed.reload, 4000)
  const admin = role === 'tenant_admin'
  return <>
    <PageHeading title="检测报告数字档案库" description="检索本租户报告档案、结构化明细与原件。"
      action={<div className="tic-report-actions">
        <AppLink className="tic-button" to="/reports/upload">上传报告</AppLink>
        {admin ? <AppLink className="tic-button secondary" to="/reports/review">待处理队列</AppLink> : null}
        {admin ? <AppLink className="tic-button secondary" to="/reports/rules">模板规则</AppLink> : null}
      </div>} />
    <Panel>
      <form className="tic-filters tic-report-filters" role="search" onSubmit={submit}>
        <label>关键词<input type="search" value={draft.query ?? ''} placeholder="报告编号、产品、机构、委托或受检单位" onChange={event => setDraft({ ...draft, query: event.target.value })} /></label>
        <label>检测机构<input type="search" value={draft.institution ?? ''} placeholder="检测机构" onChange={event => setDraft({ ...draft, institution: event.target.value })} /></label>
        <label>报告日期起<input type="date" value={draft.dateFrom ?? ''} onChange={event => setDraft({ ...draft, dateFrom: event.target.value })} /></label>
        <label>报告日期止<input type="date" value={draft.dateTo ?? ''} onChange={event => setDraft({ ...draft, dateTo: event.target.value })} /></label>
        <label>处理状态<select value={draft.status ?? ''} onChange={event => setDraft({ ...draft, status: event.target.value })}>{statuses.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}</select></label>
        <label>检测项目<input type="search" value={draft.item ?? ''} placeholder="例如 增塑剂" onChange={event => setDraft({ ...draft, item: event.target.value })} /></label>
        <label>单项判定<select value={draft.judgement ?? ''} onChange={event => setDraft({ ...draft, judgement: event.target.value })}><option value="">全部判定</option><option value="合格">合格</option><option value="不合格">不合格</option></select></label>
        <label>档案范围<select value={draft.scope ?? 'current'} onChange={event => setDraft({ ...draft, scope: event.target.value })}>{scopes.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}</select></label>
        <div className="tic-filter-actions"><button className="tic-button" type="submit">查询</button><button className="tic-button secondary" type="button" onClick={() => { const reset = { scope: 'current', page: 1, pageSize: 5 }; setDraft(reset); setFilter(reset) }}>重置</button></div>
      </form>
      {feed.error ? <div className="tic-error" role="alert"><p>加载失败：{feed.error.message}</p><button className="tic-button secondary" onClick={feed.reload}>重试</button></div> : null}
      {feed.loading && !feed.data ? <p className="tic-muted" role="status">正在加载档案…</p> : null}
      {feed.data && feed.data.items.length === 0 && !feed.loading ? <EmptyState title="没有匹配的报告档案"><p>请调整筛选条件，或上传检测报告。</p></EmptyState> : null}
      {feed.data && feed.data.items.length > 0 ? <div className="tic-table-wrap" role="region" aria-label="报告档案列表，可横向滚动" tabIndex={0}>
        <table><thead><tr>
          <th scope="col">报告编号</th><th scope="col">报告名称</th><th scope="col">检测机构</th><th scope="col">报告日期</th>
          <th scope="col">检测结论</th><th scope="col">归档状态</th><th scope="col">版本</th><th scope="col">档案</th>
        </tr></thead><tbody>
          {feed.data.items.map(item => <tr key={item.id}>
            <td>{item.reportNumber || '待识别'}</td>
            <td><AppLink to={'/reports/' + item.id}>{item.reportName || '未命名档案'}</AppLink></td>
            <td>{item.institution || '待识别'}</td>
            <td>{item.reportDate || '—'}</td>
            <td>{detectionLabels[item.detectionResult] ?? item.detectionResult}</td>
            <td><span className={statusTone(item.archiveStatus)}>{archiveStatusLabels[item.archiveStatus] ?? item.archiveStatus}</span>{item.reviewReason ? <small className="tic-muted"> {item.reviewStage}</small> : null}</td>
            <td>v{item.reportVersion}{item.isCurrent ? ' · 当前' : ''}</td>
            <td><AppLink to={'/reports/' + item.id} aria-label={'查看档案：' + (item.reportName || item.reportNumber)}>查看档案</AppLink></td>
          </tr>)}
        </tbody></table>
      </div> : null}
      {feed.data ? <Pagination page={feed.data.page} total={feed.data.total} pageSize={feed.data.pageSize} onChange={page => setFilter({ ...filter, page })} /> : null}
      {feed.data && feed.data.items.some(item => item.file) ? <p className="tic-muted">原件摘要与页数在档案详情中展示；当前列表仅显示元数据。</p> : null}
    </Panel>
  </>
}

function usePolling(active: boolean, reload: () => void, delay: number) {
  useEffect(() => {
    if (!active) return
    const timer = setInterval(reload, delay)
    return () => clearInterval(timer)
  }, [active, reload, delay])
}

export function reportFileLabel(size?: number, pages?: number): string {
  return [formatBytes(size), pages ? pages + ' 页' : ''].filter(Boolean).join(' · ')
}
