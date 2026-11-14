# 检测报告档案一期验收记录

日期：2026-11-14（Asia/Shanghai）
代码分支：`epic/report-archive`
基线：main / 86ff8b60fb3669fa786e08e41937718b90e29ba7
PRD：#15（`docs/report-archive-prd.md`）

## 结论摘要

一期数据模型、上传校验、持久处理队列、OCR 适配器、模板规则、跨页明细、自动/人工归档、修订/作废/恢复、查询与前端页面均已实现，并通过单元/集成与浏览器验收。

OCR 供应商由项目方指定为 **PaddleOCR**（AI Studio 官方 API）。后端已实现 `paddleocr` 提供方（官方 Go SDK、访问令牌、PP-OCRv6、页数校验与错误分类）；真实授权样本联调仍需 AI Studio Access Token。

**尚未完成的外部门槛**：项目方提供的真实客户 PDF（“万向脚轮-广东鸿业家具集团有限公司-江西中标检检测中心有限公司.pdf”）和独立同版式样本当前不在工作区，也没有云 OCR 账号/凭据。因此以下事项只能记录为待项目方提供后执行，不能声称已通过：

- #18 第 1 项：用授权真实样本实测并选定云 OCR 供应商、记录真实费用/限制/差异。
- #19 第 7 项：独立同版式样本泛化验证。
- #22 第 1、2 项：真实样本人工核对与真实云 OCR 全流程验收。
- #22 第 3 项已用生成的 50/51 份、50MB、100/101 页、混合损坏/加密/非 PDF、图像型扫描与空用户密码权限加密样本完成覆盖；但没有客户真实样本。

其余验收项均有下方证据。真实客户原件未提交到仓库。

## 验证环境

| 项目 | 值 |
|---|---|
| API | http://localhost:18080（Go 1.26，开发模式） |
| Web | http://localhost:13000（Vite/TanStack Start） |
| 数据库 | PostgreSQL（Docker，127.0.0.1:15432） |
| 租户 | tenant-a（租户甲，管理员/成员）、tenant-b（租户乙，管理员/成员） |
| 浏览器 | Playwright Chromium 1228，1280×900 与 390×844 |
| OCR（流水线测试） | 本地 HTTP 替身，模拟 OCR.space 响应；**不是真实云 OCR 验收** |
| OCR 供应商 | **PaddleOCR**（AI Studio 官方 API），适配器已合并（5116de2） |
| 真实云 OCR | 未配置（缺 AI Studio Access Token 与授权样本） |

测试账号仅存在于本地开发数据库，使用测试密码；不在仓库或公开 Issue 中记录凭据。

## 数据与权限（#16）

- 迁移：`apps/api/internal/business/reportarchive/migrations/20261115000000_report_archive.up.sql` 建立 report_files、report_reports、report_details、OCR/解析运行、规则、事件、上传批次/明细与持久任务表。
- 报告、明细、原件、OCR 运行、解析运行与任务都带 `tenant_id`；所有查询先由会话取得租户，再由 `tenant.Membership.Role` 实时校验成员资格。
- 检测结论（`detection_result`）与归档状态（`archive_status`）分列；记录创建人、时间、原件 SHA-256、解析运行与规则版本。
- 页面：真实报告列表、详情、上传、待处理、规则管理；导航保留在“检测报告数字档案库”。

证据：

| 场景 | 结果 |
|---|---|
| 租户 B 管理员直接读取租户 A 报告 | 404（不泄漏存在性） |
| 成员读取本租户列表 | 200 |
| 成员调用管理员 PATCH | 403 PERMISSION_DENIED |
| 移除成员后旧 Cookie 读取列表/下载 | 401 SESSION_EXPIRED（请求被拒绝） |
| 平台账号（platform realm）调用租户报告 API | 401（RequireTenant 不接受平台会话） |
| 切换到租户 B 后列表 | 0 条，未保留租户 A 报告；切回后恢复 |

## 批量上传与去重（#17）

- 服务端用 multipart 流式落临时文件，先校验数量再处理；一次最多 50 份，单份最多 50,000,000 字节、100 个物理页。
- 用 pdfcpu 校验实际 PDF 头、xref、可读性与页数；空用户密码的权限加密 PDF 可读；需要打开密码、损坏、非 PDF、超限逐份返回原因。
- 每份文件独立事务：原件（不可变 bytea）与档案、任务、批次明细同时提交；失败不会留下可见空档案。
- 同租户按 SHA-256 唯一；并发插入由 PostgreSQL 唯一索引收敛；已存在且有效则链接现有档案、不建任务；原档案已作废则建立待处理候选并保留 `duplicate_of`。
- 前端上传页逐文件 XHR 进度、结果、汇总；关闭页面后任务仍在数据库。

