import { useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { BackLink, Button, InfoList, Mono, PageHeader, PageState, useAuth } from '@0xunknownz/foundation-platform-ui'
import { getTestItem, TestItemApiError, type TestItem } from './api.ts'
import { TestItemEditForm } from './TestItemEditForm.tsx'

export function TestItemDetailPage() {
  const { itemId = '' } = useParams({ strict: false }) as { itemId?: string }
  const [item, setItem] = useState<TestItem | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [loading, setLoading] = useState(true)
  const [retry, setRetry] = useState(0)
  const [editing, setEditing] = useState(false)
  const { can } = useAuth()

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError(null)
    getTestItem(itemId)
      .then((value) => { if (!controller.signal.aborted) setItem(value) })
      .catch((reason: unknown) => { if (!controller.signal.aborted) setError(reason) })
      .finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [itemId, retry])

  if (loading) return <PageState kind="loading" />
  if (error instanceof TestItemApiError && error.status === 403) {
    return <PageState kind="403" title="当前账号不能查看检测项目详情" permission="platform.product.testitems.read" />
  }
  if (error instanceof TestItemApiError && error.status === 404) {
    return <><BackLink to="/admin/product/test-items">返回检测项目字典</BackLink><PageState kind="404" title="检测项目不存在" description="项目可能已移除，或链接编号不正确。" /></>
  }
  if (error) {
    return <PageState kind="load-error" title="检测项目加载失败" description={error instanceof Error ? error.message : '请稍后重试。'} action={<Button variant="primary" onClick={() => setRetry((value) => value + 1)}>重新加载</Button>} />
  }
  if (!item) return <PageState kind="404" title="检测项目不存在" />

  return (
    <>
      <div className="mb-[var(--ftu-space-2)]"><BackLink to="/admin/product/test-items">返回检测项目字典</BackLink></div>
      <PageHeader title={item.name} description={<Mono>{item.code}</Mono>} actions={can('platform.product.testitems.manage') ? <Button variant="primary" onClick={() => setEditing((value) => !value)}>{editing ? '取消编辑' : '编辑项目'}</Button> : undefined} />
      <InfoList items={[
        { label: '项目编号', value: <Mono>{item.code}</Mono> },
        { label: '项目名称', value: item.name },
        { label: '类别', value: item.category },
        { label: '检测方法', value: item.method },
        { label: '说明', value: item.description || '暂无说明' },
        { label: '记录 ID', value: <Mono>{item.id}</Mono> },
      ]} />
      {editing && can('platform.product.testitems.manage') && <div className="mt-[var(--ftu-space-4)]"><TestItemEditForm key={item.id + item.revision} item={item} onUpdated={(updated) => { setItem(updated); setEditing(false) }} onReload={() => setRetry((value) => value + 1)} /></div>}
    </>
  )
}
