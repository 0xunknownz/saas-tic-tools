import { FormEvent, useState } from 'react'
import { useSession } from '@0xunknownz/foundation-tenant-ui'
import { AppLink, EmptyState, PageHeading, Panel } from '../workbench/ui'
import { ArchiveError, Rule, TrialResult, activateRule, createRule, listRules, trialRule, useArchiveFeed } from './api'

export function ReportRulesPage() {
  const session = useSession()
  const tenantId = session.session?.tenant?.id ?? ''
  const admin = session.session?.role === 'tenant_admin'
  const feed = useArchiveFeed(signal => listRules(signal), [tenantId], tenantId)
  const rules = feed.data ?? []
  const [selectedId, setSelectedId] = useState('')
  const [editor, setEditor] = useState('')
  const [name, setName] = useState('')
  const [templateKey, setTemplateKey] = useState('')
  const [trialText, setTrialText] = useState('')
  const [trial, setTrial] = useState<TrialResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const selected = rules.find(rule => rule.id === selectedId) ?? null

  function select(rule: Rule) {
    setSelectedId(rule.id)
    setEditor(JSON.stringify(rule.config, null, 2))
    setName(rule.name)
    setTemplateKey(rule.templateKey)
    setError(null)
    setNote(null)
    setTrial(null)
  }

  async function copyFrom(source: Rule) {
    if (!admin) return
    setBusy(true)
    setError(null)
    setNote(null)
    try {
      const created = await createRule({ templateKey: source.templateKey, name: source.name + '（租户副本）', copyFrom: source.id })
      await feed.reload()
      select(created)
      setNote('已从内置模板创建租户草稿 v' + created.version + '。')
    } catch (err) { setError(err instanceof ArchiveError ? err.message : '创建失败') } finally { setBusy(false) }
  }

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!admin) return
    setBusy(true)
    setError(null)
    setNote(null)
    try {
      const config = JSON.parse(editor)
      const created = await createRule({ templateKey, name, config })
      await feed.reload()
      select(created)
      setNote('已保存为草稿 v' + created.version + '；历史版本保持可追溯。')
    } catch (err) {
      setError(err instanceof ArchiveError ? err.message : '配置必须是有效 JSON，且只能包含受约束的声明式字段')
    } finally { setBusy(false) }
  }

  async function activate() {
    if (!selected || !admin) return
    setBusy(true)
    setError(null)
    setNote(null)
    try {
      await activateRule(selected.id)
      await feed.reload()
      setNote('规则已启用；仅影响后续解析。')
    } catch (err) { setError(err instanceof ArchiveError ? err.message : '启用失败') } finally { setBusy(false) }
  }

  async function runTrial(event: FormEvent) {
    event.preventDefault()
    if (!selected) return
    setBusy(true)
    setError(null)
    setTrial(null)
    try {
      setTrial(await trialRule(selected.id, { text: trialText }))
      setNote('试解析完成；结果不会修改任何归档。')
    } catch (err) { setError(err instanceof ArchiveError ? err.message : '试解析失败') } finally { setBusy(false) }
  }

  if (!admin) return <Panel><EmptyState title="无权维护模板规则"><p>只有租户管理员可以维护规则；成员不能修改解析配置。</p><AppLink className="tic-button" to="/reports">返回报告列表</AppLink></EmptyState></Panel>

  return <>
    <PageHeading title="模板规则" description="平台维护内置模板，租户管理员复制并配置本租户规则；试解析不修改归档。"
      action={<AppLink className="tic-button secondary" to="/reports">返回报告列表</AppLink>} />
    {error ? <div className="tic-error" role="alert">{error}</div> : null}
    {note ? <div className="tic-success" role="status">{note}</div> : null}
    <div className="tic-report-grid">
      <Panel title="规则列表">
        {feed.error ? <div className="tic-error" role="alert">加载失败：{feed.error.message}<button className="tic-button secondary" onClick={feed.reload}>重试</button></div> : null}
        {rules.length === 0 && !feed.loading ? <EmptyState title="尚无规则"><p>请先复制内置模板。</p></EmptyState> : <ul className="tic-rule-list">
          {rules.map(rule => <li key={rule.id}>
            <button className={'tic-rule-item' + (rule.id === selectedId ? ' active' : '')} onClick={() => select(rule)}>
              <strong>{rule.name}</strong>
              <span>{rule.templateKey} · v{rule.version} · {rule.builtin ? '内置' : '租户'} · {rule.status}</span>
            </button>
            {rule.builtin ? <button className="tic-button secondary" disabled={busy} onClick={() => void copyFrom(rule)}>复制为租户草稿</button> : null}
          </li>)}
        </ul>}
      </Panel>
      <Panel title={selected ? selected.name : '规则配置'}>
        {!selected ? <p className="tic-muted">请选择左侧规则。内置模板只读；租户规则可保存为新的草稿版本并启用。</p> : <form className="tic-form" onSubmit={save}>
          <label>规则名称<input value={name} onChange={event => setName(event.target.value)} /></label>
          <label>模板标识<input value={templateKey} onChange={event => setTemplateKey(event.target.value)} /></label>
          <label>声明式配置 JSON<textarea rows={16} value={editor} onChange={event => setEditor(event.target.value)} /></label>
          <div className="tic-filter-actions">
            <button className="tic-button" disabled={busy || selected.builtin}>保存为新草稿版本</button>
            <button className="tic-button secondary" type="button" disabled={busy || selected.builtin || selected.status === 'active'} onClick={() => void activate()}>启用该版本</button>
          </div>
        </form>}
      </Panel>
      <Panel title="试解析" className="tic-span-all">
        <form className="tic-form" onSubmit={runTrial}>
          <label>粘贴 OCR 文本（或留空使用报告 OCR）<textarea rows={8} value={trialText} onChange={event => setTrialText(event.target.value)} /></label>
          <div className="tic-filter-actions"><button className="tic-button" disabled={busy || !selected}>运行试解析</button></div>
        </form>
        {trial ? <div className="tic-trial-result">
          <p>规则 v{trial.ruleVersion} · 问题 {trial.issues.length} 条 · 明细 {trial.details.length} 条</p>
          <div className="tic-table-wrap" role="region" aria-label="试解析字段" tabIndex={0}><table><thead><tr><th scope="col">字段</th><th scope="col">值</th></tr></thead><tbody>
            {Object.entries(trial.fields).map(([key, value]) => <tr key={key}><td>{key}</td><td>{value}</td></tr>)}
          </tbody></table></div>
          {trial.issues.length > 0 ? <ul className="tic-issue-list">{trial.issues.map((issue, index) => <li key={index}>{issue.code}：{issue.message}</li>)}</ul> : <p className="tic-success">未发现校验问题。</p>}
        </div> : null}
      </Panel>
    </div>
  </>
}
