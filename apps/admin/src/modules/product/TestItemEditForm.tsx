import { useState, type FormEvent } from 'react'
import { Button, LicenseErrorNotice, Notice, licenseErrorCopy, useToast } from '@0xunknownz/foundation-platform-ui'
import { TestItemApiError, updateTestItem, type TestItem } from './api.ts'

const inputClass = 'w-full h-[var(--ftu-control-height)] px-[var(--ftu-space-3)] border border-ftu-border-strong rounded-ftu bg-ftu-background text-ftu-foreground'

export function TestItemEditForm({ item, onUpdated, onReload }: {
  item: TestItem
  onUpdated: (item: TestItem) => void
  onReload: () => void
}) {
  const { notify } = useToast()
  const [name, setName] = useState(item.name)
  const [category, setCategory] = useState(item.category)
  const [method, setMethod] = useState(item.method)
  const [description, setDescription] = useState(item.description)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<unknown>(null)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const updated = await updateTestItem(item.id, { name: name.trim(), category: category.trim(), method: method.trim(), description: description.trim(), expectedRevision: item.revision })
      onUpdated(updated)
      notify({ kind: 'success', title: '检测项目已更新', description: updated.name })
    } catch (reason) {
      setError(reason)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className="grid gap-[var(--ftu-space-4)] p-[var(--ftu-space-4)] bg-ftu-background border border-ftu-border rounded-ftu-lg" aria-label="编辑检测项目">
      <h2 className="text-[16px] font-semibold m-0">编辑检测项目</h2>
      <p className="text-[13px] text-ftu-muted m-0">项目编号 {item.code} 不可修改。当前版本 {item.revision}。</p>
      <label className="grid gap-[var(--ftu-space-1)]">项目名称<input className={inputClass} required maxLength={120} value={name} onChange={(event) => setName(event.target.value)} /></label>
      <label className="grid gap-[var(--ftu-space-1)]">类别<input className={inputClass} required maxLength={80} value={category} onChange={(event) => setCategory(event.target.value)} /></label>
      <label className="grid gap-[var(--ftu-space-1)]">检测方法<input className={inputClass} required maxLength={120} value={method} onChange={(event) => setMethod(event.target.value)} /></label>
      <label className="grid gap-[var(--ftu-space-1)]">说明<textarea className="w-full min-h-[96px] p-[var(--ftu-space-3)] border border-ftu-border-strong rounded-ftu bg-ftu-background text-ftu-foreground" maxLength={1000} value={description} onChange={(event) => setDescription(event.target.value)} /></label>
      {error ? (error instanceof TestItemApiError && error.status === 409 ? (
        <Notice kind="warning" title="记录已由其他人修改">请重新加载，查看最新版本后再编辑。<Button onClick={onReload} size="sm">重新加载详情</Button></Notice>
      ) : error instanceof TestItemApiError ? (
        licenseErrorCopy(error) ? <LicenseErrorNotice error={error} /> : <Notice kind="danger" title="保存失败">{error.message}</Notice>
      ) : (
        <Notice kind="warning" title="保存结果待确认">网络中断，无法确定操作是否完成。请重新加载详情核实。<Button onClick={onReload} size="sm">重新加载详情</Button></Notice>
      )) : null}
      <div><Button type="submit" variant="primary" disabled={saving}>{saving ? '正在保存…' : '保存修改'}</Button></div>
    </form>
  )
}
