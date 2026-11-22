import type { CrocInfo } from '../transfer.ts'

interface HeaderProps {
  hostTitle: string
  crocInfo?: CrocInfo
  isDesktop: boolean
}

export function Header({ hostTitle, crocInfo, isDesktop }: HeaderProps) {
  const isAvailable = crocInfo?.available ?? false
  const crocLabel = isAvailable
    ? `Croc ${crocInfo?.version ?? 'Sẵn sàng'}`
    : crocInfo?.message ?? (isDesktop ? 'Chưa kết nối Croc' : 'Chế độ mô phỏng')

  return (
    <header className="masthead">
      <div className="wordmark" title="CrocCoc - Ứng dụng truyền tệp ngang hàng với Croc">
        <span className="mark" aria-hidden="true">↔</span>
        <div className="title-group">
          <span className="app-name">CrocCoc</span>
          <span className="subtitle">Chuyển tệp P2P an toàn</span>
        </div>
      </div>
      <div className="masthead-tags">
        <span
          className={`status-pill ${isAvailable ? 'pill-success' : 'pill-warning'}`}
          title={crocInfo?.message || (isAvailable ? 'Croc CLI đã sẵn sàng' : 'Croc CLI chưa khả dụng')}
        >
          <span className="pill-dot" />
          {crocLabel}
        </span>
        <span className="tag" title={hostTitle}>
          {isDesktop ? 'Desktop App' : 'Bản xem trước'}
        </span>
      </div>
    </header>
  )
}
