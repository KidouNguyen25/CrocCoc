import { useEffect, useRef, useState } from 'react'
import type { TransferUpdate } from '../transfer.ts'
import { isRunning } from '../transfer.ts'
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
  const codeInputRef = useRef<HTMLInputElement>(null)
  const [code, setCode] = useState('')
  const [outputDir, setOutputDir] = useState('Downloads')
  const [errorMsg, setErrorMsg] = useState('')
  const [feedback, setFeedback] = useState('')
  const [folderCopied, setFolderCopied] = useState(false)

  useEffect(() => {
    setOutputDir((dir) => (isMock ? 'Downloads' : dir === 'Downloads' ? '' : dir))
  }, [isMock])

  const state = update?.state ?? 'IDLE'
  const activeRunning = isRunning(state)
  const isAwaiting = state === 'AWAITING_ACCEPTANCE' && !!update?.receiveOffer

  const handlePasteCode = async () => {
    try {
      const text = await navigator.clipboard.readText()
      if (text) {
        setCode(text.trim())
        setErrorMsg('')
        setFeedback('Đã dán mã từ clipboard.')
      }
    } catch {
      setErrorMsg('Không thể đọc bộ nhớ tạm. Hãy dán mã thủ công.')
    }
  }

  const handleFillSample = () => {
    setCode(SAMPLE_CODE)
    setErrorMsg('')
    setFeedback('Đã điền mã mẫu. Chọn Kiểm tra mã để tiếp tục.')
    codeInputRef.current?.focus()
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

  const handleStart = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    const trimmedCode = code.trim()
    const trimmedDir = outputDir.trim()

    if (!trimmedCode) {
      setErrorMsg(isMock ? 'Nhập mã hoặc chọn Dùng mã mẫu.' : 'Nhập mã kết nối.')
      codeInputRef.current?.focus()
      return
    }
    if (!trimmedDir) {
      setErrorMsg('Vui lòng chọn thư mục lưu tệp nhận.')
      return
    }

    setErrorMsg('')
    try {
      await onStartReceive(trimmedCode, trimmedDir)
      setCode('') // Do not retain the secret passphrase in the input field
      if (isMock) setFeedback('Đã nhận diện mã mẫu. Không có tệp nào được tải xuống.')
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Không thể bắt đầu phiên nhận.')
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
    setFeedback('')
  }

  return (
    <section className="screen" id="nhan" aria-labelledby="receive-title">
      <header className="intro">
        <h1 id="receive-title" tabIndex={-1}>
          Nhận tệp
        </h1>
        <p>Nhập mã do người gửi cung cấp để bắt đầu.</p>
      </header>

      <form
        className={isMock ? 'panel narrow form-panel preview-form' : 'panel narrow form-panel'}
        id="receive-form"
        onSubmit={handleStart}
        noValidate
      >
        <div className="panel-head">
          <h2>Kết nối với người gửi</h2>
          {isMock && (
            <button
              type="button"
              className="sample"
              id="fill-code"
              onClick={handleFillSample}
              disabled={busy}
            >
              Dùng mã mẫu
            </button>
          )}
        </div>

        <label htmlFor="receive-code">Mã Croc</label>
        <div className="code-input-group">
          <input
            id="receive-code"
            ref={codeInputRef}
            type="text"
            placeholder="Nhập mã kết nối"
            value={code}
            onChange={(e) => {
              setCode(e.target.value)
              setErrorMsg('')
              setFeedback('')
            }}
            autoComplete="off"
            spellCheck="false"
            maxLength={96}
            disabled={busy}
            aria-invalid={Boolean(errorMsg)}
            aria-describedby="receive-feedback"
          />
          {!isMock && (
            <button
              type="button"
              className="button secondary"
              onClick={() => void handlePasteCode()}
              disabled={busy}
              title="Dán từ clipboard"
            >
              Dán
            </button>
          )}
        </div>

        {!isMock && (
          <>
            <label htmlFor="receive-dir" style={{ marginTop: '16px' }}>
              Thư mục lưu tệp
            </label>
            <div className="dir-input-group">
              <input
                id="receive-dir"
                type="text"
                placeholder={onSelectReceiveDirectory ? 'Chọn thư mục lưu trên máy tính' : 'Thư mục nhận (VD: Downloads)'}
                value={outputDir}
                onChange={(e) => setOutputDir(e.target.value)}
                disabled={busy}
              />
              {onSelectReceiveDirectory && (
                <button
                  type="button"
                  className="button secondary"
                  onClick={() => void handleSelectFolder()}
                  disabled={busy}
                >
                  Chọn
                </button>
              )}
            </div>
          </>
        )}

        {errorMsg ? (
          <p className="panel-note" id="receive-feedback" role="status" data-error="">
            {errorMsg}
          </p>
        ) : (
          <p className="panel-note" id="receive-feedback" role="status">
            {feedback || (isMock ? 'Mã nhập chỉ được kiểm tra trong bản xem trước.' : 'Nhập mã kết nối an toàn.')}
          </p>
        )}

        <div className="panel-foot">
          <span />
          {!activeRunning ? (
            <button
              className="button primary"
              type="submit"
              disabled={busy || (!isMock && (!code.trim() || !outputDir.trim()))}
            >
              {isMock ? 'Kiểm tra mã' : 'Nhận tệp'}
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
      </form>

      {/* Progress and results */}
      {state !== 'IDLE' && !isAwaiting && (
        <div className="panel narrow" style={{ marginTop: '16px' }}>
          <TransferProgress
            update={update}
            direction="receive"
            isDesktop={!isMock}
            onCancel={onCancel}
            onReset={handleResetAll}
          />

          {state === 'COMPLETED' && !isMock && (
            <div style={{ marginTop: '16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
              <span style={{ fontSize: '13px', color: 'var(--muted)' }}>
                Đã lưu tại: <code style={{ color: 'var(--leaf)' }}>{outputDir}</code>
              </span>
              <button
                type="button"
                className="button secondary"
                onClick={() => void handleCopyFolderPath()}
              >
                {folderCopied ? 'Đã sao chép' : 'Sao chép đường dẫn'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Offer dialog when awaiting user decision */}
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
    </section>
  )
}
