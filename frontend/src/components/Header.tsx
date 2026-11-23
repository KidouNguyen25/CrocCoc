import type { CrocInfo } from '../transfer.ts'
import logoMark from '../assets/croccoc-mark.svg'

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
  const relayLabel = crocInfo?.relayLabel

  return (
    <header className="masthead">
      <div className="wordmark" title="CrocCoc - Ứng dụng truyền tệp ngang hàng với Croc">
        <img
          className="mark"
          src={logoMark}
          alt=""
          aria-hidden="true"
          width="38"
          height="38"
        />
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
        {relayLabel && (
          <span
            className="tag"
            title="Relay chỉ thấy metadata mạng (IP, thời điểm, lưu lượng), không đọc được nội dung tệp đã mã hóa PAKE"
          >
            {relayLabel}
          </span>
        )}
        <span className="tag" title={hostTitle}>
          {isDesktop ? 'Desktop App' : 'Bản xem trước'}
        </span>
      </div>
    </header>
  )
}
