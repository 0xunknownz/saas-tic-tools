import type { AnchorHTMLAttributes, ReactNode } from 'react'
import { Link as RouterLink } from '@tanstack/react-router'

// Product destinations are resolved by the catch-all route, while auth retains its typed routes.
export function AppLink({ to, ...props }: Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & { to: string }) {
  return <RouterLink to={to} {...props} />
}
const Link = AppLink

export function Icon({ name = 'document' }: { name?: string }) {
  const paths: Record<string, string> = {
    home: 'M3 10 12 3l9 7M5 9v12h14V9M9 21v-7h6v7',
    bell: 'M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4',
    clipboard: 'M9 4H5v17h14V4h-4M9 2h6v4H9zM8 11h8M8 15h8',
    building: 'M3 21V7l9-4v18M12 11l8-3v13M1 21h22M6 9h2M6 13h2M6 17h2M15 13h2M15 17h2',
    archive: 'M3 4h18v4H3zM5 8v13h14V8M9 12h6',
    shield: 'M12 2 3 6v7c0 5 9 9 9 9s9-4 9-9V6zM8 12l3 3 5-6',
    compass: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0M16 8l-3 5-5 3 3-5z',
    headset: 'M3 13v-2a9 9 0 0 1 18 0v2M3 12h4v8H3zM17 12h4v8h-4z',
    search: 'M20 20l-5-5M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0',
    document: 'M5 2h10l4 4v16H5zM14 2v5h5M8 11h8M8 15h8M8 18h5',
    sliders: 'M2 6h20M2 18h20M7 3v6M17 15v6',
  }
  return <svg className="tic-icon" aria-hidden="true" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d={paths[name] ?? paths.document} /></svg>
}
export function PageHeading({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return <div className="tic-page-heading"><div><h1>{title}</h1>{description ? <p>{description}</p> : null}</div>{action}</div>
}
export function Panel({ title, eyebrow, action, children, className = '' }: { title?: string; eyebrow?: string; action?: ReactNode; children: ReactNode; className?: string }) {
  return <section className={'tic-panel ' + className}>{title ? <div className="tic-panel-heading"><div>{eyebrow ? <p className="tic-eyebrow">{eyebrow}</p> : null}<h2>{title}</h2></div>{action}</div> : null}{children}</section>
}
export function EmptyState({ title = '没有匹配结果', children }: { title?: string; children?: ReactNode }) {
  return <div className="tic-empty" role="status"><Icon name="search" /><h2>{title}</h2>{children ?? <p>请调整筛选条件，或重置后查看全部示例数据。</p>}</div>
}
export function DetailMissing({ kind, backHref }: { kind: string; backHref: string }) {
  return <Panel><EmptyState title={kind + '不存在'}><p>该记录可能不存在，或为刷新后已重置的本地新增记录。</p><Link className="tic-button" to={backHref}>返回列表</Link></EmptyState></Panel>
}
export function Pagination({ page, total, pageSize = 5, onChange }: { page: number; total: number; pageSize?: number; onChange: (page: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  return <nav className="tic-pagination" aria-label="结果分页"><span role="status">共 {total} 条 · 第 {page} / {pages} 页</span><div><button className="tic-button secondary" disabled={page <= 1} onClick={() => onChange(page - 1)}>上一页</button><button className="tic-button secondary" disabled={page >= pages} onClick={() => onChange(page + 1)}>下一页</button></div></nav>
}
export function SampleNotice({ local = false, real = false }: { local?: boolean; real?: boolean }) {
  if (real) return <aside className="tic-notice" role="note"><strong>真实业务数据</strong><span>本页连接当前租户的报告档案 API；所有读写均受服务端租户与成员权限校验。</span></aside>
  return <aside className="tic-notice" role="note"><strong>示例数据 · 用于模板演示</strong><span>记录、状态与数量均为本地示例，不代表实际业务。{local ? '新增数据仅保存在当前会话，刷新后重置；请勿填写真实信息。' : '不发送真实业务写请求。'}</span></aside>
}
