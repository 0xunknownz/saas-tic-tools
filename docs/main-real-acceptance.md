# main 真实运行环境验收与修复报告

验收时间：2026-11-13 至 2026-11-14（Asia/Shanghai）。记录完成于 2026-11-13T16:10:46Z。

## 基线与环境

- 基线：main 合并提交 `948fcd62a06d6f25459797c719357a5c95dc7f8e`。修复分支：`fix/main-real-env`。
- 使用当前项目已运行的 Web `http://localhost:13000/`、租户后台 `/console/`、API `http://localhost:18080/` 与 PostgreSQL `127.0.0.1:15432`。
- Playwright Chromium，桌面 1280×900，窄屏 390×844。浏览器验收未拦截或伪造认证、MFA、会话、租户接口。
- 通过 Foundation 领域服务创建两名专用验收账号和两个独立租户；密码随机生成。MFA 通过实际 HTTP 接口绑定并计算真实 TOTP，未读取或重置既有用户凭据。
- 四条业务流程按当前产品范围继续使用明确标识的本地示例数据；本次验证它们与真实认证、租户上下文的组合行为，不新增业务持久化接口。

## 发现并修复的问题

| 编号 | 问题及复现 | 修复 | 回归证据 |
|---|---|---|---|
| QA-01 | 从 `/projects/project-001` 进入登录，密码成功后跳到 `/mfa/verify`，原 redirect 消失；真实 TOTP 验证 200 后错误返回 `/`。 | 登录包装层将经过 Foundation `safeRedirect` 校验的返回地址传入 MFA 验证或注册步骤。 | 真实回归最终返回 `/projects/project-001?tab=summary#details`；路径、查询及锚点完整保留。覆盖 pending/setup 以及恶意地址的路由测试通过。 |
| QA-02 | 未登录访问 `/console/members`，认证完成后 Web 的 SPA 通配路由接管管理地址，显示“页面未找到”，文档标题仍是 Web 应用。 | 统一认证导航适配：Web 内地址走路由，`/console/`、`/admin/` 及旧成员/账单入口走完整页面导航，并保留查询和锚点。 | MFA 完成后进入真正的租户后台，文档标题为“Saas Tic Tools 租户后台”，实际成员表格显示两名验收成员。 |
| QA-03 | 管理端发现用户未选择租户时只跳 `/tenant/select`，未携带原目标；选择后固定回首页。 | 管理端守卫为登录、选租户统一保存完整原 URL；Web 租户选择页安全续接返回目标。 | `/console/members?filter=active#table` 在登录、MFA、租户选择三个中间步骤均保留，选租户后回到原管理页。外部 URL 被回退为 `/`。 |
| QA-04 | 已登录且属于 `member` 的账号访问租户管理端，session 请求 200、role 正确，但根视图为空白。 | 保留管理员权限判断，增加“没有管理权限”说明、返回工作台及切换租户入口，复用 Foundation 样式。 | 真实 member 会话在 390px 下显示可读提示与可聚焦操作；守卫及实际 Root 组件回归测试通过。 |

## 真实浏览器验收结果

| 流程 | 实际检查 | 结果 |
|---|---|---|
| 密码登录 | 错误密码显示“邮箱或密码错误”；正确密码由真实 API 返回 200/full。 | 通过 |
| MFA 注册 | 当前密码确认、启动绑定 200、真实 TOTP 完成绑定 200，显示 10 个恢复码，保存确认前禁止进入产品。 | 通过 |
| MFA 验证 | 已绑定账号登录返回 mfa_pending；错误码 401 且可重试；有效码 200；pending 阶段 session 接口 401。 | 通过 |
| 安全返回 | 业务路径及 query/hash 保留；外部 redirect 在进入 MFA 前已回退为站内 `/`。 | 通过 |
| 租户选择 | 实际列出甲、乙两个租户；选择后服务端 context 正确且刷新保留；可以切换租户。 | 通过 |
| 租户权限 | 第二账号仅看到所属的甲租户；直接请求乙租户返回 404/TENANT_NOT_FOUND；选择甲后 role 为 member。 | 通过 |
| 公告 | 8 条、两页；关键词筛选回第一页；无结果、重置、全文详情、直接访问、刷新、未知 ID。 | 通过 |
| 项目 | 9 条；关键词与地区组合；详情/未知 ID；必填错误；取消不创建；双击只新增一条且计数为 10；跨 SPA 导航保留；刷新及选租户后本地新增消失且有反馈。 | 通过 |
| 国家标准 | 9 条；编号/名称与分类组合、分页重置、空结果、稳定详情与刷新；无不可兑现的全文下载。 | 通过 |
| 报告档案 | 9 条；关键词与状态组合、分页重置、未知 ID；对应报告与附件元信息一致，无其他报告附件混入。 | 通过 |
| 窄屏与键盘 | 四条列表均在 390px 下可操作；650px 表格在 326px 容器中通过键盘横向滚动，页面宽度维持 390px。 | 通过 |
| 管理端返回链 | 登录 → MFA → 租户选择 → 原成员管理地址；管理员实际成员表格可用，账单入口实际显示“订阅与套餐”，读请求无错误。 | 通过 |
| 退出 | 普通 full 会话和中间 mfa_pending 会话均可退出；退出后四条业务均显示登录入口，不暴露列表内容。 | 通过 |
| 业务写入边界 | 列表与本地新建过程中监听真实网络，没有业务 API 写请求；租户切换是实际会话上下文写请求。 | 通过 |

