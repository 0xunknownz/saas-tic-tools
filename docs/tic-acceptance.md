# TIC Issues #1–#5 验收记录

## 验证环境与边界

现有 Web http://localhost:13000/、API http://localhost:18080/；未新建或替换服务器。参考原型 http://localhost:5173/ 可用，已检查首页与运营十项子菜单。浏览器为 Playwright Chromium，桌面 1280×720，窄屏 390×844。截图在本地 output/playwright/，不提交运行产物。

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
