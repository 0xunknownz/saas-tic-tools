import { AppLink, DetailMissing, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import { useCatalog } from '../workbench/useCatalog'
import { standardCategories, standards } from './data'

export function StandardsPage({ path }: { path: string }) {
  if (path !== '/standards') {
    const record = standards.find(item => path === '/standards/' + item.id)
    if (!record) return <DetailMissing kind="标准" backHref="/standards" />
    return <><PageHeading title="标准详情" action={<AppLink className="tic-button secondary" to="/standards">返回标准查询</AppLink>} /><Panel><article className="tic-detail"><header><span className="tic-tag">示例标准 · 非权威目录</span><h1>{record.name}</h1></header><dl><div><dt>标准编号</dt><dd>{record.code}</dd></div><div><dt>分类</dt><dd>{record.category}</dd></div><div><dt>状态</dt><dd>{record.status}</dd></div><div><dt>发布日期（示例）</dt><dd>{record.published}</dd></div><div><dt>实施日期（示例）</dt><dd>{record.implemented}</dd></div><div><dt>资料来源</dt><dd>本地模板示例，无外部数据库连接</dd></div></dl><section><h2>示例摘要</h2><p>{record.summary}</p></section><p className="tic-muted">不提供标准全文或下载文件。编号、状态、日期与摘要均仅供演示；正式使用前请在权威标准平台核实。</p></article></Panel></>
  }
  return <StandardList />
}
function StandardList() {
  const catalog = useCatalog(standards, item => item.code + item.name, item => item.category)
  return <><PageHeading title="国家标准" description="按编号、名称与分类查询本地示例标准。非权威目录，不提供标准全文。" /><Panel><form className="tic-filters" role="search" onSubmit={event => event.preventDefault()}><label>标准关键词<input type="search" value={catalog.keyword} placeholder="标准编号或名称" onChange={event => catalog.setKeyword(event.target.value)} /></label><label>标准分类<select value={catalog.category} onChange={event => catalog.setCategory(event.target.value)}><option value="">全部分类</option>{standardCategories.map(category => <option key={category}>{category}</option>)}</select></label><button type="button" className="tic-button secondary" onClick={catalog.reset}>重置</button></form>{catalog.filtered.length ? <div className="tic-table-wrap" role="region" aria-label="标准列表，可横向滚动" tabIndex={0}><table><thead><tr><th scope="col">标准编号</th><th scope="col">标准名称</th><th scope="col">分类</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{catalog.visible.map(item => <tr key={item.id}><td>{item.code}</td><td><AppLink to={'/standards/' + item.id}>{item.name}</AppLink> <span className="tic-tag">示例</span></td><td>{item.category}</td><td>{item.status}</td><td><AppLink to={'/standards/' + item.id} aria-label={'查看标准：' + item.name}>查看详情</AppLink></td></tr>)}</tbody></table></div> : <EmptyState />}<Pagination page={catalog.page} total={catalog.filtered.length} onChange={catalog.setPage} /></Panel></>
}
