import { useRouterState } from '@tanstack/react-router'
import { allNavItems, navGroups, projectItems, toolItems } from './navigation'
import { AppLink as Link, EmptyState, Icon, PageHeading, Panel } from './ui'

export function WorkbenchPage() {
  const path = useRouterState({ select: state => state.location.pathname })
  if (path === '/') return <Home />
  const item = allNavItems.find(entry => entry.href === path)
  const group = navGroups.find(entry => entry.href === path && entry.items)
  if (group) return <><PageHeading title={group.label} description="按工作目的选择入口。业务数据均为示例。" /><Panel><div className="tic-tool-grid">{group.items!.map(entry => <QuickLink key={entry.href} {...entry} />)}</div></Panel></>
  if (item) return <><PageHeading title={item.label} description="模板说明页" /><Panel><span className="tic-tag">模板入口 · 尚未接入业务</span><h2 className="tic-template-title">{item.label}</h2><p>此入口用于展示导航和页面结构，暂不提供真实业务查询、授权或文件操作。</p>{path.startsWith('/operations') ? <p>成员及账单请使用<a href="/console/members">租户成员管理</a>与<a href="/console/billing">账单管理</a>；平台运营功能由<a href="/admin/">平台管理端</a>负责。</p> : null}{path === '/contact' ? <p>模板尚未配置联系信息，请通过贵组织已公布的支持渠道联系管理员。</p> : null}<Link className="tic-button secondary" to="/">返回工具首页</Link></Panel></>
  return <Panel><EmptyState title="页面未找到"><p>此地址没有对应的工作台页面，请检查链接或返回首页。</p><Link className="tic-button" to="/">返回工具首页</Link></EmptyState></Panel>
}
function QuickLink({ label, href, description, index = 0 }: { label: string; href: string; description?: string; index?: number }) {
  return <Link className={'tic-tool-card tone-' + index % 4} to={href}><Icon name={['search', 'building', 'clipboard', 'document', 'shield', 'sliders'][index % 6]} /><strong>{label}</strong><span>{description || '进入资料库'}</span><b aria-hidden="true">›</b></Link>
}
function Home() {
  return <div className="tic-home-grid"><Panel title="TIC工具箱" eyebrow="TIC TOOLBOX" action={<Link to="/tools">查看全部 ›</Link>} className="tic-tools-panel"><p className="tic-muted">按查询目的直接进入，不需要经过多层菜单。</p><div className="tic-tool-grid">{toolItems.map((item, index) => <QuickLink key={item.href} {...item} index={index} />)}</div></Panel><div className="tic-home-side"><Panel title="项目快捷入口" eyebrow="PROJECT RECORDS"><div className="tic-project-links">{projectItems.map(item => <Link key={item.href} to={item.href}><span><strong>{item.label}</strong><small>{item.description}</small></span><span aria-hidden="true">›</span></Link>)}</div></Panel><Panel title="公司公告" eyebrow="INTERNAL NOTICES" action={<Link to="/announcements">全部公告</Link>}><div className="tic-project-links">{['检测报告归档字段更新说明', '质疑材料命名规范更新', '项目资料整理提醒'].map((title, i) => <Link key={title} to="/announcements"><span><span className="tic-tag">示例公告</span><strong>{title}</strong></span><small>11-{String(13 - i).padStart(2, '0')}</small></Link>)}</div></Panel></div></div>
}
