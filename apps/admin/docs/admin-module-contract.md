# Admin Module 接入说明

本文件说明如何通过 **Admin Module Contract v1** 把 Foundation 或 Product 的管理页面接入统一后台，
以及 Shell 的目录、路由、权限、图标与验证方式。规则来源：PRD 第 4、5 节，技术设计 7.3–7.5，
设计规范 `docs/design/foundation-admin/`。

## 1. 技术基线

| 能力 | 选型 | 说明 |
|---|---|---|
| 布局与路由 | React 19 + TanStack Router（代码式路由树） | 与 Product Web 同一个路由库。PRD 5.5 要求所有路由进入同一个 TanStack Router 实例；当前仓库还没有生成 `src/generated/routes` 的 Route Generator（独立交付项），所以路由树在 `src/shell/router.tsx` 里由模块声明生成，模块通过 `admin.config.ts` 静态导入。生成器落地后只需替换树的数据来源，页面组件不改变。 |
| 数据请求 | TanStack Query | 退出登录调用 `queryClient.clear()`，前一名管理员的数据不会残留。 |
| 无障碍组件 | Base UI（`@base-ui-components/react`，锁定版本） | ConfirmDialog / CredentialDialog 用 Base UI Dialog 提供焦点循环、Esc 关闭和 `role="alertdialog"`。 |
| 样式 | Tailwind v4 + `--ftu-` 设计令牌 | 令牌与组件样式由 `@0xunknownz/foundation-platform-ui/styles.css` 提供；`src/styles.css` 是产品样式入口，并用 `@source` 让 Tailwind 扫描已发布包的 `dist/`（npm 包不含组件源码）。颜色、间距、字体、焦点环遵循设计规范。 |
| 图标 | `lucide-react`，锁定 `1.61.0` | 只按名称静态导入；模块声明 `{ from, exportName }` 并同时传入导入的组件。没有运行时按名称查找。 |
| 离线约束 | 不加载网络字体或 CDN | 字体使用系统字体栈。安装包自带全部资源。 |

## 2. 目录结构

`apps/admin` 只保留入口与 Product 模块；Foundation 页面、shell、组件、Contract 类型与 Validator 在
`@0xunknownz/foundation-platform-ui` 包里（设计 4.2/4.3，#125）：

```text
apps/admin/
├── admin.config.ts              # 启用哪些模块（Product 维护）
├── docs/admin-module-contract.md
├── src/
│   ├── App.tsx / main.tsx       # 组合根：Providers、SessionGate、Validator
│   ├── styles.css               # 产品样式入口：导入 Foundation 样式并声明 Tailwind @source
│   └── modules/product/         # Product 模块（这里是样本）
└── vite.config.ts               # 构建时从包引入 Validator 执行 Admin Module Contract 校验
```

## 3. 声明一个模块

在 `src/modules/` 下创建目录（Product 模块建议 `src/modules/<短名>/`），编写 `index.ts`：

```ts
import { BookOpen } from 'lucide-react'
import { defineAdminModule } from '@0xunknownz/foundation-platform-ui'
import { ModelsPage } from './ModelsPage'

export const modelsModule = defineAdminModule({
  id: 'product.ai.models',
  version: '1.0.0',
  contractVersion: 1,

  requires: {
    shell: '^1.0.0',
    backendModules: ['foundation.identity'],
    backendCapabilities: ['ai.models.v1'],
  },

  permissions: [
    { key: 'platform.product.ai.models.read', description: '查看 AI 模型', scope: 'platform' },
    { key: 'platform.product.ai.models.manage', description: '管理 AI 模型', scope: 'platform' },
  ],

  routes: [
    {
      id: 'product.ai.models.list',
      path: '/admin/product/ai/models',
      page: { from: './ModelsPage', exportName: 'ModelsPage' },
      component: ModelsPage,
      permission: 'platform.product.ai.models.read',
      title: '模型管理',
    },
  ],

  navigation: [
    {
      id: 'product.ai.nav',
      title: '产品',
      order: 300,
      items: [
        {
          id: 'product.ai.models.menu',
          title: '模型管理',
          routeId: 'product.ai.models.list',
          icon: { from: 'lucide-react', exportName: 'BookOpen', Icon: BookOpen },
        },
      ],
    },
  ],
})
```

然后在 `admin.config.ts` 中静态加入：

```ts
import { modelsModule } from './src/modules/ai-models'

export default defineAdminApp({
  // ...
  modules: [...foundationModules, modelsModule],
})
```

### 字段速查

