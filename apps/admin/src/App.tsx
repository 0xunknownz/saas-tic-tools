import { useMemo } from 'react'
import config from '../admin.config'
import {
  AdminProviders,
  SessionGate,
  assertValidAdminApp,
  createApiClient,
  type AdminApi,
  type AdminApp,
} from '@0xunknownz/foundation-platform-ui'

// Validate the registered modules when the app loads. A missing page, a menu pointing at an unknown
// route, a Product namespace violation or an incompatible contract version fails loudly here instead of
// rendering a half-registered console. The validator ships in the platform UI package (design 4.3).
assertValidAdminApp(config)

export function App({ app = config, client }: { app?: AdminApp; client?: AdminApi }) {
  const api = useMemo(() => client ?? createApiClient(), [client])
  return (
    <AdminProviders app={app} client={api}>
      <SessionGate />
    </AdminProviders>
  )
}
