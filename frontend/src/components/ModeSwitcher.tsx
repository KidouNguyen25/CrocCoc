export type AppRoute = 'gui' | 'nhan' | 'phien' | 'engine'

export interface SidebarRailProps {
  activeRoute: AppRoute
  onRouteChange: (route: AppRoute) => void
  blockedRoute?: AppRoute
  isDesktop?: boolean
  isMock?: boolean
  crocAvailable?: boolean
  relayLabel?: string
}

export function SidebarRail({
  activeRoute,
  onRouteChange,
  blockedRoute,
  isDesktop = false,
  isMock = true,
  crocAvailable = false,
  relayLabel,
}: SidebarRailProps) {
  const routes: { id: AppRoute; label: string; iconId: string }[] = [
    { id: 'gui', label: 'Gửi tệp', iconId: 'i-send' },
    { id: 'nhan', label: 'Nhận tệp', iconId: 'i-receive' },
    { id: 'phien', label: 'Phiên truyền', iconId: 'i-history' },
    { id: 'engine', label: 'Croc engine', iconId: 'i-engine' },
  ]

  const footTitle = isDesktop
    ? (isMock ? 'Chế độ mô phỏng' : (crocAvailable ? 'Croc engine' : 'Chưa kết nối'))
    : 'Chế độ mô phỏng'

  const footSubtitle = isDesktop
    ? (isMock ? 'Đang bật mock thủ công' : (relayLabel ? relayLabel : 'Wails desktop IPC'))
    : 'Không kết nối backend'

  return (
    <aside className="rail">
      <nav aria-label="Điều hướng">
        {routes.map((r) => (
          <button
            key={r.id}
            type="button"
            className={activeRoute === r.id ? 'active' : ''}
            aria-current={activeRoute === r.id ? 'page' : undefined}
            onClick={() => onRouteChange(r.id)}
            disabled={r.id === blockedRoute}
          >
            <svg className="icon" aria-hidden="true">
              <use href={`#${r.iconId}`} />
            </svg>
            <span>{r.label}</span>
          </button>
        ))}
      </nav>
      <div className="rail-foot">
        <span>{footTitle}</span>
        <small>{footSubtitle}</small>
      </div>
    </aside>
  )
}

// Backward-compatibility export for existing ModeSwitcher interface
export function ModeSwitcher({
  mode,
  onModeChange,
  disabled,
}: {
  mode: 'send' | 'receive'
  onModeChange: (mode: 'send' | 'receive') => void
  disabled?: boolean
}) {
  return (
    <SidebarRail
      activeRoute={mode === 'send' ? 'gui' : 'nhan'}
      onRouteChange={(r) => {
        if (r === 'gui') onModeChange('send')
        else if (r === 'nhan') onModeChange('receive')
      }}
      blockedRoute={disabled ? (mode === 'send' ? 'nhan' : 'gui') : undefined}
    />
  )
}
