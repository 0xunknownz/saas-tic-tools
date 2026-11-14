import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

export const reportArchiveBase = '/api/tenant/v1/report-archive'

export class ArchiveError extends Error {
  code: string
  status: number
  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ArchiveError'
    this.code = code
    this.status = status
  }
}

export interface FieldSource { raw: string; page: number; line: number }
export interface Detail {
  id?: string
  section: string
  itemName: string
  requirement: string
  unit: string
  method: string
  resultRaw: string
  judgement: string
  position: number
  sourcePage: number
  sourceText: string
}
export interface FileInfo {
  id: string
  fileName: string
  contentType: string
  size: number
  pages: number
  sha256: string
  createdAt: string
}
export interface OCRPage { page: number; text: string }
export interface OCRRun {
  id: string
  status: string
  provider: string
  model: string
  text?: string
  pages?: OCRPage[]
  errorCode?: string
  errorMessage?: string
  startedAt: string
  finishedAt?: string
}
export interface Issue { code: string; stage: string; field?: string; message: string; page?: number; line?: number }
export interface ParseRun {
  id: string
  status: string
  ruleId?: string
  ruleVersion?: number
  issues?: Issue[]
  createdAt: string
}
export interface ReportEvent {
  id: string
  action: string
  actor: string
  reason?: string
  createdAt: string
}
export interface ReportSummary {
  id: string
  reportNumber: string
  reportName: string
  archiveStatus: string
  reportVersion: number
  isCurrent: boolean
  revisionOf?: string
  createdAt: string
}
export interface Report {
  id: string
  file?: FileInfo
  reportNumber: string
  reportName: string
  productName: string
  institution: string
  clientUnit: string
  inspectedUnit: string
  manufacturer: string
  model: string
  category: string
  basis: string
  reportDate?: string
  conclusion: string
  detectionResult: string
  archiveStatus: string
  reviewStage?: string
  reviewReason?: string
  archiveReason?: string
  reportVersion: number
  revision: number
  revisionOf?: string
  duplicateOf?: string
  isCurrent: boolean
  ruleId?: string
  ruleVersion?: number
  fieldSources?: Record<string, FieldSource>
  rawFields?: Record<string, unknown>
  createdBy: string
  createdAt: string
  updatedAt: string
  archivedAt?: string
  voidedAt?: string
  voidReason?: string
  details?: Detail[]
  ocr?: OCRRun
  latestParse?: ParseRun
  events?: ReportEvent[]
  versions?: ReportSummary[]
}
export interface ListResult {
  items: Report[]
  total: number
  page: number
  pageSize: number
}
export interface UploadItem {
  id: string
  fileName: string
  status: string
  reportId?: string
  fileId?: string
  sha256?: string
  size?: number
  pages?: number
  errorCode?: string
  errorMessage?: string
  archiveStatus?: string
  createdAt: string
}
export interface UploadBatch {
  id: string
  createdBy: string
  total: number
  accepted: number
  failed: number
  duplicate: number
  createdAt: string
  items: UploadItem[]
}
export interface RuleConfig {
  templateKey: string
  name: string
  institution: string
  layoutMarkers: string[]
  fields: Record<string, FieldRule>
  details: DetailRule
  validation: ValidationRule
}
export interface FieldRule { labels: string[]; pattern: string; required: boolean; format?: string; exclude?: string[] }
export interface DetailRule {
  startMarkers: string[]
  endMarkers: string[]
  skipPatterns: string[]
  sectionPattern?: string
  rowPattern: string
  continuationPattern?: string
  minRows: number
  expectedRows?: number
  requireFields: string[]
}
export interface ValidationRule { requiredFields: string[]; requireDetails: boolean; allowEmptyValues?: string[] }
export interface Rule {
  id: string
  tenantId?: string
  templateKey: string
  name: string
  version: number
  status: string
  builtin: boolean
  config: RuleConfig
  createdBy: string
  createdAt: string
  activatedAt?: string
  retiredAt?: string
}
export interface TrialResult {
  ruleId: string
  ruleVersion: number
  fields: Record<string, string>
  details: Detail[]
  issues: Issue[]
}
export interface ValidationPreview { issues: Issue[] }

