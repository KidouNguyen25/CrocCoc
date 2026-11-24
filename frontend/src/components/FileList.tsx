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
  onClear?: () => void
  disabled?: boolean
}

export function FileList({ items, onRemove, disabled = false }: FileListProps) {
  if (items.length === 0) return null

  return (
    <ul className="file-list" id="file-list" aria-label="Danh sách tệp">
      {items.map((item) => (
        <li key={item.id}>
          <span className="file-label">
            <svg className="icon" aria-hidden="true">
              <use href="#i-file" />
            </svg>
            <span title={item.path || item.name}>{item.name}</span>
          </span>
          <span className="file-meta">
            {item.size !== undefined && item.size > 0 ? formatBytes(item.size) : '0 B'}
          </span>
          <button
            type="button"
            className="icon-button"
            aria-label={`Bỏ tệp ${item.name}`}
            title={`Bỏ tệp ${item.name}`}
            disabled={disabled}
            onClick={() => onRemove(item.id)}
          >
            <svg className="icon" aria-hidden="true">
              <use href="#i-close" />
            </svg>
          </button>
        </li>
      ))}
    </ul>
  )
}
