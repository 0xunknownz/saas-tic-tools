import { AppLink, DetailMissing, EmptyState, PageHeading, Pagination, Panel } from '../workbench/ui'
import { useCatalog } from '../workbench/useCatalog'
import { announcements } from './data'

export function AnnouncementsPage({ path }: { path: string }) {
  if (path !== '/announcements') {
    const record = announcements.find(item => path === '/announcements/' + item.id)
    if (!record) return <DetailMissing kind="公告" backHref="/announcements" />
    return <><PageHeading title="公告详情" action={<AppLink className="tic-button secondary" to="/announcements">返回公告列表</AppLink>} /><Panel><article className="tic-detail"><header><span className="tic-tag">示例公告 · {record.category}</span><h1>{record.title}</h1><p className="tic-muted">发布日期：<time dateTime={record.date}>{record.date}</time> · 发布方：示例资料管理组</p></header>{record.paragraphs.map(text => <p key={text}>{text}</p>)}</article></Panel></>
  }
  return <AnnouncementList />
}
function AnnouncementList() {
  const catalog = useCatalog(announcements, item => item.title + item.category + item.paragraphs.join(''))
  return <><PageHeading title="公司公告" description="查找制度更新、资料规范与工具指引。以下均为本地示例公告。" /><Panel><form className="tic-filters" onSubmit={event => event.preventDefault()} role="search"><label>公告关键词<input type="search" value={catalog.keyword} placeholder="搜索标题或正文" onChange={event => catalog.setKeyword(event.target.value)} /></label><button className="tic-button secondary" type="button" onClick={catalog.reset}>重置</button></form>{catalog.filtered.length ? <div className="tic-table-wrap" role="region" aria-label="公告列表，可横向滚动" tabIndex={0}><table><thead><tr><th scope="col">公告标题</th><th scope="col">分类</th><th scope="col">发布日期</th><th scope="col">阅读</th></tr></thead><tbody>{catalog.visible.map(item => <tr key={item.id}><td><AppLink to={'/announcements/' + item.id}>{item.title}</AppLink> <span className="tic-tag">示例</span></td><td>{item.category}</td><td><time dateTime={item.date}>{item.date}</time></td><td><AppLink to={'/announcements/' + item.id} aria-label={'阅读全文：' + item.title}>阅读全文</AppLink></td></tr>)}</tbody></table></div> : <EmptyState />}<Pagination page={catalog.page} total={catalog.filtered.length} onChange={catalog.setPage} /></Panel></>
}
