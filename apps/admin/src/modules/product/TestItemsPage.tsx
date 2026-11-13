import { Link } from '@tanstack/react-router'
import { createPlatformClient } from '@0xunknownz/foundation-client'
import { useEffect, useState } from 'react'
import { Button, DataTable, PageHeader, PageState, useAuth, type Column } from '@0xunknownz/foundation-platform-ui'
import { listTestItems, TestItemApiError, type TestItem } from './api.ts'

export function TestItemsPage() {
  const [query, setQuery] = useState('')
  const [items, setItems] = useState<TestItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [retry, setRetry] = useState(0)
  const [tenantCount, setTenantCount] = useState<number | null>(null)
  const { can } = useAuth()

  useEffect(() => {
    if (!can('platform.tenants.read')) return
    let active = true
    createPlatformClient().GET('/api/admin/v1/tenants', { params: { query: { limit: 1 } } })
      .then(({ data }) => { if (active && typeof data?.total === 'number') setTenantCount(data.total) })
      .catch(() => { /* Dictionary access is independent of the tenant count. */ })
    return () => { active = false }
  }, [can])

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError(null)
    listTestItems(query.trim())
      .then((page) => {
        if (!controller.signal.aborted) {
          setItems(page.items)
          setTotal(page.total)
        }
      })
      .catch((reason: unknown) => { if (!controller.signal.aborted) setError(reason) })
      .finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [query, retry])

  const columns: Column<TestItem>[] = [
    { key: 'code', header: '项目编号', render: (item) => <code className="font-ftu-mono">{item.code}</code> },
    { key: 'name', header: '检测项目', render: (item) => <Link to={('/admin/product/test-items/' + item.id) as never} className="text-ftu-link">{item.name}</Link> },
    { key: 'category', header: '类别', render: (item) => item.category },
    { key: 'method', header: '检测方法', render: (item) => item.method },
    { key: 'detail', header: '详情', render: (item) => <Link to={('/admin/product/test-items/' + item.id) as never} className="text-ftu-link">查看</Link> },
  ]

  if (error instanceof TestItemApiError && error.status === 403) {
    return <PageState kind="403" title="当前账号不能查看检测项目字典" permission="platform.product.testitems.read" />
  }
  return (
    <>
      <PageHeader title="检测项目字典" description="平台统一维护、各检测机构共用的检测项目与方法参考。当前为样例数据；有编辑权限的管理员可修改记录。" />
      {error ? (
        <PageState kind="load-error" title="检测项目加载失败" description={error instanceof Error ? error.message : '请稍后重试。'} action={<Button variant="primary" onClick={() => setRetry((value) => value + 1)}>重新加载</Button>} />
      ) : (
        <DataTable
          caption="检测项目字典"
          columns={columns}
          rows={items}
          rowKey={(item) => item.id}
          loading={loading}
          data-testid="test-items-table"
          toolbar={<label className="grid gap-[var(--ftu-space-1)] text-[12px] text-ftu-muted">搜索项目<input className="h-[var(--ftu-control-height)] px-[var(--ftu-space-3)] border border-ftu-border-strong rounded-ftu bg-ftu-background text-ftu-foreground" type="search" value={query} onChange={(event) => setQuery(event.target.value)} aria-label="搜索检测项目" placeholder="编号、名称或类别" /></label>}
          empty={<PageState kind="empty" title="没有检测项目" description="没有找到匹配的项目。" />}
        />
      )}
      <p className="text-[13px] text-ftu-muted">共 {total} 项 · 只读{tenantCount !== null && <> · 当前实例共有 {tenantCount} 个租户</>}</p>
    </>
  )
}
