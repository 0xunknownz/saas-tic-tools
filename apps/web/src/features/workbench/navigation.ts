export interface NavItem { label: string; href: string; description?: string }
export interface NavGroup extends NavItem { icon: string; items?: NavItem[] }
export const projectItems: NavItem[] = [
  { label: '项目库', href: '/projects', description: '关注和参与项目的资料留底' },
  { label: '项目溯源', href: '/projects/sources', description: '公共资源与政府采购网站导航' },
  { label: '投标登记', href: '/projects/bids', description: '投标、报价文件和内部备注' },
  { label: '采购意向', href: '/projects/intentions', description: '关注采购需求与计划' },
  { label: '质疑与投诉', href: '/projects/complaints', description: '案件材料、期限和处理记录' },
  { label: '结果归档', href: '/projects/archives', description: '结果公告与合同文件留底' },
]
export const toolItems: NavItem[] = [
  { label: '真实查询', href: '/tools/verification', description: '来源记录查询与核验辅助' },
  { label: '检测机构', href: '/tools/laboratories', description: '按机构或检测能力查询' },
  { label: '资质查询', href: '/tools/qualifications', description: '查询企业证书与认证范围' },
  { label: '资质证书', href: '/tools/certificates' },
  { label: '国家标准', href: '/standards' },
  { label: '法律法规', href: '/tools/regulations' },
  { label: '技术参数与评分标准', href: '/tools/scoring' },
  { label: '投诉案例', href: '/tools/cases' },
  { label: '专家、投标单位、中标单位', href: '/tools/participants' },
]
export const operationItems: NavItem[] = [
  ['用户管理', 'users'], ['用户审核', 'reviews'], ['站点管理', 'sites'], ['角色与权限', 'roles'],
  ['模块开通与菜单配置', 'modules'], ['附件管理', 'attachments'], ['操作审计', 'audit'],
  ['登录日志', 'logins'], ['系统设置', 'settings'], ['企业认证/授权', 'certification'],
].map(([label, slug]) => ({ label, href: '/operations/' + slug }))
export const navGroups: NavGroup[] = [
  { label: '工具首页', href: '/', icon: 'home' },
  { label: '公司公告', href: '/announcements', icon: 'bell' },
  { label: '项目中心', href: '/projects', icon: 'clipboard', items: projectItems },
  { label: '数据工具中心', href: '/tools', icon: 'building', items: toolItems },
  { label: '检测报告数字档案库', href: '/reports', icon: 'archive' },
  { label: '运营与管理', href: '/operations', icon: 'shield', items: operationItems },
  { label: '资质授权', href: '/authorization', icon: 'compass' },
  { label: '联系我们', href: '/contact', icon: 'headset' },
]
export const allNavItems = navGroups.flatMap(group => [group, ...(group.items ?? [])])
export function activeGroup(path: string) {
  return navGroups.find(group => group.href === path || group.items?.some(item => item.href === path))
    ?? navGroups.find(group => group.href !== '/' && path.startsWith(group.href + '/'))
    ?? (path.startsWith('/standards/') || path === '/standards' ? navGroups[3] : undefined)
}
export function pageTitle(path: string) {
  return allNavItems.find(item => item.href === path)?.label ?? (activeGroup(path)?.label || '页面未找到')
}
