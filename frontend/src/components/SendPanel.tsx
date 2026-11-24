import { useState, useRef } from 'react'
import type { TransferUpdate } from '../transfer.ts'
import { isRunning } from '../transfer.ts'
import { formatBytes } from '../utils/formatters.ts'
import { FileList } from './FileList.tsx'
import type { SelectedFileItem } from './FileList.tsx'
import { TransferProgress } from './TransferProgress.tsx'
import logoMark from '../assets/croccoc-mark.svg'

function sourceRootName(path: string): string {
  const trimmed = path.replace(/[/\\]+$/, '')
  return trimmed.split(/[/\\]/).pop() || trimmed
}

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
  const [isDragOver, setIsDragOver] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')
  const [copyStatus, setCopyStatus] = useState('')

  const fileInputRef = useRef<HTMLInputElement>(null)
  const folderInputRef = useRef<HTMLInputElement>(null)

  const state = update?.state ?? 'IDLE'
  const activeCode = update?.connectionCode || connectionCode || ''
  const hasCode = Boolean(activeCode)
  const activeRunning = isRunning(state)

  const addItems = (incoming: SelectedFileItem[]) => {
    const preview = appendDistinctRoots(items, incoming)
    setErrorMsg(
      preview.length < items.length + incoming.length
        ? 'Đã bỏ qua mục có đường dẫn hoặc tên gốc trùng với mục đã chọn.'
        : '',
    )
    setItems((prev) => appendDistinctRoots(prev, incoming))
  }

  const handleFilesAdded = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return
    const newItems: SelectedFileItem[] = Array.from(e.target.files).map((file) => ({
      id: `${file.name}-${file.size}-${Date.now()}-${Math.random()}`,
      name: file.name,
      path: (file as File & { path?: string }).path || file.name,
      size: file.size,
      isDirectory: false,
    }))
    addItems(newItems)
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
    addItems([newFolderItem])
    e.target.value = ''
  }

  const handleNativeSelect = async (kind: 'sendFiles' | 'sendDirectory') => {
    if (!onSelectPaths) return
    try {
      const paths = await onSelectPaths(kind)
      if (paths.length === 0) return
      addItems(
        paths.filter(Boolean).map((path, index) => ({
          id: `${path}-${Date.now()}-${index}`,
          path,
          name: sourceRootName(path),
          isDirectory: kind === 'sendDirectory',
        })),
      )
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể chọn đường dẫn.')
    }
  }

  const handleSampleFiles = () => {
    addItems([
      {
        id: 'sample-zip',
        name: 'CrocCoc-demo.zip',
        path: 'CrocCoc-demo.zip',
        size: 2654208,
        isDirectory: false,
      },
    ])
  }

  const handleRemoveItem = (id: string) => {
    setItems((prev) => prev.filter((item) => item.id !== id))
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
      addItems(dropped)
    }
  }

  const handleStart = async () => {
    if (items.length === 0) {
      setErrorMsg('Vui lòng chọn ít nhất một tệp hoặc thư mục để gửi.')
      return
    }
    setErrorMsg('')
    setCopyStatus('')
    const paths = items.map((i) => i.path)
    try {
      await onStartSend(paths)
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể bắt đầu phiên gửi.')
    }
  }

  const handleCopy = async () => {
    if (!activeCode) return
    if (!isMock && onCopySendCode) {
      try {
        await onCopySendCode()
        setCopyStatus('Đã sao chép mã an toàn.')
      } catch {
        setCopyStatus('Không thể sao chép an toàn. Vui lòng đọc mã trực tiếp trên màn hình.')
      }
      return
    }
    try {
      await navigator.clipboard.writeText(activeCode)
      setCopyStatus('Đã sao chép mã.')
    } catch {
      setCopyStatus('Mã đã sẵn sàng. Hãy chọn và sao chép thủ công.')
    }
  }

  const handleResetAll = () => {
    onReset()
    setItems([])
    setErrorMsg('')
    setCopyStatus('')
  }

  const totalSize = items.reduce((sum, item) => sum + (item.size ?? 0), 0)
  const selectionSummary = items.length === 0
    ? 'Chưa chọn tệp'
    : `${items.length} tệp / ${formatBytes(totalSize)}`

  let sendStateText = 'Chưa tạo'
  if (hasCode) sendStateText = 'Sẵn sàng'
  if (state === 'TRANSFERRING') sendStateText = 'Đang truyền'
  if (state === 'COMPLETED') sendStateText = 'Hoàn tất'
  if (state === 'FAILED') sendStateText = 'Lỗi'
  if (state === 'CANCELLED') sendStateText = 'Đã hủy'

  const noteText = copyStatus || (
    hasCode
      ? 'Dùng mã này để kết nối ở màn hình Nhận tệp.'
      : 'Mã sẽ xuất hiện sau khi chọn tệp và bấm Tạo mã.'
  )

  return (
    <section className="screen" id="gui" aria-labelledby="send-title">
      <header className="intro">
        <h1 id="send-title" tabIndex={-1}>
          Gửi tệp
        </h1>
        <p>Chọn tệp, tạo mã và chia sẻ với người nhận.</p>
      </header>

      {/* Hidden inputs */}
      <input
        ref={fileInputRef}
        id="picker"
        type="file"
        multiple
        hidden
        onChange={handleFilesAdded}
      />
      <input
        ref={folderInputRef}
        type="file"
        // @ts-expect-error webkitdirectory is standard for folder picker
        webkitdirectory=""
        hidden
        onChange={handleFolderAdded}
      />

      <div className="send-grid">
        {/* Left: Queue Panel */}
        <section className="panel queue" aria-labelledby="files-title" data-over={isDragOver ? '' : undefined}>
          <div className="panel-head">
            <h2 id="files-title">Tệp gửi</h2>
            <button
              type="button"
              className="sample"
              id="sample"
              onClick={handleSampleFiles}
              disabled={busy}
            >
              Dùng tệp mẫu
            </button>
          </div>

          {items.length === 0 ? (
            <div
              className="drop"
              id="drop"
              data-over={isDragOver ? '' : undefined}
              onDragOver={(e) => {
                e.preventDefault()
                setIsDragOver(true)
              }}
              onDragLeave={() => setIsDragOver(false)}
              onDrop={handleDrop}
            >
              <div className="drop-icon">
                <svg className="icon" aria-hidden="true">
                  <use href="#i-upload" />
                </svg>
              </div>
              <p>
                <strong>Kéo và thả tệp vào đây</strong>
              </p>
              <div className="drop-actions">
                <button
                  type="button"
                  className="button primary"
                  id="pick"
                  onClick={() =>
                    onSelectPaths ? void handleNativeSelect('sendFiles') : fileInputRef.current?.click()
                  }
                  disabled={busy}
                >
                  Chọn tệp
                </button>
                {onSelectPaths && (
                  <button
                    type="button"
                    className="button secondary"
                    onClick={() => void handleNativeSelect('sendDirectory')}
                    disabled={busy}
                  >
                    Chọn thư mục
                  </button>
                )}
              </div>
            </div>
          ) : (
            <div id="queue-items">
              <FileList items={items} onRemove={handleRemoveItem} disabled={busy} />
              <div className="queue-actions">
                <button
                  type="button"
                  className="button secondary"
                  id="pick-more"
                  onClick={() =>
                    onSelectPaths ? void handleNativeSelect('sendFiles') : fileInputRef.current?.click()
                  }
                  disabled={busy}
                >
                  Thêm tệp
                </button>
                {onSelectPaths && (
                  <button
                    type="button"
                    className="button secondary"
                    onClick={() => void handleNativeSelect('sendDirectory')}
                    disabled={busy}
                  >
                    Thêm thư mục
                  </button>
                )}
              </div>
            </div>
          )}

          {errorMsg && (
            <div className="error-alert" role="alert" style={{ marginTop: '16px' }}>
              <span>{errorMsg}</span>
            </div>
          )}

          <div className="panel-foot">
            <span id="selection" role="status">
              {selectionSummary}
            </span>
            {!activeRunning ? (
              <button
                type="button"
                className="button primary"
                id="start"
                disabled={items.length === 0 || busy}
                onClick={() => void handleStart()}
              >
                Tạo mã{' '}
                <svg className="icon" aria-hidden="true">
                  <use href="#i-arrow" />
                </svg>
              </button>
            ) : (
              <button type="button" className="button danger" onClick={onCancel}>
                Hủy phiên
              </button>
            )}
          </div>
        </section>

        {/* Right: Handoff Panel */}
        <section className="panel handoff" aria-labelledby="handoff-title">
          <div className="panel-head">
            <h2 id="handoff-title">Mã kết nối</h2>
            <span className="state" id="send-state" data-ready={hasCode ? '' : undefined}>
              {sendStateText}
            </span>
          </div>

          <div className="handoff-diagram" aria-hidden="true">
            <div className="endpoint origin">
              <img src={logoMark} width="43" height="34" alt="" />
            </div>
            <div className="handoff-line">
              <svg className="icon" aria-hidden="true">
                <use href="#i-arrow" />
              </svg>
            </div>
            <div className="endpoint destination">
              <svg className="icon" aria-hidden="true">
                <use href="#i-receive" />
              </svg>
            </div>
          </div>

          <div className="code-slot">
            <output
              id="send-code"
              aria-label="Mã kết nối"
              role="status"
              data-ready={hasCode ? '' : undefined}
            >
              {activeCode || '— — —'}
            </output>
            <button
              type="button"
              className="icon-button"
              id="copy"
              aria-label="Sao chép mã"
              title="Sao chép mã"
              disabled={!hasCode}
              onClick={() => void handleCopy()}
            >
              <svg className="icon" aria-hidden="true">
                <use href="#i-copy" />
              </svg>
            </button>
          </div>

          <p className="panel-note" id="copy-status" role="status">
            {noteText}
          </p>

          {/* Transfer status and metrics during active/finished transfer */}
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
        </section>
      </div>
    </section>
  )
}
