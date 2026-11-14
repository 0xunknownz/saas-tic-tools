import { useRef, useState, type FormEvent } from 'react'
import { useMembers, useMessages, useSession, type TenantMember } from '@0xunknownz/foundation-tenant-ui'
import { Dialog } from '../components/Dialog'
import { Failure, utcDate } from '../components/Feedback'

export function resolveTenantAdminPath(path: string): 'route' | 'page' {
  return path === '/members' || path === '/billing' ? 'route' : 'page'
}

type Action = { member: TenantMember; kind: 'mfa' | 'password' | 'remove' | 'role' }
const actionTitles = { mfa: '重置 MFA', password: '发送密码重置', remove: '移除成员', role: '调整为普通成员' }

// Mount an isolated hook for each invitation. Closing destroys all transient credentials.
function InvitationDrawer({ onClose }: { onClose: () => void }) {
  const invitation = useMembers(false)
  const [email, setEmail] = useState('')
  const [fieldError, setFieldError] = useState('')
  const [resultEmail, setResultEmail] = useState<string | null>(null)
  const lock = useRef(false)
  const pending = invitation.invitePending || invitation.manualPending
  async function submit(kind: 'mail' | 'manual') {
    if (lock.current) return
    const value = email.trim()
    if (!value || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
      setFieldError(value ? '请输入有效的邮箱地址。' : '请输入成员邮箱。')
      document.getElementById('tic-invite-email')?.focus()
      return
    }
    setFieldError('')
    setResultEmail(value)
    lock.current = true
    try {
      if (kind === 'mail') await invitation.invite(value)
      else await invitation.issueManualInvitation(value)
    } finally { lock.current = false }
  }
  return <Dialog title="邀请成员" className="tic-drawer" busy={pending} onClose={() => { if (!lock.current) onClose() }}>
    <p className="tic-hint">邀请新成员加入当前租户，默认角色为普通成员。</p>
    <form className="tic-stack" noValidate onSubmit={event => { event.preventDefault(); void submit('mail') }} aria-label="邀请成员">
      <div className="tic-form-field"><label className="tic-label" htmlFor="tic-invite-email">成员邮箱 <span aria-hidden="true">*</span></label><input id="tic-invite-email" className="tic-input" type="email" autoComplete="email" required value={email} disabled={pending} aria-invalid={!!fieldError} aria-describedby={fieldError ? 'tic-invite-email-error' : 'tic-invite-hint'} onChange={event => { setEmail(event.target.value); setFieldError(''); setResultEmail(null) }}/>{fieldError ? <p id="tic-invite-email-error" className="tic-field-error" role="alert">{fieldError}</p> : null}<p id="tic-invite-hint" className="tic-hint">邮件邀请由服务端处理，也可生成手动邀请凭据。</p></div>
      <Failure failure={resultEmail ? invitation.inviteError || invitation.manualError : null}/>
      {resultEmail && invitation.invited ? <p className="tic-alert" role="status">邀请请求已受理，请提醒成员查收邮件；这不代表邮件已送达。</p> : null}
      {resultEmail && invitation.manualCredential ? <div className="tic-alert" role="status"><strong>手动邀请凭据</strong><p>受邀邮箱：{resultEmail}</p><p>请通过可信渠道交给受邀成员，关闭后不再显示。</p><code className="tic-credential">{invitation.manualCredential}</code></div> : null}
      <div className="tic-actions"><button className="tic-button" type="submit" disabled={pending}>{invitation.invitePending ? '提交中…' : '发送邀请'}</button><button className="tic-button tic-button-secondary" type="button" disabled={pending} onClick={() => void submit('manual')}>{invitation.manualPending ? '生成中…' : '手动邀请'}</button></div>
      <button className="tic-button tic-button-secondary" type="button" disabled={pending} onClick={onClose}>关闭侧栏</button>
    </form>
  </Dialog>
}

