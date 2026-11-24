import { useEffect, useRef } from 'react'
import type { ReceiveOffer } from '../transfer.ts'
import { formatBytes } from '../utils/formatters.ts'

interface ReceiveOfferDialogProps {
  offer: ReceiveOffer
  onDecide: (accept: boolean) => void
  disabled?: boolean
}

export function ReceiveOfferDialog({ offer, onDecide, disabled = false }: ReceiveOfferDialogProps) {
  const { fileCount, totalBytes, items, expiresAt } = offer
  const dialogRef = useRef<HTMLDivElement>(null)
  const previouslyFocused = useRef<HTMLElement | null>(null)
  const disabledRef = useRef(disabled)
  const onDecideRef = useRef(onDecide)

  useEffect(() => {
    disabledRef.current = disabled
    onDecideRef.current = onDecide
  }, [disabled, onDecide])

  useEffect(() => {
    previouslyFocused.current = document.activeElement as HTMLElement | null

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !disabledRef.current) {
        e.preventDefault()
        onDecideRef.current(false)
        return
      }
      if (e.key === 'Tab') {
        const focusableElements = dialogRef.current?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])',
        )
        const firstElement = focusableElements?.[0] as HTMLElement | undefined
        const lastElement = focusableElements?.[focusableElements.length - 1] as HTMLElement | undefined

        if (e.shiftKey && document.activeElement === firstElement) {
          e.preventDefault()
          lastElement?.focus()
        } else if (!e.shiftKey && (document.activeElement as HTMLElement | null) === lastElement) {
          e.preventDefault()
          firstElement?.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    const focusTarget =
      dialogRef.current?.querySelector<HTMLElement>('button[autoFocus]') ||
      dialogRef.current?.querySelector<HTMLElement>('.btn-accept')
    const focusTimer = setTimeout(() => focusTarget?.focus(), 0)

    return () => {
      clearTimeout(focusTimer)
      document.removeEventListener('keydown', handleKeyDown)
      previouslyFocused.current?.focus()
    }
  }, [])

  const expiryLabel = expiresAt
    ? new Date(expiresAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : ''

  return (
    <div className="modal-backdrop" role="presentation">
      <div
        ref={dialogRef}
        className="offer-dialog-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="offer-title"
      >
        <div className="offer-header">
          <span className="offer-badge">YÊU CẦU TRUYỀN TỆP</span>
          <h3 id="offer-title">Bên gửi muốn gửi cho bạn:</h3>
          <p className="offer-summary">
            <strong>{fileCount}</strong> tệp / thư mục • Tổng dung lượng: <strong>{formatBytes(totalBytes)}</strong>
            {expiryLabel && <> • Hết hạn lúc: <strong>{expiryLabel}</strong></>}
          </p>
        </div>

        <div className="offer-items-box">
          <ul className="offer-items-list" role="list">
            {items.map((item, index) => (
              <li key={index} className="offer-item">
                <span className="file-label">
                  <svg className="icon" aria-hidden="true">
                    <use href="#i-file" />
                  </svg>
                  <span className="offer-item-path" title={item.path}>
                    {item.path}
                  </span>
                </span>
                {item.sizeBytes !== undefined && item.sizeBytes !== '' && (
                  <span className="file-meta">{formatBytes(item.sizeBytes)}</span>
                )}
              </li>
            ))}
          </ul>
        </div>

        <div className="offer-actions">
          <button
            type="button"
            className="button secondary"
            onClick={() => onDecide(false)}
            disabled={disabled}
          >
            Từ chối
          </button>
          <button
            type="button"
            className="button primary btn-accept"
            autoFocus
            onClick={() => onDecide(true)}
            disabled={disabled}
          >
            Chấp nhận tải về
          </button>
        </div>
      </div>
    </div>
  )
}