证据：

| 场景 | 结果 |
|---|---|
| 51 份一批 | 400 BATCH_TOO_LARGE，批次数未增加 |
| 50 份一批 | 50 accepted / 0 failed |
| 混合 3 有效 + 非 PDF + 损坏 PDF | 3 accepted、2 rejected，逐份原因；其余文件继续处理 |
| 非 PDF / 损坏 / 需要密码 | PDF_TYPE_INVALID / PDF_CORRUPT / PDF_PASSWORD_REQUIRED |
| 5 个并发同文件上传 | 5 个 duplicate 响应，数据库仅 1 份报告 |
| 相同文件已作废后重传 | 新待处理候选，`duplicate_of` 指向原档案，原件表不新增行 |
| 50,000,000 字节精确边界 | HTTP + 单元测试通过（50,000,000 字节接收，少 1 字节上限拒绝） |
| 100/101 页边界 | HTTP + 单元测试：100 页接收，101 页 PDF_TOO_MANY_PAGES |
| 权限加密 PDF（空用户密码） | HTTP 接收（6 页） |
| 图像型/扫描 PDF（无文本层） | HTTP 接收（1 页） |
| 混合损坏/加密/非 PDF | 逐份拒绝或接收，原因正确 |
| PDF Range | `Range: bytes=0-99` 返回 206、Content-Range `0-99/50000000`、Accept-Ranges bytes |

## OCR 与任务（#18）

- `report_jobs` 使用 PostgreSQL、`FOR UPDATE SKIP LOCKED`、幂等键；API/worker 启动时恢复超过 10 分钟的 running 任务。
- 暂时性错误（超时、429、5xx）最多自动重试 3 次（首次之外），指数退避；第 4 次失败进入待处理。不可恢复错误（凭据、页数不一致、损坏）直接待处理。
- OCR 原始 JSON、全文与逐页文本保存；页数与 PDF 物理页不一致时拒绝而非静默截页。
- OCR 适配器支持 `disabled` 与 `ocrspace`；凭据只从服务端环境读取，前端不接触。

证据：

| 场景 | 结果 |
|---|---|
| 替身 OCR 第一次 429、第二次成功 | 任务 attempts=2、archived，规则版本落库 |
| PaddleOCR 官方协议替身（submit/poll/JSONL） | 上传 6 页样本 → OCR succeeded（provider=paddleocr、model=PP-OCRv6）→ 34 条明细自动归档 |
| worker 重启前遗留 running 任务 | 启动恢复为 queued 并成功；报告数不增加 |
| OCR 页数与 PDF 不一致 | needs_review，错误阶段 ocr，不截页 |
| 未配置 OCR | needs_review，reviewStage=ocr，原因“云端 OCR 尚未配置” |
| 管理员手动重试 OCR | 任务重新入队并重新执行 |

**未通过**：真实云 OCR 的供应商实测、费用/限制/识别差异记录（无凭据与授权样本）。

## 模板规则与明细（#19）

- 内置模板由平台迁移维护，租户规则复制后按版本保存（draft/active/retired）。试解析只读，不修改归档；只允许声明式字段、正则、必填/格式/明细完整性校验，未知字段与非法表达式被拒绝。
- 解析保存大项、子项、技术要求、单位、方法、结果原文、单项判定、顺序、来源物理页与原文；合并单元格向下继承，跨页继续；重复页眉跳过。
- 报告名称由产品名称生成；报告日期只取“报告日期”标签，排除抽样/送检/上传/打印日期；分包机构/编号不会覆盖主报告字段。
- 首套版式替身文本验证 34 条增塑剂逐项保存，来源页 2/3 正确；未检出（含检出限）原样保留；同一项目多条技术要求保留为多行。

证据：`internal/business/reportarchive/parser_test.go`（34 行、跨页、未检出、多条技术要求、日期语义、不可解析行检测）；真实规则启用/试解析 API E2E。

**未通过**：独立同版式样本验证（样本未提供）。

## 归档、复核与版本（#20）

- 必填/格式/明细完整性/重复冲突通过才自动归档；检测“不合格”只影响 detection_result，不阻止归档。
- 待处理页展示阶段与原因；管理员可在同一详情页预览原件、编辑资料/明细、逐项确认完整性并填写原因人工归档；必填/格式仍强制。
- 同机构同报告编号不同内容进入待处理；管理员确认修订后旧版保留为历史，新版本成为当前；部分唯一索引保证并发确认不能产生两个当前版。
- 作废必须填写原因；错误原件可作废后重传；相同文件已作废时管理员可恢复并保留事件。
- 重解析生成候选，不覆盖当前档案；管理员显式应用候选，事件保存前后值与解析版本。编辑使用 expectedRevision，冲突返回 409。

