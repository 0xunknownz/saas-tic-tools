import { ErrorAlert, useMessages, type ApiFailure } from '@0xunknownz/foundation-tenant-ui'

export function Failure({ failure }: { failure: ApiFailure | null }) {
  const t = useMessages()
  if (!failure) return null
  return <div className="tic-alert"><ErrorAlert failure={failure}>{failure.messageKey ? t(failure.messageKey) : failure.message}</ErrorAlert></div>
}

export function utcDate(value?: string | null) {
  if (!value) return '未提供'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '未提供' : date.toISOString().slice(0, 16).replace('T', ' ') + ' UTC'
}
