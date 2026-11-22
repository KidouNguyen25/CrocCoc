import { formatBytes } from '../utils/formatters.ts'

export interface SelectedFileItem {
  id: string
  name: string
  path: string
  size?: number
  isDirectory?: boolean
}

interface FileListProps {
  items: SelectedFileItem[]
  onRemove: (id: string) => void
  onClear: () => void
  disabled?: boolean
}

export function FileList({ items, onRemove, onClear, disabled = false }: FileListProps) {
  if (items.length === 0) return null

  const totalSize = items.reduce((sum, item) => sum + (item.size ?? 0), 0)
  const hasKnownSize = items.some((item) => item.size !== undefined && item.size > 0)

  return (
    <div className="file-list-container" aria-label="Danh sách tệp và thư mục đã chọn">
      <div className="file-list-header">
        <span className="file-count">
          <strong>{items.length}</strong> mục đã chọn {hasKnownSize && `(${formatBytes(totalSize)})`}
        </span>
        <button
          type="button"
          className="btn-text btn-clear"
          onClick={onClear}
          disabled={disabled}
          title="Xóa tất cả mục đã chọn"
        >
          Xóa tất cả
        </button>
      </div>

      <ul className="file-items-list" role="list">
        {items.map((item) => (
          <li key={item.id} className="file-item">
            <div className="file-icon" aria-hidden="true">
              {item.isDirectory ? '📁' : '📄'}
            </div>
            <div className="file-info">
              <span className="file-name" title={item.path || item.name}>
                {item.name}
              </span>
              {item.size !== undefined && item.size > 0 && (
                <span className="file-size">{formatBytes(item.size)}</span>
              )}
            </div>
            <button
              type="button"
              className="btn-icon btn-remove"
              onClick={() => onRemove(item.id)}
              disabled={disabled}
              title={`Bỏ mục ${item.name}`}
              aria-label={`Bỏ mục ${item.name}`}
            >
              ✕
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