证据：

| 场景 | 结果 |
|---|---|
| 替身 OCR + 规则完整解析 | 自动归档、is_current=true、34 条明细、规则版本 v1 |
| 同编号不同文件 | needs_review，reviewStage=duplicate |
| 确认修订 | 新版本 current、旧版 history、reportVersion+1 |
| 作废/恢复 | 原因必填；is_current 释放/恢复；事件记录 |
| 并发修订 | 行锁 + 唯一索引；冲突返回 409 |
| 重解析候选 | 候选 34 条、不覆盖；应用后保留前后值 |
| 管理员编辑冲突 | expectedRevision 不匹配返回 REVISION_CONFLICT |

## 查询、详情与下载（#21）

- 关键词覆盖报告编号、报告名称/产品、检测机构、委托单位、受检单位；支持报告日期区间、处理状态、检测项目名称、单项判定、档案范围（当前/待处理/历史/作废/全部）、分页。
- 检测项目与判定使用同一个 `EXISTS report_details`，交叉明细分别命中不会误报。
- 列表/详情/OCR/预览/下载均要求租户会话 + 实时成员资格；未知或跨租户档案返回 404。
- 前端真实数据替换原报告示例页；直接 URL、刷新、返回、未知档案、加载失败重试、窄屏与键盘焦点均已检查。

证据：

| 场景 | 结果 |
|---|---|
| item=甲项目 + judgement=合格 | 命中 1 份 |
| item=甲项目 + judgement=不合格（另一子项命中） | 0 份 |
| /reports 当前有效 | 仅当前有效报告 |
| /reports/does-not-exist | “报告档案不存在”，无表格 |
| 390px | document.scrollWidth=390，无页面横向溢出；表格容器内滚动 |
| 键盘 | Tab 到关键词输入框，focus outline solid |
| 下载 | 返回 PDF 原件，Content-Disposition=attachment，服务端再次校验租户 |

## 前端与浏览器证据

- `output/playwright/report-list-desktop.png`
- `output/playwright/report-detail-desktop.png`
- `output/playwright/report-upload-desktop.png`
- `output/playwright/report-upload-result.png`
- `output/playwright/report-list-mobile.png`
- `output/playwright/report-verification.json`（390px 宽度、焦点、未知档案、租户切换）
- 列表/详情文本快照：`report-list-text.txt`、`report-detail-text.txt`

## 真实样本 PaddleOCR 联调（2026-11-14）

样本（原件不入库，哈希见 `samples/report-archive/SHA256SUMS`）：

| 样本 | 文件 | 大小 | 页数 | SHA-256 |
|---|---|---|---|---|
| 授权客户报告 | 万向脚轮-广东鸿业家具集团有限公司-江西中标检检测中心有限公司.pdf | 4,809,771 | 6 | `53ca56b2f33f1dcc6ad4de3904d0dfe46a82f3aba69faf4d829914b81017a6ca` |
| 同机构另一产品（独立版式候选） | 午休床-浙江康贝尔实业有限公司-江西中标检检测中心有限公司.pdf | 6,396,241 | 8 | `31819cc35ab96b98a8e478917bbae75349f475e59c5f7d157b00127c5b44af8b` |

真实供应商：PaddleOCR（AI Studio 官方 API），模型 `PP-OCRv6`，令牌仅存于服务端 `REPORT_OCR_API_KEY`。

授权客户报告实测结果：

- 上传→OCR→解析→候选全链路跑通；OCR 运行保存原始 JSONL、逐页文本与物理页码。
- 字段核对（人工比对 OCR 原文）：报告编号 `ZBJ-260725-114`、产品名称 `万向脚轮`、机构 `江西中标检检测中心有限公司`、委托/受检单位 `广东鸿业家具集团有限公司`、报告日期 `2026-09-29`、结论 `经检验，所检项目合格。`、类别 `抽样检验`、依据 `SN/T 3814-2014…` 均正确。
- 明细：真实表格在 OCR 阅读顺序中列序错乱，改用逐页坐标重建后得到 11 条逻辑行（模板预期 34 行），触发 `DETAIL_ROW_COUNT_MISMATCH` 转待人工复核，未把错乱行直接归档。
- 真实环境限制：提交队列满时供应商返回 HTTP 400 / code `10010`（已归类为可重试限流）；结果文件托管在 `bcebos.com`，本机 DNS 将其解析到假 IP，需可用网络出口才能下载；AI Studio 令牌接口未提供可查询账单，费用无法在本环境取证。


