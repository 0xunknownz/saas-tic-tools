import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { WorkbenchBoundary } from '../src/features/workbench/Shell'
import { WorkbenchPage } from '../src/features/workbench/WorkbenchPage'
import { validateProject } from '../src/features/projects/ProjectsPage'
import { announcements } from '../src/features/announcements/data'
import { reports } from '../src/features/reports/data'
import { mockApi, renderWithProviders } from './helpers'

afterEach(() => vi.unstubAllGlobals())

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function apiReport(item: (typeof reports)[number]) {
  return {
    id: item.id,
    file: { id: 'file-' + item.id, fileName: item.attachments[0]?.name ?? item.id + '.pdf', contentType: 'application/pdf', size: 1_000_000, pages: 4, sha256: 'a'.repeat(64), createdAt: item.issued + 'T00:00:00Z' },
    reportNumber: item.number,
    reportName: item.name,
    productName: item.name,
    institution: item.organization,
    clientUnit: item.object,
    inspectedUnit: item.object,
    manufacturer: '',
    model: '',
    category: '',
    basis: '',
    reportDate: item.issued,
    conclusion: item.summary,
    detectionResult: item.status === '已归档' ? 'qualified' : 'unknown',
    archiveStatus: item.status === '已归档' ? 'archived' : item.status === '待补充' ? 'needs_review' : 'parsing',
    reviewStage: item.status === '待补充' ? 'parse' : '',
    reviewReason: item.status === '待补充' ? '示例待补充原因' : '',
    reportVersion: 1,
    revision: 1,
    isCurrent: item.status === '已归档',
    createdBy: 'account:test',
    createdAt: item.issued + 'T00:00:00Z',
    updatedAt: item.issued + 'T00:00:00Z',
    details: item.id === 'report-002' ? [{ id: 'd1', section: '水质', itemName: 'pH 值', requirement: '6.5-8.5', unit: '', method: 'GB/T 5750.4', resultRaw: '7.2', judgement: '合格', position: 0, sourcePage: 2, sourceText: 'pH 值 7.2 合格' }] : [],
    fieldSources: { reportDate: { raw: '报告日期：' + item.issued, page: 1, line: 5 } },
    events: [],
    versions: [],
  }
}

function mockReportFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    if (url.includes('/report-archive/reports/missing')) return jsonResponse({ code: 'NOT_FOUND', message: '记录不存在' }, 404)
    if (url.includes('/report-archive/reports/report-002')) return jsonResponse(apiReport(reports.find(item => item.id === 'report-002')!))
    if (url.includes('/report-archive/reports?')) {
      const query = new URL(url, 'http://localhost').searchParams
      const keyword = query.get('query') ?? ''
      const scope = query.get('scope') ?? 'current'
      let items = reports.filter(item => scope === 'review' ? item.status !== '已归档' : scope === 'voided' ? false : true)
      if (keyword) items = items.filter(item => (item.number + item.name + item.object).includes(keyword))
      const page = Number(query.get('page') ?? '1')
      const pageSize = Number(query.get('pageSize') ?? '20')
      return jsonResponse({ items: items.map(apiReport), total: items.length, page, pageSize })
    }
    if (url.includes('/validation')) return jsonResponse({ issues: [] })
    if (url.includes('/candidate')) return jsonResponse({ code: 'CANDIDATE_NOT_FOUND', message: '没有候选' }, 404)
    return jsonResponse({ code: 'NOT_FOUND', message: '记录不存在' }, 404)
  }))
}

