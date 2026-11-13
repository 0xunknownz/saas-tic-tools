export type TestItem = { id: string; code: string; name: string; category: string; method: string; description: string; revision: number }
export type UpdateTestItemInput = Pick<TestItem, 'name' | 'category' | 'method' | 'description'> & { expectedRevision: number }
export type TestItemPage = { items: TestItem[]; total: number }
export class TestItemApiError extends Error { constructor(public status: number, public code: string, message: string) { super(message); this.name = 'TestItemApiError' } }
async function request<T>(path: string, input?: UpdateTestItemInput): Promise<T> {
  const response = await fetch(path, { method: input ? 'PUT' : 'GET', credentials: 'same-origin', headers: { Accept: 'application/json', ...(input ? { 'Content-Type': 'application/json' } : {}) }, body: input ? JSON.stringify(input) : undefined })
  if (!response.ok) { const payload = await response.json().catch(() => ({})) as { code?: string; message?: string }; throw new TestItemApiError(response.status, payload.code ?? 'ERROR', payload.message ?? response.statusText) }
  return response.json() as Promise<T>
}
export function listTestItems(q = ''): Promise<TestItemPage> { const query = new URLSearchParams({ limit: '100', ...(q ? { q } : {}) }); return request<TestItemPage>('/api/product-admin/v1/test-items?' + query.toString()) }
export function getTestItem(id: string): Promise<TestItem> { return request<TestItem>('/api/product-admin/v1/test-items/' + encodeURIComponent(id)) }
export function updateTestItem(id: string, input: UpdateTestItemInput): Promise<TestItem> { return request<TestItem>('/api/product-admin/v1/test-items/' + encodeURIComponent(id), input) }
