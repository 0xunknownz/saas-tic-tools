import type { ReactNode } from 'react'
import { Icon } from '../workbench/ui'
import './auth-entry.css'

/** Shared entry frame for tenant authentication; authentication stays in Foundation. */
export function AuthEntryLayout({ children }: { children: ReactNode }) {
  return <div className="tic-entry">
    <div className="tic-entry-panel">
      <aside className="tic-entry-intro" aria-label="TIC工具箱介绍">
        <div className="tic-entry-brand"><span className="tic-entry-logo"><Icon name="clipboard" /></span><span>TIC工具箱</span></div>
        <div className="tic-entry-story">
          <p className="tic-entry-kicker">团队的检验检测工作空间</p>
          <h2>检验检测工作，<br />从这里开始。</h2>
          <p>连接项目、标准与报告，<br className="tic-entry-desktop-break" />让团队工作井然有序。</p>
          <ul className="tic-entry-features">
            <li><Icon name="building" /><div><strong>项目协作</strong><span>汇集项目信息，协同推进每一步。</span></div></li>
            <li><Icon name="document" /><div><strong>标准查阅</strong><span>查找适用标准，让检测有据可依。</span></div></li>
            <li><Icon name="archive" /><div><strong>报告归档</strong><span>有序管理报告，留存每一份成果。</span></div></li>
          </ul>
        </div>
        <p className="tic-entry-caption">专注检验检测 · 连接团队工作</p>
      </aside>
      <div className="tic-entry-content">{children}</div>
    </div>
  </div>
}
