import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router'
import { describe, expect, it } from 'vitest'
import { WorkbenchBoundary } from '../src/features/workbench/Shell'
import { WorkbenchPage } from '../src/features/workbench/WorkbenchPage'
import { validateProject } from '../src/features/projects/ProjectsPage'
import { announcements } from '../src/features/announcements/data'
import { mockApi, renderWithProviders } from './helpers'

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
  { path: '/reports', label: '报告关键词', total: 9, query: 'never-match', match: 'DEMO-RPT-2026-001', text: '办公楼室内空气检测报告' },
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
    { path: '/reports', label: '报告关键词', query: '饮用水', select: '报告状态', value: '待补充', badValue: '已归档' },
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
    { path: '/reports/missing', title: '报告档案不存在' },
  ]) {
    it(entry.path + ' provides explicit missing feedback', async () => {
      renderBusiness(entry.path)
      expect(await screen.findByRole('heading', { name: entry.title })).toBeTruthy()
      expect(screen.getByRole('link', { name: '返回列表' })).toBeTruthy()
      expect(screen.queryByRole('table')).toBeNull()
    })
  }
  it('shows only the selected report attachments without promising file operations', async () => {
    renderBusiness('/reports/report-002')
    expect(await screen.findByRole('heading', { name: '饮用水水质检测报告' })).toBeTruthy()
    expect(screen.getByText('DEMO-RPT-2026-002')).toBeTruthy()
    expect(screen.getByText('示例-002-水质检测.pdf')).toBeTruthy()
    expect(screen.queryByText('示例-001-报告正文.pdf')).toBeNull()
    expect(screen.queryByRole('button', { name: /下载|上传/ })).toBeNull()
    expect(screen.queryByRole('link', { name: /下载|上传/ })).toBeNull()
  })
  it('handles an archive with no attachment metadata', async () => {
    renderBusiness('/reports/report-006')
    expect(await screen.findByRole('heading', { name: '暂无示例附件信息' })).toBeTruthy()
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
