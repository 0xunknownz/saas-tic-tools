# TIC 公告页面规范

本页继承 [公共设计规范](tic-design.md) 的颜色、字号、圆角、线形图标、桌面三栏与窄屏单列规则，组件不跨应用发布。

公告列表按标题、分类与正文关键词即时筛选；每页 5 条；关键词变化重置分页。首页三条公告直接使用同一数据源并链接对应全文。稳定 URL /announcements/notice-001 等支持直接打开与刷新，详情包含标题、固定日期、分类、发布方和完整段落。未知 ID 显示公告不存在与返回列表。正文不接受 HTML，避免注入；全文为示例规则，不作为组织正式制度。

复用 PageHeading、Panel、AppLink、Pagination、EmptyState、DetailMissing；列表使用 tic-filters / tic-table-wrap，详情使用 tic-detail。所有表格区域可通过 Tab 聚焦后横向滚动；表头有 scope，控件有可见 label，分页边界按钮禁用，空结果保留筛选与重置。小于 1000px 时筛选换行、详情单列；不对宽表格强行压缩。

所有数据由本地 TypeScript 示例提供，顶端统一显示示例提示，不发送业务写请求。自动测试覆盖非平凡状态转换，浏览器检查与证据见 [验收记录](tic-acceptance.md)。
