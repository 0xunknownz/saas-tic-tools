import { AppLink, DetailMissing, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import { useCatalog } from '../workbench/useCatalog'
import { reports, reportStatuses } from './data'

export function ReportsPage({ path }: { path: string }) {
  if (path !== '/reports') {
    const record = reports.find(item => path === '/reports/' + item.id)
    if (!record) return <DetailMissing kind="报告档案" backHref="/reports" />
    return <><PageHeading title="报告档案详情" action={<AppLink className="tic-button secondary" to="/reports">返回报告列表</AppLink>} /><div className="tic-detail"><Panel><span className="tic-tag">示例报告 · {record.status}</span><h1>{record.name}</h1><dl><div><dt>报告编号</dt><dd>{record.number}</dd></div><div><dt>关联对象</dt><dd>{record.object}</dd></div><div><dt>出具日期（示例）</dt><dd>{record.issued}</dd></div><div><dt>出具机构（示例）</dt><dd>{record.organization}</dd></div></dl><p>{record.summary}</p></Panel><Panel title="附件信息"><p className="tic-muted">以下仅展示示例附件元信息，未提供可用文件，不支持上传或下载。</p>{record.attachments.length ? <div className="tic-table-wrap" role="region" aria-label="示例附件信息，可横向滚动" tabIndex={0}><table><thead><tr><th scope="col">示例附件名称</th><th scope="col">格式</th><th scope="col">大小（示例）</th><th scope="col">说明</th></tr></thead><tbody>{record.attachments.map(file => <tr key={file.name}><td>{file.name} <span className="tic-tag">示例附件</span></td><td>{file.format}</td><td>{file.size}</td><td>{file.description}</td></tr>)}</tbody></table></div> : <EmptyState title="暂无示例附件信息"><p>此报告尚未提供附件元信息。</p></EmptyState>}</Panel></div></>
  }
  return <ReportList />
}
function ReportList() {
  const catalog = useCatalog(reports, item => item.number + item.name + item.object, item => item.status)
  return <><PageHeading title="检测报告数字档案库" description="按报告编号、名称或关联对象定位本地示例档案。" /><Panel><form className="tic-filters" role="search" onSubmit={event => event.preventDefault()}><label>报告关键词<input type="search" value={catalog.keyword} placeholder="报告编号、名称或关联对象" onChange={event => catalog.setKeyword(event.target.value)} /></label><label>报告状态<select value={catalog.category} onChange={event => catalog.setCategory(event.target.value)}><option value="">全部状态</option>{reportStatuses.map(status => <option key={status}>{status}</option>)}</select></label><button className="tic-button secondary" type="button" onClick={catalog.reset}>重置</button></form>{catalog.filtered.length ? <div className="tic-table-wrap" role="region" aria-label="报告列表，可横向滚动" tabIndex={0}><table><thead><tr><th scope="col">报告编号</th><th scope="col">报告名称</th><th scope="col">关联对象</th><th scope="col">出具日期</th><th scope="col">状态</th><th scope="col">档案</th></tr></thead><tbody>{catalog.visible.map(item => <tr key={item.id}><td>{item.number}</td><td><AppLink to={'/reports/' + item.id}>{item.name}</AppLink> <span className="tic-tag">示例</span></td><td>{item.object}</td><td>{item.issued}</td><td>{item.status}</td><td><AppLink to={'/reports/' + item.id} aria-label={'查看档案：' + item.name}>查看档案</AppLink></td></tr>)}</tbody></table></div> : <EmptyState />}<Pagination page={catalog.page} total={catalog.filtered.length} onChange={catalog.setPage} /></Panel></>
}
