import { ADMIN_SHELL_VERSION, defineAdminApp, foundationModules } from '@0xunknownz/foundation-platform-ui'
import { productModules } from './src/modules/product/index.ts'

// Admin modules are listed here and imported statically (technical design 7.3, 7.4). To add a Product
// module, import its definition and append it to modules; do not remove Foundation modules.
// The module shape and the validation rules are documented in docs/admin-module-contract.md.
export default defineAdminApp({
  appId: 'saas-tic-tools.admin',
  shellVersion: ADMIN_SHELL_VERSION,
  branding: { name: 'Saas Tic Tools Admin' },
  modules: [...foundationModules, ...productModules],
})
