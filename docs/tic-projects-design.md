# TIC 项目页面规范

本页继承 [公共设计规范](tic-design.md) 的颜色、字号、圆角、线形图标、桌面三栏与窄屏单列规则，组件不跨应用发布。

项目列表关键词（名称、编号、关联单位）与地区组合即时筛选；每页 5 条。项目详情 URL /projects/project-001；新建 URL /projects/new；项目溯源等模板菜单保留独立路径。新建必填名称 2–100 字、关联单位 2–80 字、受控地区、有限正数预算（不超过 10 亿万元），说明最多 2000 字。输入 trim 后保存，错误以 aria-invalid / aria-describedby 关联，取消不保存，同步 ref 锁阻止连续提交重复创建。创建后回列表且新记录在首行，可看详情。ProjectsProvider 挂在登录保护内，以账号与租户作为 key；仅内存保存，刷新、退出或切换身份清空。不使用 localStorage，不调用项目 API。local UUID 刷新后访问显示记录不存在和刷新重置说明。

复用 PageHeading、Panel、AppLink、Pagination、EmptyState、DetailMissing；列表使用 tic-filters / tic-table-wrap，详情使用 tic-detail。所有表格区域可通过 Tab 聚焦后横向滚动；表头有 scope，控件有可见 label，分页边界按钮禁用，空结果保留筛选与重置。小于 1000px 时筛选换行、详情单列；不对宽表格强行压缩。

所有数据由本地 TypeScript 示例提供，顶端统一显示示例提示，不发送业务写请求。自动测试覆盖非平凡状态转换，浏览器检查与证据见 [验收记录](tic-acceptance.md)。
