import { Link } from '@tanstack/react-router'

// A single navigation for the tenant admin: members, billing, and the way back to the tenant front end.
export function Nav() {
  return (
    <nav className="ftu-row" aria-label="租户后台">
      <Link to="/members" className="ftu-link">成员</Link>
      <Link to="/billing" className="ftu-link">账单</Link>
      <a href="/" className="ftu-link">返回前台</a>
    </nav>
  )
}