## 原始验收的自动验证与证据（f29eac1）

`pnpm build`、`pnpm typecheck`、`pnpm test`、`git diff --check` 全部通过。Web：15 个测试文件、86 个测试；租户后台：3 个测试文件、10 个测试，共 96 个前端测试。

新增回归覆盖实际认证路由包装层（完整 MFA 返回链、注册分支、租户选择及非法返回地址）、跨应用目的地分类、守卫返回参数和无权限 Root 视图。修复前，相关测试分别复现 redirect 缺失、错误应用接管和空白状态；修复后通过。单元/路由测试使用 API stub，真实浏览器验收独立使用实际后端。

本地截图位于 `output/playwright/`，作为忽略的运行产物保留：

- `real-main-mfa-return-before.png` / `real-main-mfa-return-after.png`
- `real-main-console-return-before.png` / `real-main-console-return-after.png`
- `real-main-member-forbidden-before.png` / `real-main-member-forbidden-after.png`
- `real-main-notice-001-desktop.png` / `real-main-notice-001-mobile.png`
- `real-main-project-001-desktop.png` / `real-main-project-001-mobile.png`
- `real-main-standard-003-desktop.png` / `real-main-standard-003-mobile.png`
- `real-main-report-002-desktop.png` / `real-main-report-002-mobile.png`

MFA 种子、二维码、恢复码、随机密码和数据库凭据没有写入本报告或提交。

## 环境清理与交付范围

本次两名账号和两个租户按唯一 ID、邮箱、slug 校验归属后，在事务中清理；查询验证专用账号及租户剩余行数为 0，既有审计记录保留。实际 session 接口再次返回 401，测试浏览器中的 MFA 私有数据与 cookies 已清空，凭据缓存文件已删除，独立浏览器已关闭。

修复交付在独立分支供 PR 审阅；本报告不表示修复已经合入 main。main 基线的问题列表、真实回归结果及自动验证范围如上。

## PR #7 补充代码与安全审阅（2026-11-14）

审阅固定比较点：`948fcd6...f29eac1`。规范轴检查项目设计、应用边界及文件归属，未发现应修改的硬性规范违例。独立需求静态审阅未报告额外问题；主代理继续以执行性路由测试核对返回地址，确认下面一处遗漏。

### R-01：P2，规范化后的返回地址缺少最终校验（已修复）

固定版本 Foundation 0.2.1 的 `safeRedirect` 会把 `/console/..//evil.example/landing` 规范化成 `//evil.example/landing`，把 `/projects/../..//evil.example` 规范化成 `//evil.example`。这类输入原本以单斜线开头，原始检查通过后，URL 规范化改变了返回值的含义。原 PR 的直接外链测试没有覆盖这一转换。

修复前，实际 Login 路由包装层测试在 full 登录后进入 Not Found；MFA 中间路由携带了 `//evil.example/...`，没有按要求回退为 `/`。已确认的是不安全地址形式进入导航及错误返回；没有据此声称当前浏览器已成功站外跳转，也没有发现服务端权限被绕过。

修复在 Product 导航边界进行：先使用 Foundation 现有返回值策略，再校验规范化结果仍为同源、单斜线相对路径；对所有导航的规范化结果再次检查 origin 和路径，之后才判断 Web 路由或完整跨应用导航。Login 与租户选择共享此策略，MFA 直接入口的最终导航也受保护。没有修改 node_modules 或放宽后端权限。

新增回归覆盖 full 登录、MFA 中间步骤、直接 MFA 验证入口和租户选择的规范化攻击；覆盖编码遍历、双层编码斜线、反斜线、非法百分号和危险 scheme；保留合法跨应用 query/hash、opaque token 原始编码，以及编码后超过原输入长度限制的合法 MFA 中间 URL。

本次补充审阅的 `pnpm build`、`pnpm typecheck`、`pnpm test`、`git diff --check` 均通过：Web 95 个测试、租户后台 10 个测试，共 105 个前端测试。规范轴没有新增违规，需求/安全执行性补查确认 1 处问题并完成修复。

本节是补充代码/路由回归验证；上文真实 API、TOTP、租户数据、清理与截图结果仍属于原始验收记录，本轮没有重新创建账号或声称重新执行整套真实浏览器验收。