| 字段 | 必填 | 规则 |
|---|---|---|
| `id` | 是 | `foundation.*` 或 `product.*`，小写字母/数字，点分隔；全局唯一。Product 不能用 `foundation.`。 |
| `version` | 是 | 语义化版本 `x.y.z`。 |
| `contractVersion` | 是 | 必须为 `1`。 |
| `requires.shell` | 否 | Shell 版本范围，支持 `*`、精确、`^`、`~`、`>=`、`<=`；不兼容时构建失败。 |
| `requires.backendModules` | 否 | 需要的后端插件 ID（如 `foundation.tenant`）。会话返回的 `modules` 缺少它时，该模块的菜单与路由不渲染（FAS-007）。 |
| `requires.backendCapabilities` | 否 | 形如 `ai.models.v1`。`validateAdminApp(app, { capabilities })` 传入后端能力清单时校验；M3 会话上下文暂无能力清单，运行时只按 `backendModules` 判断。 |
| `permissions` | 是 | 权限键，`platform.*` / `tenant.*`；Product 只能是 `platform.product.*` 或 `tenant.product.*`。Foundation 内置权限无需重复声明。 |
| `routes[].id` | 是 | 必须以 `<module.id>.` 开头；全局唯一。 |
| `routes[].path` | 是 | `/` 开头、无尾斜杠；Foundation 在 `/admin/foundation/` 下，Product 在 `/admin/product/` 下；参数写作 `$name`。 |
| `routes[].page` | 是 | `{ from, exportName }` 静态引用；`from` 必须是模块内相对路径（`./`），禁止绝对路径和 `..`。 |
| `routes[].component` | 是 | 与 `page` 对应的静态导入组件。Route Generator 落地后由生成文件提供，页面无需改动。 |
| `routes[].permission` | 是 | 必须已声明或是 Foundation 内置权限；未声明时构建失败。 |
| `routes[].feature` | 否 | 功能权益 ID；M3 没有 Entitlement Provider，带 `feature` 的路由不会出现在菜单中（阶段 2/3 接入）。 |
| `navigation` | 否 | 省略或空数组都表示不注册菜单；路由仍然注册并受权限控制（PRD 5.2，AC-017）。 |
| `navigation[].id` | 是 | 菜单组 ID，以 `<module.id>.` 开头。 |
| `navigation[].order` | 是 | 数字。Foundation：运营 100、系统 200；Product 使用产品组（如 300）。 |
| `items[].routeId` | 是 | 指向已注册路由；否则构建失败。 |
| `items[].icon` | 否 | `{ from, exportName, Icon }`；`from` 只能是 `lucide-react` 或模块内相对路径。字符串图标名会被拒绝。 |

### 菜单合并

Shell 按 `order` 排序菜单组；`order` 与标题都相同的组合并显示（Foundation 各模块保持自己的
命名空间，但共用「运营」「系统」两组）。Product 菜单放在「产品」组。没有权限的菜单项不渲染，
也不显示成禁用状态（PRD 4.4）。

## 4. 权限与保护规则

- 前端只读取 `GET /api/admin/v1/session` 的 `permissions` 数组，不按角色名推断权限。
- 无权限的菜单和按钮不渲染；直接访问 URL 时显示 403 PageState，服务端仍然会再校验（AC-006）。
- 有权限但被业务规则拦下（最后一名超级管理员、不能停用自己）时，按钮保留 `aria-disabled`，
  用 `title` 与页内 warning Notice 说明原因。服务端返回 `409 LAST_SUPERADMIN_REQUIRED` 时
  按阻断性错误展示，而不是通用错误。
- 敏感操作一律经过 ConfirmDialog：选择目标 → 展示影响并填写原因 → 确认 → 服务端应用 → 展示实际状态。
  原因必填并写入审计。所有请求（状态变更、角色授予/撤销、凭据重置）都携带 `reason`；
  `DELETE .../roles/{role}` 的 reason 通过 query 参数传递。
- 结果以服务端为准：提交中禁用按钮；成功后重新查询该行并弹 Toast；失败保留原因并在对话框内提示；
  超时显示「结果待确认」，租户状态操作可通过 `GET .../status-operations/{operationId}` 查询结果。
  界面不会把超时当成功或失败。

## 5. 数据访问

页面通过 `useAdminClient()` 使用 `AdminApi` 接口（`src/lib/api.ts`）。HTTP 与数据类型来自
`docs/design/m3-api.md`：

- 列表：`limit` + 不透明 `cursor` + `nextCursor` + `total`，界面不拼 offset。
- Mutation 通过 `useApiMutation` 自动失效相关 query key。
- 一次性凭据（密码重置链接、邀请码）只在 CredentialDialog 显示一次，不进入查询缓存。
- 本地开发无 API 时：`VITE_ADMIN_MOCK=1 pnpm dev` 使用 `src/lib/mock.ts` 的内存数据。
  生产构建始终使用 HTTP client。

## 6. 验证

```bash
cd apps/admin
pnpm typecheck        # tsc --noEmit
pnpm test             # vitest：Contract 校验、导航/权限、错误边界、退出清理缓存、ConfirmDialog、状态四态、对比度
pnpm build            # vite build；构建开始时执行 Admin Module Contract 校验
pnpm build -- --base=/admin/   # 部署在 /admin/ 子路径；资源 URL 自动带前缀，路由仍是 /admin/...
```

生成产品的 CI 会运行 `pnpm build` 与 `pnpm typecheck`；模块不合法时构建直接失败。

### 本仓库是 Go template，注意分隔符

`saasctl init` 用 Go `text/template` 渲染 `templates/go-tanstack`，分隔符是两个左方括号与两个右方括号
（写作 `[` `[` 与 `]` `]`）。在 `apps/admin` 的模板文件里 **不能出现相邻的这两个字符**
（包括嵌套数组、正则字符类、字符串和注释）。需要嵌套数组时在括号间加空格，例如 `[ ['users'] ]`。
修改模板后运行：

```bash
grep -rn -E '\\[\\[' templates/go-tanstack/apps/admin
grep -rn -E '\\]\\]' templates/go-tanstack/apps/admin
go run ./cmd/saasctl init demo --dir /tmp/admin-check --skip-install
```

## 7. 国际化与文案

界面默认中文，文案集中在页面与组件中；PRD FAS-013 的语言包机制尚未实现。接入时沿用现有文案规范：
动词 + 对象、状态词固定（正常 / 冻结 / 停用；已绑定 / 未绑定）、时间 `YYYY-MM-DD HH:mm`、
错误说明「没发生什么、为什么、下一步」。