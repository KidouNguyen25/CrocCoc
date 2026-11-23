import { useState, useRef } from 'react'
import type { TransferUpdate } from '../transfer.ts'
import { FileList } from './FileList.tsx'
import type { SelectedFileItem } from './FileList.tsx'
import { TransferProgress } from './TransferProgress.tsx'

function sourceRootName(path: string): string {
  const trimmed = path.replace(/[/\\]+$/, '')
  return trimmed.split(/[/\\]/).pop() || trimmed
}

// The receiver publishes root names into one directory. Two distinct sources
// with the same basename would collide there even if their full paths differ.
function appendDistinctRoots(existing: SelectedFileItem[], incoming: SelectedFileItem[]): SelectedFileItem[] {
  const paths = new Set(existing.map((item) => item.path.replace(/[/\\]+$/, '').replace(/\//g, '\\').toLowerCase()))
  const roots = new Set(existing.map((item) => item.name.trim().toLowerCase()))
  const merged = [...existing]
  for (const item of incoming) {
    const path = item.path.replace(/[/\\]+$/, '').replace(/\//g, '\\').toLowerCase()
    const root = item.name.trim().toLowerCase()
    if (!path || !root || paths.has(path) || roots.has(root)) continue
    paths.add(path)
    roots.add(root)
    merged.push(item)
  }
  return merged
}

interface SendPanelProps {
  update?: TransferUpdate
  connectionCode?: string
  busy: boolean
  isMock: boolean
  onStartSend: (paths: string[]) => Promise<void>
  onSelectPaths?: (kind: 'sendFiles' | 'sendDirectory') => Promise<string[]>
  onCopySendCode?: () => Promise<void>
  onCancel: () => void
  onReset: () => void
}

export function SendPanel({
  update,
  connectionCode,
  busy,
  isMock,
  onStartSend,
  onSelectPaths,
  onCopySendCode,
  onCancel,
  onReset,
}: SendPanelProps) {
  const [items, setItems] = useState<SelectedFileItem[]>([])
  const [manualPath, setManualPath] = useState('')
  const [isDragOver, setIsDragOver] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const fileInputRef = useRef<HTMLInputElement>(null)
  const folderInputRef = useRef<HTMLInputElement>(null)

  const state = update?.state ?? 'IDLE'

  const handleFilesAdded = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return
    const newItems: SelectedFileItem[] = Array.from(e.target.files).map((file) => ({
      id: `${file.name}-${file.size}-${Date.now()}-${Math.random()}`,
      name: file.name,
      path: (file as File & { path?: string }).path || file.name,
      size: file.size,
      isDirectory: false,
    }))
    setItems((prev) => appendDistinctRoots(prev, newItems))
    setErrorMsg('')
    e.target.value = ''
  }

  const handleFolderAdded = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return
    const files = Array.from(e.target.files)
    const firstRel = files[0].webkitRelativePath || ''
    const folderName = firstRel.split('/')[0] || 'Thư mục'
    const totalSize = files.reduce((sum, f) => sum + f.size, 0)
    const folderPath = (files[0] as File & { path?: string }).path
      ? (files[0] as File & { path?: string }).path!.replace(/[/\\][^/\\]+$/, '')
      : folderName

    const newFolderItem: SelectedFileItem = {
      id: `${folderName}-${Date.now()}-${Math.random()}`,
      name: folderName,
      path: folderPath,
      size: totalSize,
      isDirectory: true,
    }
    setItems((prev) => appendDistinctRoots(prev, [newFolderItem]))
    setErrorMsg('')
    e.target.value = ''
  }

  const handleAddManualPath = () => {
    const trimmed = manualPath.trim()
    if (!trimmed) return
    const name = sourceRootName(trimmed)
    setItems((prev) => appendDistinctRoots(prev, [{
      id: `${trimmed}-${Date.now()}`,
      name,
      path: trimmed,
      size: undefined,
      isDirectory: false,
    }]))
    setManualPath('')
    setErrorMsg('')
  }

  const handleNativeSelect = async (kind: 'sendFiles' | 'sendDirectory') => {
    if (!onSelectPaths) return
    try {
      const paths = await onSelectPaths(kind)
      if (paths.length === 0) return // user cancelled picker
      setItems((existing) => appendDistinctRoots(existing, paths.filter(Boolean).map((path, index) => ({
        id: `${path}-${Date.now()}-${index}`,
        path,
        name: sourceRootName(path),
        isDirectory: kind === 'sendDirectory',
      }))))
      setErrorMsg('')
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể chọn đường dẫn.')
    }
  }

  const handleRemoveItem = (id: string) => {
    setItems((prev) => prev.filter((item) => item.id !== id))
  }

  const handleClearItems = () => {
    setItems([])
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOver(false)
    if (busy) return
    if (onSelectPaths) {
      setErrorMsg('Hãy dùng nút Chọn tệp/Chọn thư mục để lấy đường dẫn thật từ Windows.')
      return
    }
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const dropped: SelectedFileItem[] = Array.from(e.dataTransfer.files).map((f) => ({
        id: `${f.name}-${f.size}-${Date.now()}-${Math.random()}`,
        name: f.name,
        path: (f as File & { path?: string }).path || f.name,
        size: f.size,
        isDirectory: false,
      }))
      setItems((prev) => appendDistinctRoots(prev, dropped))
      setErrorMsg('')
    }
  }

  const handleStart = async () => {
    if (items.length === 0) {
      setErrorMsg('Vui lòng chọn ít nhất một tệp hoặc thư mục để gửi.')
      return
    }
    setErrorMsg('')
    const paths = items.map((i) => i.path)
    try {
      await onStartSend(paths)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể bắt đầu phiên gửi.')
    }
  }

  const handleResetAll = () => {
    onReset()
    setItems([])
    setErrorMsg('')
  }

  return (
    <div className="panel-content send-panel" aria-label="Bảng gửi tệp">
      {/* Hidden native inputs */}
      <input
        ref={fileInputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleFilesAdded}
      />
      <input
        ref={folderInputRef}
        type="file"
        // @ts-expect-error webkitdirectory attribute is standard in browsers for directory picker
        webkitdirectory=""
        style={{ display: 'none' }}
        onChange={handleFolderAdded}
      />

      {/* When IDLE: Show file selector and list */}
      {state === 'IDLE' && (
        <div className="selector-area">
          <div
            className={`dropzone ${isDragOver ? 'dropzone-active' : ''}`}
            onDragOver={(e) => {
              e.preventDefault()
              setIsDragOver(true)
            }}
            onDragLeave={() => setIsDragOver(false)}
            onDrop={handleDrop}
          >
            <div className="dropzone-icon" aria-hidden="true">
              📦
            </div>
            <h3 className="dropzone-title">{onSelectPaths ? 'Chọn tệp hoặc thư mục trên máy tính' : 'Kéo thả tệp hoặc thư mục vào đây'}</h3>
            <p className="dropzone-sub">Hỗ trợ gửi nhiều tệp hoặc nguyên thư mục cùng lúc</p>
            <div className="dropzone-buttons">
              <button
                type="button"
                className="btn-select"
                onClick={() => onSelectPaths ? void handleNativeSelect('sendFiles') : fileInputRef.current?.click()}
                disabled={busy}
              >
                📄 Chọn tệp
              </button>
              <button
                type="button"
                className="btn-select"
                onClick={() => onSelectPaths ? void handleNativeSelect('sendDirectory') : folderInputRef.current?.click()}
                disabled={busy}
              >
                📁 Chọn thư mục
              </button>
            </div>
          </div>

          {/* Manual path entry (useful for desktop or custom paths) */}
          <div className="manual-path-box">
            <input
              type="text"
              className="text-input"
              placeholder="Hoặc nhập đường dẫn tệp/thư mục (VD: C:\data\file.zip)"
              value={manualPath}
              onChange={(e) => setManualPath(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddManualPath()}
              disabled={busy}
            />
            <button
              type="button"
              className="btn-secondary"
              onClick={handleAddManualPath}
              disabled={busy || !manualPath.trim()}
            >
              Thêm
            </button>
          </div>

          {/* Mock mode convenience helper */}
          {isMock && items.length === 0 && (
            <div className="mock-quick-actions">
              <span>Gợi ý thử nghiệm: </span>
              <button
                type="button"
                className="btn-link"
                onClick={() =>
                  setItems([
                    {
                      id: 'sample-1',
                      name: 'tai-lieu-bao-cao.pdf',
                      path: 'tai-lieu-bao-cao.pdf',
                      size: 2450000,
                      isDirectory: false,
                    },
                    {
                      id: 'sample-2',
                      name: 'anh-chup-man-hinh.png',
                      path: 'anh-chup-man-hinh.png',
                      size: 1120000,
                      isDirectory: false,
                    },
                  ])
                }
              >
                Thêm tệp mẫu
              </button>
            </div>
          )}

          {/* Selected files list */}
          <FileList
            items={items}
            onRemove={handleRemoveItem}
            onClear={handleClearItems}
            disabled={busy}
          />

          {errorMsg && (
            <div className="error-alert" role="alert">
              <span className="error-icon">⚠</span>
              <span>{errorMsg}</span>
            </div>
          )}

          {/* Action button */}
          <div className="action-row">
            <button
              type="button"
              className="btn-primary btn-large btn-send"
              onClick={() => void handleStart()}
              disabled={busy || items.length === 0}
            >
              🚀 Bắt đầu gửi ({items.length} mục)
            </button>
          </div>
        </div>
      )}

      {/* When running or finished: Show TransferProgress */}
      {state !== 'IDLE' && (
        <TransferProgress
          update={update}
          connectionCode={connectionCode}
          direction="send"
          isDesktop={!isMock}
          onCopySendCode={onCopySendCode}
          onCancel={onCancel}
          onReset={handleResetAll}
        />
      )}
    </div>
  )
}