export interface ArchiveListFilter {
  query?: string
  institution?: string
  dateFrom?: string
  dateTo?: string
  status?: string
  item?: string
  judgement?: string
  scope?: string
  page?: number
  pageSize?: number
}

async function request<T>(path: string, init: RequestInit = {}, signal?: AbortSignal): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  headers.set('Accept', 'application/json')
  let response: Response
  try {
    response = await fetch(path, { ...init, headers, credentials: 'same-origin', signal })
  } catch (error) {
    if ((error as Error).name === 'AbortError') throw error
    throw new ArchiveError(0, 'NETWORK_ERROR', '网络请求失败，请重试')
  }
  const text = await response.text()
  if (!response.ok) {
    let code = 'HTTP_' + response.status
    let message = response.statusText || '请求失败'
    try {
      const body = JSON.parse(text) as { code?: string; message?: string }
      if (body.code) code = body.code
      if (body.message) message = body.message
    } catch { /* keep HTTP-derived values */ }
    throw new ArchiveError(response.status, code, message)
  }
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export function queryOf(filter: ArchiveListFilter): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filter)) {
    if (value === undefined || value === null || value === '') continue
    params.set(key, String(value))
  }
  const s = params.toString()
  return s ? '?' + s : ''
}

export async function listReports(filter: ArchiveListFilter, signal?: AbortSignal): Promise<ListResult> {
  return request<ListResult>(reportArchiveBase + '/reports' + queryOf(filter), {}, signal)
}
export async function getReport(id: string, signal?: AbortSignal): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id), {}, signal)
}
export async function getOCR(id: string, signal?: AbortSignal): Promise<OCRRun> {
  return request<OCRRun>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/ocr', {}, signal)
}
export async function getCandidate(id: string, signal?: AbortSignal): Promise<{ id: string; fields: Record<string, string>; details: Detail[]; issues: Issue[] }> {
  return request(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/candidate', {}, signal)
}
export async function getValidation(id: string, signal?: AbortSignal): Promise<ValidationPreview> {
  return request(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/validation', {}, signal)
}
export async function updateReport(id: string, input: Record<string, unknown>): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id), { method: 'PATCH', body: JSON.stringify(input) })
}
export async function archiveReport(id: string, reason: string, confirmedIssues: string[]): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/archive', { method: 'POST', body: JSON.stringify({ reason, confirmedIssues }) })
}
export async function voidReport(id: string, reason: string): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/void', { method: 'POST', body: JSON.stringify({ reason }) })
}
export async function restoreReport(id: string, reason: string): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/restore', { method: 'POST', body: JSON.stringify({ reason }) })
}
export async function confirmRevision(id: string, reason: string): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/revision', { method: 'POST', body: JSON.stringify({ reason }) })
}
export async function reparseReport(id: string, reason: string): Promise<void> {
  await request(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/reparse', { method: 'POST', body: JSON.stringify({ reason }) })
}
export async function retryOCR(id: string, reason: string): Promise<void> {
  await request(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/retry-ocr', { method: 'POST', body: JSON.stringify({ reason }) })
}
export async function applyCandidate(id: string, expectedRevision: number): Promise<Report> {
  return request<Report>(reportArchiveBase + '/reports/' + encodeURIComponent(id) + '/candidate/apply', { method: 'POST', body: JSON.stringify({ expectedRevision }) })
}
export async function getBatch(id: string, signal?: AbortSignal): Promise<UploadBatch> {
  return request<UploadBatch>(reportArchiveBase + '/uploads/' + encodeURIComponent(id), {}, signal)
}
export async function listRules(signal?: AbortSignal): Promise<Rule[]> {
  const body = await request<{ items: Rule[] }>(reportArchiveBase + '/rules', {}, signal)
  return body.items
}
export async function createRule(input: Record<string, unknown>): Promise<Rule> {
  return request<Rule>(reportArchiveBase + '/rules', { method: 'POST', body: JSON.stringify(input) })
}
export async function activateRule(id: string): Promise<Rule> {
  return request<Rule>(reportArchiveBase + '/rules/' + encodeURIComponent(id) + '/activate', { method: 'POST', body: '{}' })
}
export async function trialRule(id: string, input: { text?: string; reportId?: string }): Promise<TrialResult> {
  return request<TrialResult>(reportArchiveBase + '/rules/' + encodeURIComponent(id) + '/trial', { method: 'POST', body: JSON.stringify(input) })
}

export interface LocalUploadRow {
  id: string
  file: File
  progress: number
  state: 'ready' | 'uploading' | 'done' | 'failed'
  item?: UploadItem
  error?: string
}

function uploadWithProgress(path: string, file: File, onProgress: (percent: number) => void): Promise<UploadBatch> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', path, true)
    xhr.withCredentials = true
    xhr.upload.onprogress = event => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onload = () => {
      let body: UploadBatch | { code?: string; message?: string } | null = null
      try { body = JSON.parse(xhr.responseText) } catch { body = null }
      if (xhr.status >= 200 && xhr.status < 300 && body) {
        onProgress(100)
        resolve(body as UploadBatch)
        return
      }
      const err = body as { code?: string; message?: string } | null
      reject(new ArchiveError(xhr.status, err?.code ?? 'UPLOAD_FAILED', err?.message ?? '上传失败'))
    }
    xhr.onerror = () => reject(new ArchiveError(0, 'NETWORK_ERROR', '网络请求失败，请重试'))
    const form = new FormData()
    form.append('files', file, file.name)
    xhr.send(form)
  })
}

