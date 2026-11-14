import { ChangeEvent, FormEvent, useState } from 'react'
import { useSession } from '@0xunknownz/foundation-tenant-ui'
import { AppLink, EmptyState, PageHeading, Panel } from '../workbench/ui'
import { ArchiveError, LocalUploadRow, UploadBatch, archiveStatusLabels, formatBytes, retryUpload, uploadBatch } from './api'

const maxFiles = 50
const maxBytes = 50_000_000

export function ReportUploadPage() {
  const session = useSession()
  const tenantId = session.session?.tenant?.id ?? ''
  const [rows, setRows] = useState<LocalUploadRow[]>([])
  const [batch, setBatch] = useState<UploadBatch | null>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState<string | null>(null)

  function selectFiles(event: ChangeEvent<HTMLInputElement>) {
    const chosen = Array.from(event.target.files ?? [])
    setError(null)
    setNote(null)
    const accepted: LocalUploadRow[] = []
    const rejected: string[] = []
    for (const file of chosen) {
      if (!file.name.toLowerCase().endsWith('.pdf')) { rejected.push(file.name + '：不是 PDF'); continue }
      if (file.size > maxBytes) { rejected.push(file.name + '：超过 50,000,000 字节'); continue }
      accepted.push({ id: file.name + '-' + file.size + '-' + Math.random().toString(36).slice(2), file, progress: 0, state: 'ready' })
    }
    if (rows.length + accepted.length > maxFiles) { rejected.push('一次最多 ' + maxFiles + ' 份') }
    if (rejected.length) setError('已跳过：' + rejected.join('；'))
    setRows(previous => [...previous, ...accepted].slice(0, maxFiles))
    event.target.value = ''
  }

  function removeRow(id: string) {
    setRows(previous => previous.filter(row => row.id !== id))
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (rows.length === 0) { setError('请选择至少一份 PDF'); return }
    setUploading(true)
    setError(null)
    setNote(null)
    setRows(previous => previous.map(row => ({ ...row, state: 'uploading', progress: 0 })))
    try {
      const result = await uploadBatch(rows.map(row => row.file), (index, percent) => {
        setRows(previous => previous.map((row, i) => i === index ? { ...row, progress: percent } : row))
      })
      setBatch(result)
      setRows(previous => previous.map(row => {
        const item = result.items.find(entry => entry.fileName === row.file.name)
        return item ? { ...row, state: item.status === 'rejected' ? 'failed' : 'done', progress: 100, item, error: item.errorMessage } : row
      }))
      setNote('上传请求已完成；服务端接收 ' + result.accepted + ' 份、重复 ' + result.duplicate + ' 份、失败 ' + result.failed + ' 份。处理任务已持久化，可关闭页面。')
    } catch (err) {
      setError(err instanceof ArchiveError ? err.message : '上传失败，请重试')
      setRows(previous => previous.map(row => row.state === 'uploading' ? { ...row, state: 'failed', error: '上传中断，可重试' } : row))
    } finally {
      setUploading(false)
    }
  }

  async function retry(row: LocalUploadRow) {
    if (!batch) return
    setRows(previous => previous.map(item => item.id === row.id ? { ...item, state: 'uploading', progress: 0, error: undefined } : item))
    try {
      const result = await retryUpload(batch.id, row.file, percent => {
        setRows(previous => previous.map(item => item.id === row.id ? { ...item, progress: percent } : item))
      })
      setBatch(result)
      const item = result.items.find(entry => entry.fileName === row.file.name && entry.status !== 'rejected') ?? result.items[result.items.length - 1]
      setRows(previous => previous.map(entry => entry.id === row.id ? { ...entry, state: item?.status === 'rejected' ? 'failed' : 'done', progress: 100, item, error: item?.errorMessage } : entry))
      setNote('该文件已重新提交，处理任务已持久化。')
    } catch (err) {
      setRows(previous => previous.map(item => item.id === row.id ? { ...item, state: 'failed', error: err instanceof ArchiveError ? err.message : '重试失败' } : item))
    }
  }

  const ready = rows.filter(row => row.state === 'ready').length
  const done = rows.filter(row => row.state === 'done').length
  const failed = rows.filter(row => row.state === 'failed').length

  return <>
    <PageHeading title="批量上传检测报告" description="一次最多 50 份 PDF；单份最大 50,000,000 字节、100 个物理页。服务端逐份校验并持久化处理任务。"
      action={<AppLink className="tic-button secondary" to="/reports">返回报告列表</AppLink>} />
    {error ? <div className="tic-error" role="alert">{error}</div> : null}
    {note ? <div className="tic-success" role="status">{note}</div> : null}
    <Panel title="选择文件">
      <form className="tic-form" onSubmit={submit}>
        <label>PDF 文件<input type="file" accept="application/pdf,.pdf" multiple onChange={selectFiles} disabled={uploading} /></label>
        <p className="tic-muted">已选择 {rows.length} / {maxFiles} 份。上传与 OCR 处理分离：关闭页面不会丢失已接收文件或任务。</p>
        <div className="tic-filter-actions"><button className="tic-button" disabled={uploading || ready === 0}>{uploading ? '上传中…' : '开始上传'}</button><button className="tic-button secondary" type="button" onClick={() => { setRows([]); setBatch(null); setError(null); setNote(null) }} disabled={uploading}>清空列表</button></div>
      </form>
      <dl className="tic-upload-summary">
        <div><dt>待上传</dt><dd>{ready}</dd></div>
        <div><dt>已完成</dt><dd>{done}</dd></div>
        <div><dt>失败</dt><dd>{failed}</dd></div>
        <div><dt>服务端批次</dt><dd>{batch ? batch.id.slice(0, 8) : '—'}</dd></div>
      </dl>
      {rows.length === 0 ? <EmptyState title="尚未选择文件"><p>请选择本租户的检测报告 PDF。</p></EmptyState> : <div className="tic-table-wrap" role="region" aria-label="上传文件列表，可横向滚动" tabIndex={0}>
        <table><thead><tr><th scope="col">文件</th><th scope="col">大小</th><th scope="col">进度</th><th scope="col">结果</th><th scope="col">操作</th></tr></thead>
          <tbody>{rows.map(row => <tr key={row.id}>
            <td>{row.file.name}</td>
            <td>{formatBytes(row.file.size)}</td>
            <td><progress className="tic-progress" max={100} value={row.progress} aria-label={row.file.name + ' 上传进度'} /> {row.progress}%</td>
            <td>{row.item ? <><span className={'tic-tag ' + (row.item.status === 'rejected' ? 'warn' : 'success')}>{row.item.status === 'accepted' ? '已接收' : row.item.status === 'duplicate' ? '重复文件' : '失败'}</span>{row.item.reportId ? <AppLink to={'/reports/' + row.item.reportId}>查看档案</AppLink> : null}{row.item.errorMessage ? <small className="tic-muted"> {row.item.errorMessage}</small> : null}</> : row.error ? <span className="tic-tag warn">{row.error}</span> : row.state === 'uploading' ? '上传中' : '待上传'}</td>
            <td>{row.state === 'failed' && batch ? <button className="tic-button secondary" onClick={() => void retry(row)}>重试</button> : row.state === 'ready' ? <button className="tic-button secondary" onClick={() => removeRow(row.id)}>移除</button> : null}</td>
          </tr>)}</tbody></table>
      </div>}
    </Panel>
    {batch ? <Panel title="服务端批次汇总">
      <p>批次 {batch.id} · 共 {batch.total} 份：接收 {batch.accepted}、重复 {batch.duplicate}、失败 {batch.failed}。已接收文件各自关联档案并进入持久处理队列。</p>
      <p className="tic-muted">进度可在“报告列表”的待处理范围中继续查看；处理阶段包括待 OCR、OCR 中、规则解析、待处理与已归档。</p>
    </Panel> : null}
    {rows.some(row => row.item?.archiveStatus && ['pending_ocr', 'ocr_running', 'parsing'].includes(row.item.archiveStatus)) ? <p className="tic-muted" role="status">部分档案仍在服务端处理中，可稍后在列表中查看。</p> : null}
  </>
}
