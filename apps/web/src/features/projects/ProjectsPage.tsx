import { useRef, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AppLink, DetailMissing, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import { useCatalog } from '../workbench/useCatalog'
import { projectRegions } from './data'
import { useProjects, type NewProject } from './ProjectsProvider'

export function ProjectsPage({ path }: { path: string }) {
  const { projects } = useProjects()
  if (path === '/projects/new') return <NewProjectForm />
  if (path !== '/projects') {
    const record = projects.find(item => path === '/projects/' + item.id)
    if (!record) return <DetailMissing kind="项目" backHref="/projects" />
    return <><PageHeading title="项目详情" action={<AppLink className="tic-button secondary" to="/projects">返回项目库</AppLink>} /><Panel><article className="tic-detail"><header><span className="tic-tag">{record.local ? '本地新增 · 刷新重置' : '示例项目'}</span><h1>{record.name}</h1></header><dl><div><dt>项目编号</dt><dd>{record.id}</dd></div><div><dt>项目地区</dt><dd>{record.region}</dd></div><div><dt>关联单位（示例）</dt><dd>{record.client}</dd></div><div><dt>预算（万元，示例）</dt><dd>{record.budget.toLocaleString('zh-CN')}</dd></div><div><dt>录入日期</dt><dd>{record.date}</dd></div><div><dt>数据范围</dt><dd>{record.local ? '当前页面会话，刷新或切换账号/租户后重置' : '固定本地示例，支持直接访问和刷新'}</dd></div></dl><section><h2>项目说明</h2><p>{record.description || '暂无补充说明。'}</p></section></article></Panel></>
  }
  return <ProjectList />
}
function ProjectList() {
  const { projects } = useProjects()
  const catalog = useCatalog(projects, item => item.name + item.id + item.client, item => item.region)
  return <><PageHeading title="项目库" description="组合筛选项目关键词与地区。新增仅保存在当前会话，刷新后重置。" action={<AppLink className="tic-button" to="/projects/new">新建项目</AppLink>} /><Panel><form className="tic-filters" role="search" onSubmit={event => event.preventDefault()}><label>项目关键词<input type="search" value={catalog.keyword} placeholder="项目名称、编号或关联单位" onChange={event => catalog.setKeyword(event.target.value)} /></label><label>项目地区<select value={catalog.category} onChange={event => catalog.setCategory(event.target.value)}><option value="">全部地区</option>{projectRegions.map(region => <option key={region}>{region}</option>)}</select></label><button className="tic-button secondary" type="button" onClick={catalog.reset}>重置</button></form>{catalog.filtered.length ? <div className="tic-table-wrap" role="region" aria-label="项目列表，可横向滚动" tabIndex={0}><table><thead><tr><th scope="col">项目名称</th><th scope="col">地区</th><th scope="col">关联单位</th><th scope="col">预算（万元）</th><th scope="col">录入日期</th><th scope="col">操作</th></tr></thead><tbody>{catalog.visible.map(item => <tr key={item.id}><td><AppLink to={'/projects/' + item.id}>{item.name}</AppLink> <span className="tic-tag">{item.local ? '本地新增' : '示例'}</span></td><td>{item.region}</td><td>{item.client}</td><td>{item.budget.toLocaleString('zh-CN')}</td><td>{item.date}</td><td><AppLink to={'/projects/' + item.id} aria-label={'查看项目：' + item.name}>查看详情</AppLink></td></tr>)}</tbody></table></div> : <EmptyState />}<Pagination page={catalog.page} total={catalog.filtered.length} onChange={catalog.setPage} /></Panel></>
}
type Fields = { name: string; region: string; client: string; budget: string; description: string }
export function validateProject(fields: Fields): Partial<Record<keyof Fields, string>> {
  const errors: Partial<Record<keyof Fields, string>> = {}
  if (fields.name.trim().length < 2 || fields.name.trim().length > 100) errors.name = '请输入 2–100 字的项目名称，不能仅为空格。'
  if (!projectRegions.includes(fields.region)) errors.region = '请选择有效项目地区。'
  if (fields.client.trim().length < 2 || fields.client.trim().length > 80) errors.client = '请输入 2–80 字的关联单位，不能仅为空格。'
  const budget = Number(fields.budget)
  if (!fields.budget.trim() || !Number.isFinite(budget) || budget <= 0 || budget > 1000000000) errors.budget = '预算须大于 0 且不超过 10 亿万元。'
  if (fields.description.trim().length > 2000) errors.description = '项目说明不能超过 2000 字。'
  return errors
}
function NewProjectForm() {
  const { create } = useProjects()
  const navigate = useNavigate()
  const [fields, setFields] = useState<Fields>({ name: '', region: '', client: '', budget: '', description: '' })
  const [errors, setErrors] = useState<Partial<Record<keyof Fields, string>>>({})
  const [submitting, setSubmitting] = useState(false)
  const [navigationError, setNavigationError] = useState(false)
  const submitted = useRef(false)
  const nameInput = useRef<HTMLInputElement>(null)
  function change(key: keyof Fields, value: string) { setFields(previous => ({ ...previous, [key]: value })) }
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (submitted.current) return
    const nextErrors = validateProject(fields)
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length) { nameInput.current?.focus(); return }
    submitted.current = true
    setSubmitting(true)
    const input: NewProject = { name: fields.name.trim(), region: fields.region, client: fields.client.trim(), budget: Number(fields.budget), description: fields.description.trim() }
    create(input)
    try { await navigate({ href: '/projects' }) }
    catch { setNavigationError(true) }
  }
  return <><PageHeading title="新建项目" description="本地演示表单 · 请勿填写真实信息。新增数据仅当前会话有效，刷新或切换账号/租户后重置。" /><Panel><form className="tic-form" noValidate onSubmit={event => void submit(event)} aria-label="新建项目">{Object.keys(errors).length ? <p role="alert">请修正以下输入错误。</p> : null}<label htmlFor="project-name">项目名称（必填）<input id="project-name" ref={nameInput} required maxLength={100} value={fields.name} onChange={event => change('name', event.target.value)} aria-invalid={Boolean(errors.name)} aria-describedby={errors.name ? 'project-name-error' : undefined} />{errors.name ? <span className="tic-field-error" id="project-name-error">{errors.name}</span> : null}</label><label htmlFor="project-region">项目地区（必填）<select id="project-region" required value={fields.region} onChange={event => change('region', event.target.value)} aria-invalid={Boolean(errors.region)} aria-describedby={errors.region ? 'project-region-error' : undefined}><option value="">请选择地区</option>{projectRegions.map(region => <option key={region}>{region}</option>)}</select>{errors.region ? <span className="tic-field-error" id="project-region-error">{errors.region}</span> : null}</label><label htmlFor="project-client">关联单位（必填）<input id="project-client" required maxLength={80} value={fields.client} onChange={event => change('client', event.target.value)} aria-invalid={Boolean(errors.client)} aria-describedby={errors.client ? 'project-client-error' : undefined} />{errors.client ? <span className="tic-field-error" id="project-client-error">{errors.client}</span> : null}</label><label htmlFor="project-budget">预算（万元，必填）<input id="project-budget" type="number" inputMode="decimal" required min="0.01" max="1000000000" step="any" value={fields.budget} onChange={event => change('budget', event.target.value)} aria-invalid={Boolean(errors.budget)} aria-describedby={errors.budget ? 'project-budget-error' : undefined} />{errors.budget ? <span className="tic-field-error" id="project-budget-error">{errors.budget}</span> : null}</label><label htmlFor="project-description">项目说明（选填）<textarea id="project-description" maxLength={2000} value={fields.description} onChange={event => change('description', event.target.value)} aria-invalid={Boolean(errors.description)} aria-describedby={errors.description ? 'project-description-error' : undefined} />{errors.description ? <span className="tic-field-error" id="project-description-error">{errors.description}</span> : null}</label>{navigationError ? <p role="alert">项目已加入本地列表，请点击“取消 / 返回项目库”查看；无需重复提交。</p> : null}<div className="tic-form-actions"><button className="tic-button" type="submit" disabled={submitting}>{submitting ? '已提交本地数据…' : '创建本地项目'}</button><AppLink className="tic-button secondary" to="/projects">取消 / 返回项目库</AppLink></div></form></Panel></>
}
