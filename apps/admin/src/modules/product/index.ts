import { BookOpen } from 'lucide-react'
import { defineAdminModule } from '@0xunknownz/foundation-platform-ui'
import { TestItemDetailPage } from './TestItemDetailPage.tsx'
import { TestItemsPage } from './TestItemsPage.tsx'

export const testItemsModule = defineAdminModule({
  id: 'product.testitems', version: '0.1.0', contractVersion: 1,
  requires: { shell: '^1.0.0', backendModules: ['product.testitems'], backendCapabilities: ['product.testitems.dictionary.v2'] },
  permissions: [
    { key: 'platform.product.testitems.read', description: '查询检测项目字典', scope: 'platform' },
    { key: 'platform.product.testitems.manage', description: '编辑检测项目字典', scope: 'platform' },
  ],
  routes: [
    { id: 'product.testitems.list', path: '/admin/product/test-items', page: { from: './TestItemsPage', exportName: 'TestItemsPage' }, component: TestItemsPage, permission: 'platform.product.testitems.read', feature: 'advanced', title: '检测项目字典' },
    { id: 'product.testitems.detail', path: '/admin/product/test-items/$itemId', page: { from: './TestItemDetailPage', exportName: 'TestItemDetailPage' }, component: TestItemDetailPage, permission: 'platform.product.testitems.read', feature: 'advanced', title: '检测项目详情' },
  ],
  navigation: [{ id: 'product.testitems.navigation', title: '产品业务', order: 200, items: [{ id: 'product.testitems.menu', title: '检测项目字典', routeId: 'product.testitems.list', order: 10, icon: { from: 'lucide-react', exportName: 'BookOpen', Icon: BookOpen } }] }],
})

export const productModules = [testItemsModule]
