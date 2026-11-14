import { useRef, useState, type FormEvent } from 'react'
import { ErrorAlert, useMessages, usePasswordReset, type NavigateFn } from '@0xunknownz/foundation-tenant-ui'
import { AuthEntryLayout } from './AuthEntryLayout'

export function TicForgotPasswordPage({ navigate }: { navigate: NavigateFn }) {
  const { request } = usePasswordReset()
  const t = useMessages()
  const [email, setEmail] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)
  const submitting = useRef(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting.current || request.pending) return
    const input = event.currentTarget.elements.namedItem('email') as HTMLInputElement
    const error = !email.trim() ? '请输入账号邮箱。' : input.validity.typeMismatch ? '请输入有效的邮箱地址，例如 name@company.com。' : null
    setFieldError(error)
    if (error) { input.focus(); return }
    submitting.current = true
    try { await request.submit(email) }
    finally { submitting.current = false }
  }

  return <AuthEntryLayout>
    <main className="tic-entry-password" aria-labelledby="tic-request-title">
      <header className="tic-entry-heading"><h1 id="tic-request-title">找回密码</h1><p>输入账号邮箱，我们会向你发送密码重置链接。</p></header>
      {!request.sent ? <form className="ftu-stack" data-ftu-form="password-request" aria-label="找回密码" aria-busy={request.pending} noValidate onSubmit={event => void submit(event)}>
        <div><label className="ftu-label" htmlFor="tic-request-email">邮箱</label>
          <input className="ftu-input" id="tic-request-email" name="email" type="email" autoComplete="username" placeholder="请输入工作邮箱" required value={email} onChange={event => { setEmail(event.target.value); setFieldError(null) }} aria-invalid={fieldError ? true : undefined} aria-describedby={fieldError ? 'tic-request-field-error' : undefined} />
          {fieldError ? <p className="tic-entry-field-error" id="tic-request-field-error" role="alert">{fieldError}</p> : null}
        </div>
        {request.error ? <ErrorAlert failure={request.error}>{request.error.messageKey ? t(request.error.messageKey) : '暂时无法发送重置请求，请稍后重试。'}</ErrorAlert> : null}
        <button className="ftu-button" type="submit" disabled={request.pending}>{request.pending ? '发送中…' : '发送重置链接'}</button>
      </form> : <p className="ftu-alert ftu-alert-success" role="status">如果该邮箱对应一个账号，重置链接已发送。请查收邮件。</p>}
      <div className="tic-entry-password-back"><a className="ftu-link" href="/login" onClick={event => {
        if (!event.defaultPrevented && event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey) { event.preventDefault(); navigate('/login') }
      }}>返回登录</a></div>
      <p className="tic-entry-password-help">如未收到邮件，请检查垃圾邮件文件夹。</p>
    </main>
  </AuthEntryLayout>
}
