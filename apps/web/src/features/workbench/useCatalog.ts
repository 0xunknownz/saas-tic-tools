import { useState } from 'react'

/** Filtering and paging share one result set; changes always return to page one. */
export function useCatalog<T>(items: readonly T[], searchable: (item: T) => string, categoryOf?: (item: T) => string) {
  const [keyword, setKeywordValue] = useState('')
  const [category, setCategoryValue] = useState('')
  const [requestedPage, setPage] = useState(1)
  const query = keyword.trim().toLocaleLowerCase()
  const filtered = items.filter(item => searchable(item).toLocaleLowerCase().includes(query) && (!category || categoryOf?.(item) === category))
  const pageSize = 5
  const page = Math.min(requestedPage, Math.max(1, Math.ceil(filtered.length / pageSize)))
  function setKeyword(value: string) { setKeywordValue(value); setPage(1) }
  function setCategory(value: string) { setCategoryValue(value); setPage(1) }
  function reset() { setKeyword(''); setCategory('') }
  return { keyword, category, page, pageSize, filtered, visible: filtered.slice((page - 1) * pageSize, page * pageSize), setKeyword, setCategory, setPage, reset }
}
