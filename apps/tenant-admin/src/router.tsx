import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router'
import { Root } from './root'
import { MembersRoute } from './routes/members'
import { BillingRoute } from './routes/billing'

const rootRoute = createRootRoute({ component: Root })
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/members' })
  },
})
const membersRoute = createRoute({ getParentRoute: () => rootRoute, path: '/members', component: MembersRoute })
const billingRoute = createRoute({ getParentRoute: () => rootRoute, path: '/billing', component: BillingRoute })
const routeTree = rootRoute.addChildren([indexRoute, membersRoute, billingRoute])

// Served at /console/ on the web host; routes keep their bare paths and the basepath adds the prefix.
export const router = createRouter({ routeTree, basepath: '/console' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
