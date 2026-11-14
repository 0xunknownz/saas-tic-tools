import type { ComponentProps } from 'react'
import { LoginPage, TenantUIProvider } from '@0xunknownz/foundation-tenant-ui'
import { AuthEntryLayout } from './AuthEntryLayout'

export function TicLoginPage(props: ComponentProps<typeof LoginPage>) {
  return <AuthEntryLayout>
    <TenantUIProvider branding={{ productName: '' }} messages={{
      'login.title': '欢迎回来',
      'login.subtitle': '登录账号，进入你的检验检测工作空间',
    }}>
      <div onSubmitCapture={(event) => {
        // A pending form must also ignore repeated Enter submissions.
        if (event.target instanceof HTMLFormElement && event.target.querySelector('[data-ftu-security="submit"]:disabled')) {
          event.preventDefault()
          event.stopPropagation()
        }
      }}>
        <LoginPage {...props} className="tic-entry-login" />
      </div>
    </TenantUIProvider>
  </AuthEntryLayout>
}
