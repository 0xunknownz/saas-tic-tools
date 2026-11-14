import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ErrorAlert, toApiFailure, useMessages, useTenantApi, useTenants, type ApiFailure, type NavigateFn } from '@0xunknownz/foundation-tenant-ui'
import { safeReturnPath } from '../../cross-app'
import { AuthEntryLayout } from './AuthEntryLayout'

function unauthenticated(error: ApiFailure | null) {
  return error?.status === 401 || error?.code === 'UNAUTHENTICATED' || error?.code === 'SESSION_EXPIRED'
}

export function TicTenantSelectPage({ navigate, redirect }: { navigate: NavigateFn; redirect?: string }) {
  const api = useTenantApi()
  const tenants = useTenants()
  const t = useMessages()
  const target = safeReturnPath(redirect)
  const loginHref = '/login?redirect=' + encodeURIComponent(target)
  const busy = useRef(false)
  const [operation, setOperation] = useState<string | null>(null)
  const [logoutError, setLogoutError] = useState<ApiFailure | null>(null)
  const [manualId, setManualId] = useState('')
  const disabled = operation !== null || tenants.loading || tenants.switching
  const expired = unauthenticated(tenants.switchError) || unauthenticated(tenants.listError)

  useEffect(() => {
    if (!tenants.loading && ((!tenants.session && !tenants.error) || expired)) navigate(loginHref)
  }, [tenants.loading, tenants.session, tenants.error, expired, navigate, loginHref])

  async function enter(id: string) {
    if (busy.current || disabled) return
    busy.current = true
    setOperation(id)
    try {
      if (await tenants.switchTo(id)) navigate(target)
    } finally {
      busy.current = false
      setOperation(null)
    }
  }
  async function retry() {
    if (busy.current || disabled) return
    busy.current = true
    setOperation('refresh')
    try { await tenants.refresh() }
    finally { busy.current = false; setOperation(null) }
  }
  async function logout() {
    if (busy.current || disabled) return
    busy.current = true
    setOperation('logout')
    setLogoutError(null)
    try { await api.logout(); navigate(loginHref) }
    catch (cause) { setLogoutError(toApiFailure(cause)) }
    finally { busy.current = false; setOperation(null) }
  }
  function manualSubmit(event: FormEvent) {
    event.preventDefault()
    void enter(manualId)
  }
  const listUnavailable = tenants.listError && [404, 405, 501].includes(tenants.listError.status)
  const errorMessage = (error: ApiFailure) => error.messageKey ? t(error.messageKey) : '请求未能完成，请重试。'

  return <AuthEntryLayout>
    <main className="tic-entry-tenants" aria-labelledby="tic-tenant-title" aria-busy={disabled}>
      <header className="tic-entry-heading"><h1 id="tic-tenant-title">选择租户</h1><p>选择组织，进入对应工作空间</p></header>
      {tenants.loading ? <p role="status" className="tic-entry-status">正在加载工作空间…</p> : null}
      {tenants.error ? <div className="ftu-stack"><ErrorAlert failure={tenants.error}>{errorMessage(tenants.error)}</ErrorAlert><button className="ftu-button ftu-button-secondary" disabled={disabled} onClick={() => void retry()}>重新加载</button></div> : null}
      {!tenants.loading && !tenants.session && !tenants.error ? <p role="status">登录状态已失效，正在前往登录…</p> : null}
      {tenants.session && !expired ? <>
        <div className="tic-entry-account"><span>当前账号</span><strong>{tenants.session.email}</strong></div>
        {tenants.switchError ? <ErrorAlert failure={tenants.switchError}>{errorMessage(tenants.switchError)}</ErrorAlert> : null}
        {tenants.listError ? <div className="ftu-stack tic-entry-list-error">
          <ErrorAlert failure={tenants.listError}>{listUnavailable ? '组织列表接口暂不可用，可重试或使用租户 ID 进入。' : '组织列表加载失败，请重试。'}</ErrorAlert>
          <button className="ftu-button ftu-button-secondary" disabled={disabled} onClick={() => void retry()}>重试加载组织</button>
        </div> : null}
        {tenants.tenants.length > 0 ? <ul className="tic-entry-tenant-list" aria-label="所属组织">
          {tenants.tenants.map(tenant => {
            const current = tenant.id === tenants.session?.tenant?.id
            return <li key={tenant.id} className={current ? 'is-current' : undefined}>
              <div className="tic-entry-tenant-info"><strong>{tenant.name}</strong><div className="tic-entry-tenant-meta"><span>{tenant.role === 'tenant_admin' ? '管理员' : tenant.role === 'member' ? '成员' : tenant.role}</span>{current ? <span className="tic-entry-current">当前租户</span> : null}{tenant.status !== 'active' ? <span>{tenant.status === 'frozen' ? '已冻结' : tenant.status === 'disabled' ? '已停用' : tenant.status}</span> : null}</div></div>
              <button type="button" className="ftu-button ftu-button-secondary" disabled={disabled} onClick={() => void enter(tenant.id)} aria-label={'进入 ' + tenant.name}>{operation === tenant.id ? '进入中…' : '进入'}</button>
            </li>
          })}
        </ul> : null}
        {!tenants.loading && tenants.tenants.length === 0 && !tenants.listError ? <div className="tic-entry-empty" role="status"><strong>尚未加入任何租户</strong><p>请联系组织管理员邀请你加入，再重新加载。</p><button className="ftu-button ftu-button-secondary" disabled={disabled} onClick={() => void retry()}>重新加载</button></div> : null}
        {listUnavailable ? <form className="ftu-stack tic-entry-manual" onSubmit={manualSubmit} data-ftu-form="tenant-switch-manual"><div><label className="ftu-label" htmlFor="tic-manual-tenant">租户 ID</label><input className="ftu-input" id="tic-manual-tenant" name="tenantId" value={manualId} onChange={event => setManualId(event.target.value)} required aria-describedby="tic-manual-help" /><p id="tic-manual-help">向组织管理员获取租户 ID；进入权限仍由服务器校验。</p></div><button className="ftu-button" type="submit" disabled={disabled || !manualId.trim()}>进入工作空间</button></form> : null}
        <footer className="tic-entry-account-actions">
          {logoutError ? <ErrorAlert failure={logoutError}>退出失败，当前会话未退出，请重试。</ErrorAlert> : null}
          <button className="ftu-link" type="button" disabled={disabled} onClick={() => void logout()}>{operation === 'logout' ? '正在退出…' : '使用其他账号'}</button>
        </footer>
      </> : null}
    </main>
  </AuthEntryLayout>
}
