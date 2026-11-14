import { useEffect, useId, useRef, type ReactNode } from 'react'

// Native modal dialogs make the rest of the application inert, including at narrow widths.
export function Dialog({ title, children, onClose, className = '', busy = false }: {
  title: string
  children: ReactNode
  onClose: () => void
  className?: string
  busy?: boolean
}) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const dialog = ref.current!
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    dialog.showModal()
    const first = dialog.querySelector<HTMLElement>('input, textarea, select, [autofocus]')
      ?? dialog.querySelector<HTMLElement>('button, [href], [tabindex="0"]')
    first?.focus()
    return () => {
      dialog.close()
      const details = previous?.closest('details')
      if (details && !details.open) details.querySelector('summary')?.focus()
      else previous?.focus()
    }
  }, [])
  return <dialog ref={ref} className={'tic-dialog ' + className} aria-labelledby={titleId} aria-busy={busy}
    onCancel={event => { event.preventDefault(); if (!busy) close.current() }}
    onKeyDown={event => {
      if (event.key !== 'Tab') return
      const elements = [...event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]')]
      const first = elements[0], last = elements[elements.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
    }}>
    <header className="tic-dialog-heading"><h2 id={titleId}>{title}</h2><button type="button" className="tic-close" aria-label="关闭" disabled={busy} onClick={onClose}>×</button></header>
    {children}
  </dialog>
}