function renderBusiness(path: string) {
  const root = createRootRoute({ component: () => <WorkbenchBoundary><Outlet /></WorkbenchBoundary> })
  const route = createRoute({ getParentRoute: () => root, path: '$', component: WorkbenchPage })
  const home = createRoute({ getParentRoute: () => root, path: '/', component: WorkbenchPage })
  const router = createRouter({ routeTree: root.addChildren([route, home]), history: createMemoryHistory({ initialEntries: [path] }) })
  const api = mockApi()
  const view = renderWithProviders(<RouterProvider router={router} />, { api })
  return { ...view, router, api }
}
const catalogs = [
  { path: '/announcements', label: '公告关键词', total: 8, query: '档案字段不会有这个词', match: '归档字段', text: '检测报告归档字段更新说明' },
  { path: '/projects', label: '项目关键词', total: 9, query: 'never-match', match: '园区环境', text: '示例园区环境检测服务' },
  { path: '/standards', label: '标准关键词', total: 9, query: 'never-match', match: '001-2026', text: '检测与校准实验室能力要求' },
]
describe('TIC local business catalogs', () => {
  for (const catalog of catalogs) {
    it(catalog.path + ' filters the current result set, resets paging, restores counts and handles empty results', async () => {
      renderBusiness(catalog.path)
      const input = await screen.findByRole('searchbox', { name: catalog.label })
      expect(screen.getByText('共 ' + catalog.total + ' 条 · 第 1 / 2 页')).toBeTruthy()
      expect(screen.getAllByRole('row')).toHaveLength(6)
      fireEvent.click(screen.getByRole('button', { name: '下一页' }))
      expect(screen.getByText('共 ' + catalog.total + ' 条 · 第 2 / 2 页')).toBeTruthy()
      fireEvent.change(input, { target: { value: '  ' + catalog.match + '  ' } })
      expect(screen.getByText('共 1 条 · 第 1 / 1 页')).toBeTruthy()
      expect(screen.getByRole('link', { name: catalog.text, exact: true })).toBeTruthy()
      fireEvent.change(input, { target: { value: catalog.query } })
      expect(screen.getByRole('heading', { name: '没有匹配结果' })).toBeTruthy()
      expect(screen.getByText('共 0 条 · 第 1 / 1 页')).toBeTruthy()
      expect((screen.getByRole('button', { name: '下一页' }) as HTMLButtonElement).disabled).toBe(true)
      fireEvent.click(screen.getByRole('button', { name: '重置' }))
      expect((input as HTMLInputElement).value).toBe('')
      expect(screen.getByText('共 ' + catalog.total + ' 条 · 第 1 / 2 页')).toBeTruthy()
    })
  }
  for (const entry of [
    { path: '/projects', label: '项目关键词', query: '环境', select: '项目地区', value: '北京市', badValue: '上海市' },
    { path: '/standards', label: '标准关键词', query: '采样', select: '标准分类', value: '环境检测', badValue: '食品检测' },
  ]) {
    it(entry.path + ' combines keyword and category instead of replacing either condition', async () => {
      renderBusiness(entry.path)
      fireEvent.change(await screen.findByRole('searchbox', { name: entry.label }), { target: { value: entry.query } })
      fireEvent.change(screen.getByRole('combobox', { name: entry.select }), { target: { value: entry.value } })
      expect(screen.getByText('共 1 条 · 第 1 / 1 页')).toBeTruthy()
      fireEvent.change(screen.getByRole('combobox', { name: entry.select }), { target: { value: entry.badValue } })
      expect(screen.getByText('共 0 条 · 第 1 / 1 页')).toBeTruthy()
      fireEvent.click(screen.getByRole('button', { name: '重置' }))
      expect((screen.getByRole('combobox', { name: entry.select }) as HTMLSelectElement).value).toBe('')
    })
  }
  it('opens the selected announcement with matching title, date and full article, and provides a way back', async () => {
    const { router } = renderBusiness('/announcements')
    fireEvent.click(await screen.findByRole('link', { name: announcements[0].title, exact: true }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/announcements/notice-001'))
    expect(screen.getByRole('heading', { name: announcements[0].title })).toBeTruthy()
    expect(screen.getByText(announcements[0].paragraphs[0])).toBeTruthy()
    expect(screen.getByText(announcements[0].date)).toBeTruthy()
    fireEvent.click(screen.getByRole('link', { name: '返回公告列表' }))
    expect(await screen.findByRole('searchbox', { name: '公告关键词' })).toBeTruthy()
  })
  for (const entry of [
    { path: '/announcements/missing', title: '公告不存在' },
    { path: '/projects/local-vanished', title: '项目不存在' },
    { path: '/standards/missing', title: '标准不存在' },
  ]) {
    it(entry.path + ' provides explicit missing feedback', async () => {
      renderBusiness(entry.path)
      expect(await screen.findByRole('heading', { name: entry.title })).toBeTruthy()
      expect(screen.getByRole('link', { name: '返回列表' })).toBeTruthy()
      expect(screen.queryByRole('table')).toBeNull()
    })
  }
  it('loads the real report list, filters, and shows an explicit unknown archive', async () => {
    mockReportFetch()
    const first = renderBusiness('/reports')
    expect(await screen.findByRole('searchbox', { name: '关键词' })).toBeTruthy()
    expect(await screen.findByText('共 9 条 · 第 1 / 2 页')).toBeTruthy()
    fireEvent.change(screen.getByRole('searchbox', { name: '关键词' }), { target: { value: 'DEMO-RPT-2026-001' } })
    fireEvent.click(screen.getByRole('button', { name: '查询' }))
    expect(await screen.findByText('共 1 条 · 第 1 / 1 页')).toBeTruthy()
    expect(screen.getByRole('link', { name: '办公楼室内空气检测报告' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '重置' }))
    expect(await screen.findByText('共 9 条 · 第 1 / 2 页')).toBeTruthy()
    first.unmount()
    renderBusiness('/reports/missing')
    expect(await screen.findByRole('heading', { name: '报告档案不存在' })).toBeTruthy()
    expect(screen.getByRole('link', { name: '返回报告列表' })).toBeTruthy()
  })
  it('shows the real report detail with structured fields and the original file link', async () => {
    mockReportFetch()
    renderBusiness('/reports/report-002')
    expect(await screen.findByRole('heading', { name: '饮用水水质检测报告' })).toBeTruthy()
    expect(screen.getByText('DEMO-RPT-2026-002')).toBeTruthy()
    expect(screen.getByText('示例水质检测中心')).toBeTruthy()
    expect(screen.getByRole('link', { name: '下载原件' }).getAttribute('href')).toBe('/api/tenant/v1/report-archive/reports/report-002/download')
    expect(screen.getByTitle('报告原件预览')).toBeTruthy()
    expect(screen.queryByText('示例-001-报告正文.pdf')).toBeNull()
  })
  it('supports stable standard detail directly and clearly limits full-text capabilities', async () => {
    renderBusiness('/standards/standard-003')
    expect(await screen.findByRole('heading', { name: '地表水采样质量控制指南' })).toBeTruthy()
    expect(screen.getByText('GB/T 示例 003-2025')).toBeTruthy()
    expect(screen.getByText('环境检测')).toBeTruthy()
    expect(screen.queryByRole('button', { name: /下载/ })).toBeNull()
    expect(screen.getByRole('link', { name: '返回标准查询' })).toBeTruthy()
  })
})
describe('local project form and lifetime', () => {
  it('rejects whitespace, invalid regions, non-finite/negative budgets and overlong values', () => {
    const valid = { name: '示例新增项目', region: '北京市', client: '示例单位', budget: '1.5', description: '' }
    expect(validateProject(valid)).toEqual({})
    expect(Object.keys(validateProject({ name: '  ', region: '非法地区', client: ' ', budget: '-1', description: 'x'.repeat(2001) }))).toHaveLength(5)
    expect(validateProject({ ...valid, budget: 'Infinity' }).budget).toBeTruthy()
    expect(validateProject({ ...valid, budget: 'NaN' }).budget).toBeTruthy()
    expect(validateProject({ ...valid, name: 'x'.repeat(101) }).name).toBeTruthy()
  })
  it('does not create on cancel, associates field errors, prevents duplicate creation, and resets after reload', async () => {
    const view = renderBusiness('/projects/new')
    const form = await screen.findByRole('form', { name: '新建项目' })
    fireEvent.submit(form)
    expect(screen.getByRole('alert').textContent).toContain('输入错误')
    const name = screen.getByLabelText(/项目名称/)
    expect(name.getAttribute('aria-invalid')).toBe('true')
    expect(name.getAttribute('aria-describedby')).toBe('project-name-error')
    fireEvent.click(screen.getByRole('link', { name: '取消 / 返回项目库' }))
    await screen.findByRole('searchbox', { name: '项目关键词' })
    expect(screen.getByText('共 9 条 · 第 1 / 2 页')).toBeTruthy()
    fireEvent.click(screen.getByRole('link', { name: '新建项目' }))
    await screen.findByRole('form', { name: '新建项目' })
    fireEvent.change(screen.getByLabelText(/项目名称/), { target: { value: '  本地新增测试项目  ' } })
    fireEvent.change(screen.getByLabelText(/项目地区/), { target: { value: '北京市' } })
    fireEvent.change(screen.getByLabelText(/关联单位/), { target: { value: '  示例测试单位  ' } })
    fireEvent.change(screen.getByLabelText(/预算/), { target: { value: '3.25' } })
    const validForm = screen.getByRole('form', { name: '新建项目' })
    fireEvent.submit(validForm)
    fireEvent.submit(validForm)
    await screen.findByRole('searchbox', { name: '项目关键词' })
    expect(screen.getByText('共 10 条 · 第 1 / 2 页')).toBeTruthy()
    const localLink = screen.getByRole('link', { name: '本地新增测试项目', exact: true })
    const localHref = localLink.getAttribute('href')!
    expect(localHref).toMatch(/^\/projects\/local-/)
    fireEvent.click(localLink)
    expect(await screen.findByRole('heading', { name: '本地新增测试项目' })).toBeTruthy()
    expect(screen.getByText('示例测试单位')).toBeTruthy()
    await view.router.navigate({ href: '/standards' })
    await screen.findByRole('searchbox', { name: '标准关键词' })
    await view.router.navigate({ href: localHref })
    expect(await screen.findByRole('heading', { name: '本地新增测试项目' })).toBeTruthy()
    expect(view.api.inviteMember).not.toHaveBeenCalled()
    expect(view.api.purchasePlan).not.toHaveBeenCalled()
    view.unmount()
    renderBusiness(localHref)
    expect(await screen.findByRole('heading', { name: '项目不存在' })).toBeTruthy()
    expect(screen.getByText(/刷新后已重置/)).toBeTruthy()
  })
})
