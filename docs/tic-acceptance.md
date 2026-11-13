# TIC Issues #1–#5 验收记录

## 验证环境与边界

现有 Web http://localhost:13000/、API http://localhost:18080/；未新建或替换服务器。参考原型 http://localhost:5173/ 可用，已检查首页与运营十项子菜单。浏览器为 Playwright Chromium，桌面 1280×720 / 1280×900，窄屏 390×844。截图在本地 output/playwright/，不提交运行产物。

浏览器认证流程使用隔离的 Playwright 网络响应夹具（仅 login 与 session），通过原有登录表单进入工作台；没有生产认证后门或业务 API mock 开关。真实 API 未登录返回 401 已验证。没有实际账号凭据，因此未声称验证真实服务器账号的密码/MFA。现有登录、租户、重置、邀请、MFA 和安全 redirect/token 契约由自动测试验证。

## #1 工作台与完整导航

- [x] 原有登录表单成功后进入 TIC 首页，工具/项目快捷入口与菜单 URL 一致。
- [x] 32 个唯一业务 URL 可直接打开；国家标准路径刷新、后退、前进及选中状态正确；未知 URL 有明确提示。
- [x] 保留全部认证路由与原参数；现有安全 redirect、登录/MFA、重置、邀请和租户测试均通过。
- [x] 成员、账单和平台端为 /console/members、/console/billing、/admin/ 的完整跳转，管理端代码未修改。
- [x] docs/tic-design.md 记录全部视觉、布局、状态、响应式、URL 和示例数据边界。
- [x] 非重点入口均有模板说明，所有菜单可操作；运营十个子菜单齐全。
- [x] 首页与原型同层级，同视觉参数，初始公共组件可复用。
- [x] 1280px 桌面与 390px 窄屏截图已检查；模块菜单可展开，页面无横向溢出，focus-visible 样式可见。
- [x] 全局本地示例提示明确；项目入口提示会话及刷新重置；没有业务写请求、授权或上传成功反馈。
- [x] Web typecheck、生产 build、13 个测试文件共 61 个测试通过，含 5 个工作台集成测试。浏览器日志仅首次真实会话的预期 401 与开发提示，无运行异常。

证据：output/playwright/reference.png、workbench-desktop.png、workbench-mobile.png。认证浏览器夹具的边界见本文开头。

## #2 公司公告

- [x] 从导航进入公告列表，标题、发布日期与全文入口清晰；首页预览使用相同数据源。
- [x] 8 条示例分 2 页；浏览器在第 2 页输入“归档字段”后回到第 1 页，计数为 1；无匹配后重置恢复 8 条。
- [x] notice-001 点击、直接访问和刷新均显示对应标题、日期与全文，返回路径明确。
- [x] 未知公告显示“公告不存在”，无结果显示空状态，没有误显其他正文。
- [x] 复用工作台组件，并补充 tic-announcements-design.md。
- [x] 桌面与 390px 列表、详情截图已检查；表格 650px 内容在 326px 区域内滚动，页面宽度保持 390px。
- [x] 全部为示例公告，无业务写请求或虚假操作反馈。
- [x] 非平凡交互测试、类型检查、构建与浏览器检查通过。

证据：announcement-detail-desktop.png、announcements-mobile.png、announcement-detail-mobile.png。

## #3 项目查询、新建与详情

- [x] 9 条固定示例，关键词与地区组合筛选、分页和计数一致；筛选变更回到第一页，空结果与重置可用。
- [x] project-001 支持点击、直接打开与刷新；项目库导航在详情仍正确选中，未知 ID 明确反馈。
- [x] 名称、地区、关联单位及预算标注必填；空白、非法地区、负数/非有限预算、过长输入会被拒绝。取消不创建；有效提交 trim 后进入列表，打开详情数据一致。
- [x] 新增仅当前会话有效，跨 SPA 页面仍保留；刷新本地新增详情后显示“项目不存在”与重置说明。切换身份清空存储。
- [x] 连续两次表单提交的自动测试与浏览器双击均只新增 1 条；计数从 9 变 10。错误与字段关联，无结果反馈明确。
- [x] 复用工作台组件，并补充 tic-projects-design.md。
- [x] 桌面与 390px 表格/表单/详情可用；Tab 从名称进入地区选择，焦点轮廓为 solid，页面宽度保持 390px。
- [x] 示例及本地新增标识清楚；浏览器请求监听确认整个新增操作没有任何 /api/ 非只读请求。
- [x] 类型检查、构建、行为测试与浏览器验收通过。

