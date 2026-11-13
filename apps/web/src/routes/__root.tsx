import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router'
import { createHttpTenantApi } from '@0xunknownz/foundation-tenant-ui'
import { TenantApiProvider } from '@0xunknownz/foundation-tenant-ui'
import { TenantUIProvider } from '@0xunknownz/foundation-tenant-ui'
import '../styles/app.css'

const api = createHttpTenantApi()

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'Saas Tic Tools' },
    ],
  }),
  component: Root,
})

function Root() {
  return (
    <html lang="zh-CN">
      <head>
        <HeadContent />
      </head>
      <body>
        <TenantUIProvider branding={{ productName: 'Saas Tic Tools', locale: 'zh-CN' }}>
          <TenantApiProvider api={api}>
            <Outlet />
          </TenantApiProvider>
        </TenantUIProvider>
        <Scripts />
      </body>
    </html>
  )
}