export function MembersRoute() {
  const members = useMembers()
  const { session } = useSession()
  const t = useMessages()
  const [inviting, setInviting] = useState(false)
  const [target, setTarget] = useState<Action | null>(null)
  const [reason, setReason] = useState('')
  const [reasonError, setReasonError] = useState('')
  const lock = useRef(false)
  const actionBusy = members.actionPending || members.loading
  const permissions = session?.permissions ?? []
  const canManage = session?.role === 'tenant_admin' || permissions.includes('tenant.members.manage')
  const canManageRoles = permissions.includes('tenant.roles.manage')
  function openAction(member: TenantMember, kind: Action['kind']) {
    members.clearAction()
    setReason(''); setReasonError(''); setTarget({ member, kind })
  }
  async function confirm(event: FormEvent) {
    event.preventDefault()
    if (!target || lock.current) return
    if ((target.kind === 'mfa' || target.kind === 'password') && !reason.trim()) {
      setReasonError('请填写操作原因。')
      document.getElementById('tic-action-reason')?.focus()
      return
    }
    lock.current = true
    try {
      const id = target.member.accountId
      const ok = target.kind === 'remove' ? await members.removeMember(id)
        : target.kind === 'role' ? await members.changeRole(id, 'member')
        : target.kind === 'mfa' ? await members.resetMfa(id, reason) : await members.sendPasswordReset(id, reason)
      if (ok) { setTarget(null); setReason('') }
    } finally { lock.current = false }
  }
  return <div className="tic-page">
    <header className="tic-page-heading"><div><h1>成员管理</h1><p>管理成员访问权限与账号安全。</p></div>{canManage ? <button className="tic-button" onClick={() => setInviting(true)}>＋ 邀请成员</button> : null}</header>
    {!canManage ? <p className="tic-alert" role="alert">没有成员管理权限。</p> : <section className="tic-panel" aria-labelledby="tic-members-heading">
      <div className="tic-panel-heading"><h2 id="tic-members-heading">团队成员</h2><span className="tic-hint">{!members.loading && !members.error ? members.members.length + ' 位成员' : '成员访问管理'}</span></div>
      {members.loading ? <p className="tic-empty" role="status">正在加载成员…</p> : members.error ? <div className="tic-stack"><Failure failure={members.error}/><button className="tic-button tic-button-secondary" onClick={() => void members.refresh()}>重新加载成员</button></div> : members.members.length === 0 ? <p className="tic-empty">当前租户暂无成员。</p> : <div className="tic-table-scroll" role="region" aria-label="成员列表，可横向滚动" tabIndex={0}><table className="tic-table"><thead><tr><th scope="col">成员</th><th scope="col">邮箱</th><th scope="col">角色</th><th scope="col">MFA</th><th scope="col">加入时间</th><th scope="col">操作</th></tr></thead><tbody>{members.members.map(member => {
        const self = session?.accountId === member.accountId
        return <tr key={member.accountId}><td><div className="tic-member-name"><span className="tic-avatar" aria-hidden="true">{(member.displayName || member.email).slice(0, 1).toUpperCase()}</span><strong title={member.displayName || member.email.split('@')[0]}>{member.displayName || member.email.split('@')[0]}</strong>{self ? <span className="tic-badge">当前账号</span> : null}</div>{member.viaForwardedInvitation ? <span className="tic-hint">通过转发邀请加入</span> : null}</td><td>{member.email}</td><td>{member.role === 'tenant_admin' ? '管理员' : member.role === 'member' ? '普通成员' : member.role}</td><td><span className={'tic-badge' + (member.mfaBound ? ' success' : '')}>{member.mfaBound === undefined ? '未提供' : member.mfaBound ? '已绑定' : '未绑定'}</span></td><td className="tic-nowrap">{utcDate(member.joinedAt)}</td><td>{self ? <span className="tic-hint">不可操作自身</span> : <details className="tic-menu" onKeyDown={event => { if (event.key === 'Escape') { event.currentTarget.open = false; event.currentTarget.querySelector('summary')?.focus() } }}><summary>管理</summary><div>{(['mfa', 'password', 'remove', ...(canManageRoles && member.role === 'tenant_admin' ? ['role'] : [])] as Action['kind'][]).map(kind => <button key={kind} type="button" disabled={actionBusy} onClick={event => { event.currentTarget.closest('details')!.open = false; openAction(member, kind) }}>{actionTitles[kind]}</button>)}</div></details>}</td></tr>
      })}</tbody></table></div>}
      <p className="tic-hint tic-panel-note">{canManageRoles ? '可将其他管理员调整为普通成员；' : '角色调整需要角色管理权限；'}当前接口不支持提升成员为管理员。加入时间以 UTC 标注。</p>
      {!target ? <Failure failure={members.actionError}/> : null}
      {!target && members.actionNoticeKey ? <p className="tic-alert" role="status">{members.actionNoticeKey === 'members.reset.passwordSent' ? '密码重置请求已受理，不代表邮件已送达。' : t(members.actionNoticeKey)}</p> : null}
      {!target && members.actionCredential ? <div className="tic-alert" role="status"><p>重置凭据仅在此显示，请通过可信渠道交给成员。</p><code className="tic-credential">{members.actionCredential}</code><button className="tic-button tic-button-secondary" onClick={members.clearAction}>隐藏凭据</button></div> : null}
    </section>}
    {inviting ? <InvitationDrawer onClose={() => setInviting(false)}/> : null}
    {target ? <Dialog title={actionTitles[target.kind]} busy={actionBusy} onClose={() => { if (!lock.current) setTarget(null) }}><form className="tic-stack" onSubmit={confirm} noValidate aria-label={actionTitles[target.kind]}><p>操作对象：<strong>{target.member.email}</strong></p><p className="tic-hint">{target.kind === 'remove' ? '移除后，该成员将失去当前租户的访问权限。确认继续？' : target.kind === 'role' ? '调整后，该账号将失去租户管理权限。服务端会保护最后一名管理员。' : target.kind === 'mfa' ? '将重置成员的 MFA，影响其后续登录。请确认已核实成员身份。' : '将申请密码重置。服务端按既有协议处理邮件或返回临时凭据。'}</p>{target.kind === 'mfa' || target.kind === 'password' ? <div className="tic-form-field"><label className="tic-label" htmlFor="tic-action-reason">操作原因</label><textarea className="tic-input" id="tic-action-reason" rows={3} required disabled={actionBusy} value={reason} onChange={event => { setReason(event.target.value); setReasonError('') }} aria-invalid={!!reasonError} aria-describedby={reasonError ? 'tic-reason-error' : undefined}/>{reasonError ? <p role="alert" id="tic-reason-error" className="tic-field-error">{reasonError}</p> : null}</div> : null}<Failure failure={members.actionError}/><div className="tic-actions"><button type="submit" className="tic-button tic-button-danger" disabled={actionBusy}>{actionBusy ? '处理中…' : '确认' + actionTitles[target.kind]}</button><button type="button" className="tic-button tic-button-secondary" disabled={actionBusy} onClick={() => setTarget(null)}>取消</button></div></form></Dialog> : null}
  </div>
}
