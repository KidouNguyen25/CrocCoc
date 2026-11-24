import type { CrocInfo } from '../transfer.ts'
import logoMark from '../assets/croccoc-mark.svg'
import logoWordmark from '../assets/croccoc-wordmark.svg'

interface HeaderProps {
  hostTitle: string
  crocInfo?: CrocInfo
  isDesktop: boolean
  onBrandClick?: () => void
}

export function Header({ crocInfo, isDesktop, onBrandClick }: HeaderProps) {
  const isAvailable = crocInfo?.available ?? false
  const tagText = isDesktop
    ? (isAvailable ? (crocInfo?.relayLabel ? `Desktop · ${crocInfo.relayLabel}` : 'Ứng dụng desktop') : 'Ứng dụng desktop · Chưa phát hiện Croc')
    : 'Bản xem trước'

  return (
    <header className="topbar">
      <a
        href="#gui"
        className="brand"
        aria-label="CrocCoc — Gửi tệp"
        onClick={(e) => {
          if (onBrandClick) {
            e.preventDefault()
            onBrandClick()
          }
        }}
      >
        <img className="brand-mark" src={logoMark} width="35" height="28" alt="" />
        <img className="brand-word" src={logoWordmark} width="115" height="27" alt="CrocCoc" />
      </a>
      <span className="preview-tag">
        <svg className="icon" aria-hidden="true">
          <use href="#i-preview" />
        </svg>
        <span>{tagText}</span>
      </span>
    </header>
  )
}
