import { useEffect, type ReactNode } from 'react'
import { Outlet } from '@tanstack/react-router'
import {
  TenantApiProvider,
  TenantUIProvider,
  createHttpTenantApi,
  useSession,
  type UseSessionResult,
} from '@0xunknownz/foundation-tenant-ui'
import { Nav } from './navigation'

const api = createHttpTenantApi()

export type SessionDecision = 'loading' | 'error' | 'login' | 'select' | 'forbidden' | 'ok'

// The root guard is the six-state table in design 4.1. Check tenant before role: a signed-in user
// who has not selected a tenant has an empty role and is not "not an administrator".
export function decideSession(session: UseSessionResult): SessionDecision {
  if (session.loading) return 'loading'
  if (session.error) return 'error'
  if (!session.session) return 'login'
  if (!session.session.tenant) return 'select'
  if (session.session.role !== 'tenant_admin') return 'forbidden'
  return 'ok'
}

export function sessionEntryHref(decision: 'login' | 'select', location: Pick<Location, 'pathname' | 'search' | 'hash'>): string {
  const destination = decision === 'login' ? '/login' : '/tenant/select'
  return destination + '?redirect=' + encodeURIComponent(location.pathname + location.search + location.hash)
}

function Guard({ children }: { children: ReactNode }) {
  const session = useSession()
  const decision = decideSession(session)

  useEffect(() => {
    if (decision === 'login' || decision === 'select') {
      window.location.assign(sessionEntryHref(decision, window.location))
    }
  }, [decision])

  if (decision === 'loading') return <p className="ftu-hint">加载中…</p>
  if (decision === 'error') {
    return (
      <div className="ftu-stack">
        <p role="alert">会话加载失败，请重试。</p>
        <button type="button" className="ftu-button" onClick={() => void session.refresh()}>重试</button>
      </div>
    )
  }
  if (decision === 'forbidden') {
    return <main className="ftu-card ftu-stack">
      <h1>没有管理权限</h1>
      <p>当前账号不是所选租户的管理员。请选择有管理权限的租户，或返回工作台。</p>
      <div className="ftu-row">
        <a className="ftu-button" href="/">返回工作台</a>
        <a className="ftu-link" href={sessionEntryHref('select', window.location)}>切换租户</a>
      </div>
    </main>
  }
  if (decision !== 'ok') return null
  return (
    <>
      <Nav />
      {children}
    </>
  )
}

function Root() {
  return (
    <TenantUIProvider branding={{ productName: 'Saas Tic Tools', locale: 'zh-CN' }}>
      <TenantApiProvider api={api}>
        <Guard>
          <Outlet />
        </Guard>
      </TenantApiProvider>
    </TenantUIProvider>
  )
}

export { Root }
