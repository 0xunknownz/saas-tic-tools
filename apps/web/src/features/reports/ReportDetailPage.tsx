import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useSession } from '@0xunknownz/foundation-tenant-ui'
import { AppLink, EmptyState, PageHeading, Panel } from '../workbench/ui'
import {
  ArchiveError, Detail, Report, archiveStatusLabels, applyCandidate, archiveReport, confirmRevision, detectionLabels,
  formatBytes, getReport, getValidation, getCandidate, reparseReport, restoreReport, retryOCR, statusTone, updateReport,
  useArchiveFeed, voidReport,
} from './api'

const base = '/api/tenant/v1/report-archive'

const fieldLabels: Array<[string, string]> = [
  ['reportNumber', '报告编号'], ['reportName', '报告名称'], ['productName', '产品名称'], ['institution', '检测机构'],
  ['clientUnit', '委托单位'], ['inspectedUnit', '受检单位'], ['manufacturer', '生产单位'], ['model', '型号'],
  ['category', '检验类别'], ['basis', '检验依据'], ['reportDate', '报告日期'], ['conclusion', '检验结论'],
]

export function ReportDetailPage({ id }: { id: string }) {
  const session = useSession()
  const tenantId = session.session?.tenant?.id ?? ''
  const admin = session.session?.role === 'tenant_admin'
  const feed = useArchiveFeed(signal => getReport(id, signal), [id, tenantId], tenantId)
  const report = feed.data
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [actionNote, setActionNote] = useState<string | null>(null)
  const [form, setForm] = useState<Record<string, string>>({})
  const [details, setDetails] = useState<string>('[]')
  const [issues, setIssues] = useState<string[]>([])
  const [confirmed, setConfirmed] = useState<string[]>([])
  const [reason, setReason] = useState('')
  const [candidate, setCandidate] = useState<{ fields: Record<string, string>; details: Detail[]; issues: Array<{ code: string; message: string }> } | null>(null)

  useEffect(() => {
    if (!report || !editing) return
    const source = report as unknown as Record<string, unknown>
    const next: Record<string, string> = {}
    for (const [key] of fieldLabels) next[key] = String(source[key] ?? '')
    setForm(next)
    setDetails(JSON.stringify(report.details ?? [], null, 2))
  }, [report, editing])

  useEffect(() => {
    if (!report || !admin) return
    let cancelled = false
    getValidation(id).then(result => { if (!cancelled) setIssues(result.issues.map(issue => issue.code + '｜' + issue.message)) })
      .catch(() => { if (!cancelled) setIssues([]) })
    getCandidate(id).then(result => { if (!cancelled) setCandidate({ fields: result.fields, details: result.details, issues: result.issues }) })
      .catch(() => { if (!cancelled) setCandidate(null) })
    return () => { cancelled = true }
  }, [report, admin, id])

  async function saveEdit(event: FormEvent) {
    event.preventDefault()
    if (!report) return
    setSaving(true)
    setActionError(null)
    setActionNote(null)
    try {
      let parsedDetails
      try { parsedDetails = JSON.parse(details) } catch { throw new ArchiveError(400, 'DETAILS_JSON', '明细必须是有效的 JSON 数组') }
      await updateReport(id, { ...form, details: parsedDetails, expectedRevision: report.revision })
      setEditing(false)
      setActionNote('资料已保存。')
      feed.reload()
    } catch (error) {
      setActionError(error instanceof ArchiveError ? error.message : '保存失败，请重试')
    } finally {
      setSaving(false)
    }
  }

  async function runAction(fn: () => Promise<unknown>, success: string) {
    setActionError(null)
    setActionNote(null)
    setSaving(true)
    try {
      await fn()
      setActionNote(success)
      setReason('')
      feed.reload()
    } catch (error) {
      setActionError(error instanceof ArchiveError ? error.message : '操作失败，请重试')
    } finally {
      setSaving(false)
    }
  }

  if (feed.error && feed.error.status === 404) {
    return <Panel><EmptyState title="报告档案不存在"><p>该档案可能已被删除，或链接已失效。</p><AppLink className="tic-button" to="/reports">返回报告列表</AppLink></EmptyState></Panel>
  }
  if (feed.error) {
    return <Panel><EmptyState title="档案加载失败"><p role="alert">{feed.error.message}</p><button className="tic-button" onClick={feed.reload}>重试</button></EmptyState></Panel>
  }
  if (!report) return <Panel><p className="tic-muted" role="status">正在加载档案…</p></Panel>

  const fileHref = base + '/reports/' + encodeURIComponent(id)
  return <>
    <PageHeading title={report.reportName || '报告档案详情'} description="结构化资料、检测明细、原件与版本追溯。"
      action={<div className="tic-report-actions">
        <AppLink className="tic-button secondary" to="/reports">返回报告列表</AppLink>
        {admin ? <button className="tic-button secondary" onClick={() => { setEditing(!editing); setActionError(null); setActionNote(null) }}>{editing ? '取消编辑' : '编辑资料'}</button> : null}
        <a className="tic-button secondary" href={fileHref + '/download'}>下载原件</a>
      </div>} />
    {actionError ? <div className="tic-error" role="alert">{actionError}</div> : null}
    {actionNote ? <div className="tic-success" role="status">{actionNote}</div> : null}
    <div className="tic-detail tic-report-grid">
      <Panel title="档案状态">
        <p><span className={statusTone(report.archiveStatus)}>{archiveStatusLabels[report.archiveStatus] ?? report.archiveStatus}</span>{report.isCurrent ? <span className="tic-tag">当前有效版</span> : null}</p>
        <dl>
          <div><dt>检测结论</dt><dd>{detectionLabels[report.detectionResult] ?? report.detectionResult} · {report.conclusion || '未识别'}</dd></div>
          <div><dt>版本</dt><dd>v{report.reportVersion} · 记录修订号 {report.revision}</dd></div>
          <div><dt>创建人 / 时间</dt><dd>{report.createdBy} · {formatTime(report.createdAt)}</dd></div>
          <div><dt>原件</dt><dd>{report.file ? report.file.fileName + ' · ' + formatBytes(report.file.size) + ' · ' + report.file.pages + ' 页' : '—'}</dd></div>
          <div><dt>文件摘要</dt><dd className="tic-mono">{report.file?.sha256 ?? '—'}</dd></div>
          {report.ruleId ? <div><dt>解析规则</dt><dd>v{report.ruleVersion} · {report.ruleId}</dd></div> : null}
          {report.reviewReason ? <div><dt>处理原因</dt><dd>{report.reviewStage}：{report.reviewReason}</dd></div> : null}
          {report.voidReason ? <div><dt>作废原因</dt><dd>{report.voidReason}</dd></div> : null}
        </dl>
      </Panel>
      <Panel title="基本资料">
        {editing ? <form className="tic-form tic-report-form" onSubmit={saveEdit}>
          {fieldLabels.map(([key, label]) => <label key={key}>{label}<input value={form[key] ?? ''} onChange={event => setForm({ ...form, [key]: event.target.value })} /></label>)}
          <label>检测明细 JSON<textarea rows={10} value={details} onChange={event => setDetails(event.target.value)} /></label>
          <div className="tic-filter-actions"><button className="tic-button" disabled={saving}>{saving ? '保存中…' : '保存资料'}</button><button className="tic-button secondary" type="button" onClick={() => setEditing(false)}>取消</button></div>
        </form> : <dl>{fieldLabels.map(([key, label]) => <div key={key}><dt>{label}</dt><dd>{String((report as unknown as Record<string, unknown>)[key] ?? '') || '—'}{report.fieldSources?.[key] ? <small className="tic-muted"> 来源第 {report.fieldSources[key].page} 页</small> : null}</dd></div>)}</dl>}
      </Panel>
      <Panel title="检测明细" className="tic-span-all">
        {report.details && report.details.length > 0 ? <div className="tic-table-wrap" role="region" aria-label="检测明细，可横向滚动" tabIndex={0}>
          <table><thead><tr><th scope="col">序号</th><th scope="col">大项</th><th scope="col">子项</th><th scope="col">技术要求</th><th scope="col">单位</th><th scope="col">检验方法</th><th scope="col">结果原文</th><th scope="col">单项判定</th><th scope="col">来源页</th></tr></thead>
            <tbody>{report.details.map((detail, index) => <tr key={detail.id ?? index}>
              <td>{index + 1}</td><td>{detail.section}</td><td>{detail.itemName}</td><td>{detail.requirement}</td><td>{detail.unit}</td><td>{detail.method}</td><td>{detail.resultRaw}</td><td>{detail.judgement}</td><td>物理第 {detail.sourcePage} 页</td>
            </tr>)}</tbody></table>
        </div> : <EmptyState title="尚无结构化检测明细"><p>请等待 OCR 与规则解析，或由管理员对照原件补齐。</p></EmptyState>}
      </Panel>
      <Panel title="原件预览">
        {report.file ? <iframe className="tic-report-preview" title="报告原件预览" src={fileHref + '/file'} /> : <p>暂无原件。</p>}
        <p className="tic-muted">预览与下载均在服务端校验当前租户成员资格；物理页码用于定位明细来源。</p>
      </Panel>
      <Panel title="OCR 与处理进度">
        {report.ocr ? <dl>
          <div><dt>状态</dt><dd>{report.ocr.status} · {report.ocr.provider} / {report.ocr.model}</dd></div>
          {report.ocr.errorMessage ? <div><dt>失败原因</dt><dd>{report.ocr.errorCode}：{report.ocr.errorMessage}</dd></div> : null}
          {report.ocr.pages && report.ocr.pages.length > 0 ? <div><dt>识别页数</dt><dd>{report.ocr.pages.length} 页</dd></div> : null}
        </dl> : <p className="tic-muted">尚无 OCR 记录。</p>}
        {report.ocr?.text ? <details className="tic-ocr-text"><summary>查看 OCR 全文</summary><pre>{report.ocr.pages?.map(page => '—— 物理第 ' + page.page + ' 页 ——\n' + page.text).join('\n\n') ?? report.ocr.text}</pre></details> : null}
        {report.latestParse ? <p className="tic-muted">最近解析：{report.latestParse.status} · 规则版本 v{report.latestParse.ruleVersion}</p> : null}
      </Panel>
      {candidate && admin ? <Panel title="重解析候选" className="tic-span-all">
        <p className="tic-muted">候选不会覆盖当前档案，需管理员确认应用；应用前会保留修改前值。</p>
        <div className="tic-table-wrap" role="region" aria-label="候选资料" tabIndex={0}><table><thead><tr><th scope="col">字段</th><th scope="col">候选值</th></tr></thead><tbody>
          {Object.entries(candidate.fields).map(([key, value]) => <tr key={key}><td>{key}</td><td>{value}</td></tr>)}
        </tbody></table></div>
        <p>候选明细 {candidate.details.length} 条，问题 {candidate.issues.length} 条。</p>
        <button className="tic-button" disabled={saving} onClick={() => runAction(() => applyCandidate(id, report.revision), '候选已应用，请复核后归档。')}>应用候选并保留旧值</button>
      </Panel> : null}
      {admin ? <Panel title="管理员处理" className="tic-span-all">
        <div className="tic-report-actions">
          {report.archiveStatus === 'needs_review' || report.archiveStatus === 'archived' ? <button className="tic-button secondary" disabled={saving} onClick={() => runAction(() => reparseReport(id, reason || '按当前规则重解析'), '重解析任务已受理。')}>按当前规则重解析</button> : null}
          {['needs_review', 'pending_ocr', 'ocr_running', 'parsing'].includes(report.archiveStatus) ? <button className="tic-button secondary" disabled={saving} onClick={() => runAction(() => retryOCR(id, reason || '处理后重试 OCR'), 'OCR 重试任务已受理。')}>重试 OCR</button> : null}
          {report.archiveStatus === 'voided' ? <button className="tic-button" disabled={saving} onClick={() => runAction(() => restoreReport(id, reason), '档案已恢复。')}>恢复档案</button> : <button className="tic-button secondary" disabled={saving} onClick={() => runAction(() => voidReport(id, reason), '档案已作废。')}>作废档案</button>}
          {report.duplicateOf || issues.some(issue => issue.startsWith('REPORT_NUMBER_DUPLICATE')) ? <button className="tic-button" disabled={saving} onClick={() => runAction(() => confirmRevision(id, reason), '修订关系已确认。')}>确认修订关系</button> : null}
          <button className="tic-button" disabled={saving} onClick={() => runAction(() => archiveReport(id, reason, confirmed), '档案已人工归档。')}>人工归档</button>
        </div>
        <label className="tic-reason">处理原因（作废、恢复、归档、重解析、OCR 重试均需填写）<textarea rows={2} value={reason} onChange={event => setReason(event.target.value)} /></label>
        {issues.length > 0 ? <fieldset className="tic-issue-list"><legend>完整性异常（人工归档需逐项确认）</legend>
          {issues.map(issue => <label key={issue}><input type="checkbox" checked={confirmed.includes(issue.split('｜')[0])} onChange={event => {
            const code = issue.split('｜')[0]
            setConfirmed(previous => event.target.checked ? [...new Set([...previous, code])] : previous.filter(value => value !== code))
          }} /> {issue}</label>)}
        </fieldset> : <p className="tic-muted">服务端校验未发现额外完整性异常；必填与格式问题仍会阻止归档。</p>}
      </Panel> : null}
      <Panel title="版本关系" className="tic-span-all">
        {report.versions && report.versions.length > 0 ? <div className="tic-table-wrap" role="region" aria-label="版本列表" tabIndex={0}>
          <table><thead><tr><th scope="col">版本</th><th scope="col">报告编号</th><th scope="col">状态</th><th scope="col">当前</th><th scope="col">创建时间</th></tr></thead>
            <tbody>{report.versions.map(version => <tr key={version.id}><td>v{version.reportVersion}</td><td>{version.reportNumber}</td><td>{archiveStatusLabels[version.archiveStatus] ?? version.archiveStatus}</td><td>{version.isCurrent ? '是' : '否'}</td><td>{formatTime(version.createdAt)}</td></tr>)}</tbody></table>
        </div> : <p className="tic-muted">当前没有同编号的历史版本。</p>}
      </Panel>
      <Panel title="操作记录" className="tic-span-all">
        {report.events && report.events.length > 0 ? <ol className="tic-events">
          {report.events.map(event => <li key={event.id}><strong>{event.action}</strong> · {event.actor} · {formatTime(event.createdAt)}{event.reason ? <> — {event.reason}</> : null}</li>)}
        </ol> : <p className="tic-muted">暂无操作记录。</p>}
      </Panel>
    </div>
  </>
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}
