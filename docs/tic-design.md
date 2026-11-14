# TIC 工作台设计规范

## 范围与参考

参考运行中的 TIC 原型 http://localhost:5173/。2026-11-13 对照首页和运营菜单，截图保存在本地 output/playwright/reference.png。交付 Web 应用内复用组件，不提取跨应用共享包。不迁入原型 bearer-token、登录弹窗、旧角色演示或 API 客户端。

五个重点页面：首页、公司公告、项目库、国家标准、检测报告数字档案库。其他入口为可访问的模板说明页，不提供虚假查询、授权或上传结果。运营菜单完整保留原型十个子项，管理端仍承担真实成员、账单和平台管理。

## 视觉与布局

系统中文字体；背景 #F8FAFC，面板白色；主色 #3B82F6；文字 #0F172A / #475569 / #64748B；边框 #E2E8F0。标题 19px/800，面板标题 15px/800，正文与表格 13px，控件 12px。面板、输入、导航、按钮圆角依次 16、12、10、9px。

桌面一级图标栏 72px，模块二级栏 232px，顶栏 58px，内容内边距 16px 22px 28px。首页左侧工具三列，右侧项目快捷入口与公告；线形 SVG 图标接近原型，装饰图标对辅助技术隐藏。

低于 1000px 改为单列，一级菜单成为可横向滚动的文字图标导航，二级模块通过“模块菜单”按钮展开。小于 540px 工具卡两列，详情单列，筛选和表单自适应。表格仅在自身 tic-table-wrap 内横向滚动，不挤出页面。

## 组件与状态

公共组件位于 apps/web/src/features/workbench/ui.tsx：PageHeading、Panel、Icon、SampleNotice、EmptyState、DetailMissing、Pagination。业务沿用 tic-filters / tic-table-wrap / tic-form / tic-detail。链接用于导航，按钮用于动作；悬停淡蓝底，键盘 focus-visible 蓝色轮廓，禁用按钮降低透明度且不可触发。空结果显示解释和重置路径，未知详情显示返回列表入口。

分页默认每页 5 条，过滤条件变化回到第 1 页；结果计数针对筛选结果。表单必填项与错误关联，trim 后验证；提交期间禁用，取消不保存。仅会话内新增数据必须提示刷新重置。

## URL、认证与数据

目录 apps/web/src/features/workbench/navigation.ts 是导航和快捷入口的统一来源。业务使用真实 pathname，链接支持直接访问、刷新、前进后退。根路由与 /$ 分发工作台页面，未知地址显示找不到页面；详情以稳定 ID 访问，不依赖导航内存。

现有 /login、/tenant/select 采用共享 TIC 双栏入口；登录复用 Foundation LoginPage，租户选择使用 Foundation 的公开会话 hook/API。/forgot-password、/reset-password、/invitations/accept、/mfa/enroll、/mfa/verify 完整保留 Foundation 认证页。双栏入口验收见 [验收记录](tic-auth-entry-acceptance.md)。工作台使用 useSession，只在服务器确认会话后展示；未登录保留目标 URL 的安全 redirect。错误支持重试。注销失败不伪造成功。成员和账单走 /console/members 与 /console/billing，平台管理走 /admin/，均为完整页面跳转，不修改管理端样式。

所有业务数据均明确为示例，不发送业务写请求；不声称已授权、已上传或已下载。无可用文件时仅展示附件元信息。示例日期为固定日期，不冒充实时运营数据。

## 页面补充规范

页面补充：[公告](tic-announcements-design.md)、[项目](tic-projects-design.md)、[国家标准](tic-standards-design.md)、[报告档案](tic-reports-design.md)。四页沿用本规范，筛选即时生效，所有表格可在区域内横向滚动。验收结果与范围见 [验收记录](tic-acceptance.md)。