证据：projects-desktop.png、project-detail-desktop.png、project-form-mobile.png、project-local-detail-mobile.png、projects-mobile.png。

## #4 国家标准

- [x] 9 条示例标准展示编号、名称、分类、状态；编号和详情均显式标注示例，不冒充权威目录。
- [x] 关键词“采样”与“环境检测”组合得 1 条，改为“食品检测”得 0 条；分页、重置及计数正确，条件变化回第一页。
- [x] standard-003 对应编号和摘要正确，直接访问、刷新及返回查询可用；国家标准子菜单保持选中。
- [x] 未知标准与无结果均明确反馈；没有全文下载按钮或虚假成功提示。
- [x] 复用工作台组件，补充 tic-standards-design.md。
- [x] 桌面和 390px 列表/详情可用；宽表 650px 在 326px 容器内滚动，页面无横向溢出。
- [x] 全部为本地示例，只读，不连接外部标准数据库或发送业务写请求。
- [x] 类型检查、构建、交互测试与浏览器验收通过。

证据：standards-desktop.png、standard-detail-desktop.png、standards-mobile.png、standard-detail-mobile.png。

## #5 检测报告档案

- [x] 9 条报告展示报告编号、名称、关联对象、日期与状态；报告和附件均明确示例。
- [x] “饮用水”与“待补充”组合得 1 条，改为“已归档”得 0 条；分页、重置、计数及页码重置通过。
- [x] report-002 直接访问和刷新均显示对应报告及水质附件，返回列表可用；未知档案明确反馈。
- [x] 仅展示附件名称、格式、大小与说明；没有上传/下载操作或虚假文件成功提示。
- [x] 390px 下报告字段与附件均可读；附件区域键盘横向滚动成功，未混入 report-001 的文件信息；report-006 展示无附件元信息状态。
- [x] 复用工作台组件，补充 tic-reports-design.md。
- [x] 桌面与窄屏列表、详情、附件表格截图已检查，页面宽度保持 390px。
- [x] 本地只读示例，无业务写请求、授权或上传成功模拟。
- [x] 类型检查、构建、行为测试与浏览器验收通过。

证据：reports-desktop.png、report-detail-desktop.png、reports-mobile.png、report-detail-mobile.png。

## 最终集成验证（2026-11-13T11:08:29Z）

pnpm build、pnpm typecheck、pnpm test 和 git diff --check 全部通过。Web 14 个测试文件 / 78 个测试，租户管理端 2 个测试文件 / 8 个测试，共 86 个测试通过。平台管理端构建与类型检查通过，仓库本身未配置其测试文件。

实际点击成员与账单后，浏览器分别到达 http://localhost:13000/console/members 与 /console/billing，文档标题为“Saas Tic Tools 租户后台”（原应用）。开发服务新增 /console/ 代理与 /admin/ 跳转；平台入口到达 http://localhost:15173/admin/，标题为“Saas Tic Tools Admin”。两个管理端代码与样式未修改；生产继续使用 edge 的同域路由。

测试输出中的安全 redirect 拒绝信息、jsdom scrollTo 未实现信息及现有 Admin 大包提示不构成失败。没有新增依赖、业务 API、上传或标准下载能力。真实账号密码/MFA 的验证边界见开头。

## GitHub 交付与 CI 后续修复

草稿 PR：https://github.com/0xunknownz/saas-tic-tools/pull/6 。Issues #1–#5 已全部关闭，44 项复选框均勾选。

初次 GitHub Actions 运行 43414873246 未通过：api / security 在空 Foundation 凭据的强制检查处退出，frontend 下载 npm 包返回 401。后续检查确认 Foundation 源码实际公开，v0.2.1 可匿名读取；工作流已删除不必要的源码凭据门槛，并让前端默认使用只读 GITHUB_TOKEN 下载包，保留 FOUNDATION_READ_TOKEN 可选覆盖。修复后的运行 [43416328629](https://github.com/0xunknownz/saas-tic-tools/actions/runs/43416328629) 已验证 api 与 frontend 通过且未配置额外 secret。详细诊断、认证边界与备用 token 配置步骤见 [CI 指南](ci.md)；安全任务结果以该运行记录和 PR 最新提交检查为准。