export async function uploadBatch(files: File[], onProgress: (index: number, percent: number) => void): Promise<UploadBatch> {
  // The first request creates the durable batch; the rest append to it, so one page close still
  // leaves every received file and its processing task in the database.
  if (files.length === 0) throw new ArchiveError(400, 'FILE_REQUIRED', '请选择至少一份 PDF')
  let batch = await uploadWithProgress(reportArchiveBase + '/uploads', files[0], percent => onProgress(0, percent))
  for (let i = 1; i < files.length; i++) {
    batch = await uploadWithProgress(reportArchiveBase + '/uploads/' + encodeURIComponent(batch.id) + '/items', files[i], percent => onProgress(i, percent))
  }
  return batch
}

export async function retryUpload(batchId: string, file: File, onProgress: (percent: number) => void): Promise<UploadBatch> {
  return uploadWithProgress(reportArchiveBase + '/uploads/' + encodeURIComponent(batchId) + '/items', file, onProgress)
}

export interface ArchiveFeedState<T> {
  data: T | null
  error: ArchiveError | null
  loading: boolean
  reload: () => void
}

export function useArchiveFeed<T>(loader: (signal: AbortSignal) => Promise<T>, deps: unknown[], key: string): ArchiveFeedState<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<ArchiveError | null>(null)
  const [loading, setLoading] = useState(true)
  const [nonce, setNonce] = useState(0)
  const generation = useRef(0)
  useEffect(() => {
    const controller = new AbortController()
    const current = ++generation.current
    setLoading(true)
    loader(controller.signal)
      .then(value => {
        if (generation.current !== current) return
        setData(value)
        setError(null)
      })
      .catch(err => {
        if (generation.current !== current || (err as Error).name === 'AbortError') return
        setError(err instanceof ArchiveError ? err : new ArchiveError(0, 'UNKNOWN', (err as Error).message))
      })
      .finally(() => {
        if (generation.current === current) setLoading(false)
      })
    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, key, nonce])
  const reload = useCallback(() => setNonce(value => value + 1), [])
  return { data, error, loading, reload }
}

export function formatBytes(size?: number): string {
  if (!size && size !== 0) return '—'
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB'
  return (size / 1024 / 1024).toFixed(2) + ' MB'
}

export const archiveStatusLabels: Record<string, string> = {
  pending_ocr: '待 OCR',
  ocr_running: 'OCR 中',
  parsing: '规则解析',
  needs_review: '待处理',
  archived: '已归档',
  voided: '已作废',
}
export const detectionLabels: Record<string, string> = { unknown: '未识别', qualified: '合格', unqualified: '不合格' }

export function statusTone(status: string): string {
  if (status === 'archived') return 'tic-tag success'
  if (status === 'voided') return 'tic-tag muted'
  if (status === 'needs_review') return 'tic-tag warn'
  return 'tic-tag'
}

export function useDebounced<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

export function useMemoList<T>(items: T[], key: (item: T) => string): Map<string, T> {
  return useMemo(() => new Map(items.map(item => [key(item), item])), [items, key])
}
