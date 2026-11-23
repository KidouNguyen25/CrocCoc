import { useEffect, useState } from 'react'
import type { TransferUpdate } from '../transfer.ts'
import { SAMPLE_CODE } from '../mockTransfer.ts'
import { TransferProgress } from './TransferProgress.tsx'
import { ReceiveOfferDialog } from './ReceiveOfferDialog.tsx'

interface ReceivePanelProps {
  update?: TransferUpdate
  busy: boolean
  isMock: boolean
  onStartReceive: (code: string, outputDir: string) => Promise<void>
  onSelectReceiveDirectory?: () => Promise<string[]>
  onDecideReceive?: (accept: boolean) => Promise<void>
  onCancel: () => void
  onReset: () => void
}

export function ReceivePanel({
  update,
  busy,
  isMock,
  onStartReceive,
  onSelectReceiveDirectory,
  onDecideReceive,
  onCancel,
  onReset,
}: ReceivePanelProps) {
  const [code, setCode] = useState('')
  const [outputDir, setOutputDir] = useState('Downloads')
  const [errorMsg, setErrorMsg] = useState('')
  const [folderCopied, setFolderCopied] = useState(false)

  useEffect(() => {
    // Desktop paths are not valid mock defaults and vice versa. Reset the
    // directory when changing engine modes, never reuse a native path in mock.
    setOutputDir((dir) => isMock ? 'Downloads' : (dir === 'Downloads' ? '' : dir))
  }, [isMock])

  const state = update?.state ?? 'IDLE'
  const isAwaiting = state === 'AWAITING_ACCEPTANCE' && !!update?.receiveOffer

  const handlePasteCode = async () => {
    try {
      const text = await navigator.clipboard.readText()
      if (text) {
        setCode(text.trim())
        setErrorMsg('')
      }
    } catch {
      setErrorMsg('Không thể truy cập bộ nhớ tạm. Vui lòng dán thủ công.')
    }
  }

  const handleStart = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    const trimmedCode = code.trim()
    const trimmedDir = outputDir.trim()

    if (!trimmedCode) {
      setErrorMsg('Vui lòng nhập mã kết nối.')
      return
    }
    if (!trimmedDir) {
      setErrorMsg('Vui lòng chọn thư mục lưu tệp.')
      return
    }

    setErrorMsg('')
    try {
      await onStartReceive(trimmedCode, trimmedDir)
      setCode('') // do not keep the secret in the input after submission
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể bắt đầu phiên nhận.')
    }
  }

  const handleSelectFolder = async () => {
    if (!onSelectReceiveDirectory) return
    try {
      const paths = await onSelectReceiveDirectory()
      if (paths.length > 0) {
        setOutputDir(paths[0])
        setErrorMsg('')
      }
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể chọn thư mục nhận.')
    }
  }

  const handleCopyFolderPath = async () => {
    try {
      await navigator.clipboard.writeText(outputDir)
      setFolderCopied(true)
      setTimeout(() => setFolderCopied(false), 2000)
    } catch {
      // Fallback
    }
  }

  const handleResetAll = () => {
    onReset()
    setCode('')
    setErrorMsg('')
  }

  return (
    <div className="panel-content receive-panel" aria-label="Bảng nhận tệp">
      {/* When IDLE: Show code input and output directory selector */}
      {state === 'IDLE' && (
        <form className="receive-form" onSubmit={handleStart}>
          <div className="form-group">
            <label htmlFor="connection-code-input" className="form-label">
              MÃ KẾT NỐI
            </label>
            <div className="input-with-button">
              <input
                id="connection-code-input"
                type="text"
                className="text-input code-input"
                placeholder="Nhập mã từ người gửi (VD: 1234-alpha-beta)"
                value={code}
                onChange={(e) => {
                  setCode(e.target.value)
                  setErrorMsg('')
                }}
                disabled={busy}
                autoFocus
              />
              <button
                type="button"
                className="btn-secondary"
                onClick={handlePasteCode}
                disabled={busy}
                title="Dán mã từ clipboard"
              >
                📋 Dán
              </button>
            </div>
            {isMock && (
              <div className="mock-quick-actions">
                <span>Gợi ý thử nghiệm: </span>
                <button
                  type="button"
                  className="btn-link"
                  onClick={() => {
                    setCode(SAMPLE_CODE)
                    setErrorMsg('')
                  }}
                >
                  Dùng mã mô phỏng mẫu
                </button>
              </div>
            )}
          </div>

          <div className="form-group">
            <label htmlFor="output-dir-input" className="form-label">
              THƯ MỤC LƯU TỆP NHẬN
            </label>
            <div className="input-with-button">
              <input
                id="output-dir-input"
                type="text"
                className="text-input"
                placeholder={onSelectReceiveDirectory ? 'Chọn thư mục lưu trên Windows' : 'Đường dẫn thư mục lưu (VD: Downloads)'}
                value={outputDir}
                onChange={(e) => setOutputDir(e.target.value)}
                disabled={busy}
              />
              {onSelectReceiveDirectory && (
                <button type="button" className="btn-secondary" onClick={() => void handleSelectFolder()} disabled={busy}>
                  📁 Chọn thư mục
                </button>
              )}
            </div>
            <p className="field-hint">
              Các tệp nhận về được tải vào vùng tạm riêng, kiểm tra toàn vẹn và chỉ ghi mới (không ghi đè) tại thư mục này.
            </p>
          </div>

          {errorMsg && (
            <div className="error-alert" role="alert">
              <span className="error-icon">⚠</span>
              <span>{errorMsg}</span>
            </div>
          )}

          <div className="action-row">
            <button
              type="submit"
              className="btn-primary btn-large btn-receive"
              disabled={busy || !code.trim() || !outputDir.trim()}
            >
              📥 Bắt đầu nhận tệp
            </button>
          </div>
        </form>
      )}

      {/* When waiting for peer approval of incoming offer */}
      {isAwaiting && update?.receiveOffer && (
        <ReceiveOfferDialog
          offer={update.receiveOffer}
          onDecide={(accept) => {
            if (onDecideReceive) {
              void onDecideReceive(accept)
            } else if (!accept) {
              onCancel()
            }
          }}
          disabled={busy && state !== 'AWAITING_ACCEPTANCE'}
        />
      )}

      {/* When running or finished (and not currently showing offer dialog): Show TransferProgress */}
      {state !== 'IDLE' && !isAwaiting && (
        <div className="receive-progress-wrapper">
          <TransferProgress
            update={update}
            direction="receive"
            isDesktop={!isMock}
            onCancel={onCancel}
            onReset={handleResetAll}
          />

          {/* Post-completion extra info: Destination directory and copy path */}
          {state === 'COMPLETED' && (
            <div className="destination-card">
              <div className="dest-icon">📂</div>
              <div className="dest-info">
                <span className="dest-label">Thư mục đã lưu:</span>
                <code className="dest-path">{outputDir}</code>
              </div>
              <button
                type="button"
                className="btn-secondary btn-copy-dest"
                onClick={handleCopyFolderPath}
                title="Sao chép đường dẫn thư mục"
              >
                {folderCopied ? '✓ Đã chép' : 'Sao chép đường dẫn'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
