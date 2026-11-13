import { useState, type ReactNode } from 'react'
import { useRouterState } from '@tanstack/react-router'
import { useSession, useTenantApi } from '@0xunknownz/foundation-tenant-ui'
import { activeGroup, navGroups, pageTitle } from './navigation'
import { AppLink as Link, Icon, SampleNotice } from './ui'
import { ProjectsProvider } from '../projects/ProjectsProvider'

const authPaths = ['/login', '/tenant/select', '/forgot-password', '/reset-password', '/invitations/accept', '/mfa/enroll', '/mfa/verify']
export function WorkbenchBoundary({ children }: { children: ReactNode }) {
  const path = useRouterState({ select: state => state.location.pathname })
  return authPaths.includes(path) ? children : <WorkbenchShell>{children}</WorkbenchShell>
}
export function WorkbenchShell({ children }: { children: ReactNode }) {
  const location = useRouterState({ select: state => state.location })
  const current = activeGroup(location.pathname)
  const selectedItem = current?.items?.find(item => item.href === location.pathname || (item.href !== current.href && location.pathname.startsWith(item.href + "/")))
    ?? (current?.href === "/projects" ? current.items?.[0] : undefined)
  const session = useSession()
  const api = useTenantApi()
  const [menuOpen, setMenuOpen] = useState(false)
  const [logoutState, setLogoutState] = useState<'idle' | 'busy' | 'failed'>('idle')
  const loginHref = '/login?redirect=' + encodeURIComponent(location.href)
  if (session.loading) return <main className="tic-auth-state" role="status">正在确认登录状态…</main>
  if (session.error) return <main className="tic-auth-state"><h1>暂时无法确认登录状态</h1><p role="alert">请重试，或继续完成认证流程。</p><button className="tic-button" onClick={() => void session.refresh()}>重试</button> <Link to="/mfa/verify">MFA 验证</Link> · <Link to="/mfa/enroll">MFA 注册</Link> · <Link to={loginHref}>返回登录</Link></main>
  if (!session.session) return <main className="tic-auth-state"><h1>登录后进入 TIC 工作台</h1><p>请使用现有账号登录，继续访问工具与项目。</p><Link className="tic-button" to={loginHref}>前往登录</Link></main>
  async function logout() {
    setLogoutState('busy')
    try { await api.logout(); window.location.assign('/login') }
    catch { setLogoutState('failed') }
  }
  return <div className={'tic-app' + (current?.items ? ' has-secondary' : '')}>
    <a className="tic-skip" href="#tic-content">跳到主要内容</a>
    <aside className="tic-primary" aria-label="主导航"><Link className="tic-brand" to="/" aria-label="TIC工具箱首页"><Icon name="archive" /></Link><nav>{navGroups.map(group => <Link key={group.href} to={group.href} className={'tic-primary-link' + (current === group ? ' active' : '')} aria-label={group.label} title={group.label} aria-current={current === group ? 'page' : undefined} onClick={() => setMenuOpen(false)}><Icon name={group.icon} /><span>{group.label}</span></Link>)}</nav></aside>
    {current?.items ? <aside className={'tic-secondary' + (menuOpen ? ' open' : '')} aria-label={current.label + '二级菜单'}><p className="tic-eyebrow">工作模块</p><h2>{current.label}</h2><nav>{current.items.map(item => <Link key={item.href} to={item.href} className={selectedItem === item ? 'active' : ''} aria-current={selectedItem === item ? 'page' : undefined} onClick={() => setMenuOpen(false)}>{item.label}<span aria-hidden="true">›</span></Link>)}</nav></aside> : null}
    <div className="tic-workspace"><header className="tic-topbar"><strong>{location.pathname === '/' ? 'TIC工具箱' : pageTitle(location.pathname)}</strong><div>{current?.items ? <button className="tic-button secondary tic-menu-toggle" aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>模块菜单</button> : null}<Link to="/announcements" className="tic-top-icon" aria-label="查看公司公告"><Icon name="bell" /></Link><span className="tic-account" title={session.session.email}>{session.session.tenant?.name || session.session.email}</span><Link to="/tenant/select">切换租户</Link><button className="tic-button secondary" disabled={logoutState === 'busy'} onClick={() => void logout()}>{logoutState === 'busy' ? '退出中…' : '退出'}</button></div></header>
    <main id="tic-content" className="tic-content" tabIndex={-1}><SampleNotice local={location.pathname.startsWith('/projects')} />{logoutState === 'failed' ? <p role="alert">退出失败，请重试。当前会话仍有效。</p> : null}<ProjectsProvider key={session.session.accountId + ":" + (session.session.tenant?.id ?? "")}>{children}</ProjectsProvider><footer className="tic-footer"><span>TIC 工具箱 · 模板演示</span><nav aria-label="账户管理"><a href="/console/members">成员管理</a><a href="/console/billing">账单管理</a><a href="/admin/">平台管理端</a></nav></footer></main>
    </div>
  </div>
}
