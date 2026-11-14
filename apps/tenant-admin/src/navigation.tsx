import { useState, type ReactNode } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useSession } from '@0xunknownz/foundation-tenant-ui'

function Icon({ kind }: { kind: 'home' | 'settings' | 'members' | 'billing' }) {
  return <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === 'home' ? <><path d="m3 10 9-7 9 7v10H3Z"/><path d="M9 20v-7h6v7"/></> : kind === 'members' ? <><circle cx="9" cy="8" r="3"/><path d="M3 21v-3a6 6 0 0 1 12 0v3m2-16a3 3 0 0 1 0 6m1 4a5 5 0 0 1 3 5"/></> : kind === 'billing' ? <><rect x="3" y="5" width="18" height="14" rx="3"/><path d="M3 10h18M7 15h3"/></> : <><path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3" fill="white"/><circle cx="15" cy="17" r="3" fill="white"/></>}
  </svg>
}

// The router owns console links; anchors cross back into the Web application.
export function Nav({ onNavigate }: { onNavigate?: () => void }) {
  return <nav aria-label="租户后台">
    <Link to="/members" activeProps={{ className: 'active', 'aria-current': 'page' }} onClick={onNavigate}><Icon kind="members"/>成员管理<span aria-hidden="true">›</span></Link>
    <Link to="/billing" activeProps={{ className: 'active', 'aria-current': 'page' }} onClick={onNavigate}><Icon kind="billing"/>账单管理<span aria-hidden="true">›</span></Link>
    <a href="/"><Icon kind="home"/>返回工作台<span aria-hidden="true">↗</span></a>
  </nav>
}

export function TenantLayout({ children }: { children: ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false)
  const returnHref = useRouterState({ select: state => '/console' + state.location.href })
  const { session } = useSession()
  return <div className="tic-admin">
    <a className="tic-skip" href="#tic-admin-content">跳到主要内容</a>
    <aside className="tic-primary" aria-label="主导航"><a href="/" className="tic-brand" aria-label="TIC 工具箱首页">T</a><nav><a href="/"><Icon kind="home"/><span>工作台</span></a><a href="/console/members" className="active" aria-current="true"><Icon kind="settings"/><span>租户管理</span></a></nav></aside>
    <aside className={'tic-secondary' + (menuOpen ? ' open' : '')} id="tic-admin-menu"><p className="tic-eyebrow">TIC 工具箱</p><h2>租户管理</h2><Nav onNavigate={() => setMenuOpen(false)}/><p className="tic-sidebar-note">管理团队访问与订阅服务</p></aside>
    <div className="tic-workspace"><header className="tic-topbar"><strong>租户管理</strong><div><button className="tic-button tic-button-secondary tic-menu-toggle" aria-controls="tic-admin-menu" aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>管理菜单</button><span className="tic-account" title={session?.email}>{session?.tenant?.name}</span><a href={'/tenant/select?redirect=' + encodeURIComponent(returnHref)}>切换租户</a></div></header>
      <main id="tic-admin-content" className="tic-content" tabIndex={-1}>{children}</main>
    </div>
  </div>
}