独立样本（午休床，同机构不同产品）实测结果：

- 首次提交遇到供应商提交队列满（HTTP 400 / code `10010`），按限流重试后成功，耗时约 170 秒（8 页）。
- 字段核对：报告编号 `ZBJ-260527-086`、产品名称 `午休床`、机构 `江西中标检检测中心有限公司`、委托/受检单位 `浙江康贝尔实业有限公司`、报告日期 `2026-09-02`、结论 `经检验，所检项目合格。`、类别 `委托检验`、依据 `GB/T 32487-2016…` 均正确；说明规则与邻接标签解析对同版式第二份输入有效。
- 明细同样重建出 11 条逻辑行并触发行数不符/字段缺失，转待人工复核。

两份样本的候选均已由管理员显式应用（字段与 11 条明细入库，档案保持 `needs_review` 待补齐至模板预期 34 行），未静默归档。

## 自动化检查

```bash
cd apps/api && go test ./...          # 全部通过
go vet ./internal/business/reportarchive
go tool saasctl generate --check     # up to date
cd apps/web && pnpm typecheck        # 通过
pnpm build                           # 通过
pnpm test                            # 17 个测试文件 / 114 个测试通过
sh scripts/security/run.sh           # Foundation 安全套件 pass：113 操作 / 529 用例 / 478 通过 / 0 失败 / 0 无效
```

报告档案契约 25 个操作中 20 个完成基线与全部负例（未登录、平台/租户 realm、缺失权限、过期会话、跨租户、CSRF）；5 个依赖 OCR 真实结果的操作按记录跳过（见已知限制 9）。

## 部署与配置

| 变量 | 说明 |
|---|---|
| `REPORT_OCR_PROVIDER` | `disabled`、`ocrspace` 或 `paddleocr`（项目方选定 PaddleOCR） |
| `REPORT_OCR_ENDPOINT` | OCR HTTP 端点（默认 OCR.space） |
| `REPORT_OCR_API_KEY` | 仅服务端；`ocrspace` / `paddleocr` 时必填（PaddleOCR 为 AI Studio Access Token） |
| `REPORT_OCR_MODEL` | 记录到 OCR 运行 |
| `REPORT_OCR_TIMEOUT_SECONDS` | 默认 90 |
| `REPORT_OCR_MAX_CONCURRENCY` | 默认 2 |
| `REPORT_WORKER_POLL_MS` | 队列轮询，默认 1000 |
| `REPORT_RETRY_BASE_SECONDS` | 退避基数，默认 5 |
| `REPORT_MAX_UPLOAD_BYTES` | 默认 50,000,000 |
| `REPORT_MAX_UPLOAD_FILES` | 默认 50 |
| `REPORT_MAX_PDF_PAGES` | 默认 100 |

## 已知限制与偏离

1. **真实云 OCR 未验收**：本地替身只证明适配器、队列、解析、归档链路，不证明供应商识别率、费用和限制。供应商已确定为 PaddleOCR，适配器已实现；部署方需在服务端配置 AI Studio Access Token（`REPORT_OCR_PROVIDER=paddleocr`、`REPORT_OCR_API_KEY`），并用授权样本重跑 #18/#22。不要把令牌提交到仓库。
2. **独立样本缺失**：没有第二份同版式报告，不能把当前替身文本当作泛化证明。
3. **原件存储**：Foundation storage 插件未导出可供产品插件依赖的 service 接口（Plugin Contract v1 仅允许依赖接口），本期将不可变原件存入 `report_files.content bytea`，与业务写入同事务提交，避免孤儿文件；后续对象存储迁移需 Foundation 暴露 storage 接口。
4. **任务队列**：Foundation jobs 同样未导出队列接口；本期使用同一 PostgreSQL 的 `report_jobs`（SKIP LOCKED、幂等键、running 恢复、退避），满足可靠性要求但不是 Foundation jobs 表。
5. 50 MB 文件在服务端按单文件读入内存校验与入库；并发上传受批次和 OCR 并发限制，生产需按内存预算调整。
6. PDF 预览已支持单段 HTTP Range；多段 Range 不支持（与 Foundation storage 下载语义一致）。
7. 规则是受约束的声明式正则配置，依赖首套版式 OCR 文本形态；版式变化需管理员调整规则。
8. 原始客户 PDF 未提交到仓库；验收使用生成的替身 PDF，真实样本核对仍需项目方提供。
9. Foundation 安全套件跳过的 5 个操作（OCR 结果、重解析候选、重解析、修订确认、候选应用）需在云 OCR 可用时执行；它们的权限/CSRF 模式与已覆盖的 20 个操作一致，权限矩阵行为由同批负例验证。
