interface ModeSwitcherProps {
  mode: 'send' | 'receive'
  onModeChange: (mode: 'send' | 'receive') => void
  disabled: boolean
}

export function ModeSwitcher({ mode, onModeChange, disabled }: ModeSwitcherProps) {
  return (
    <div className="switcher" role="group" aria-label="Chọn chế độ gửi hoặc nhận">
      <button
        type="button"
        aria-pressed={mode === 'send'}
        disabled={disabled}
        onClick={() => onModeChange('send')}
        className={`switch-tab ${mode === 'send' ? 'active' : ''}`}
      >
        <span className="tab-icon">↑</span>
        <span className="tab-label">Gửi tệp (Send)</span>
      </button>
      <button
        type="button"
        aria-pressed={mode === 'receive'}
        disabled={disabled}
        onClick={() => onModeChange('receive')}
        className={`switch-tab ${mode === 'receive' ? 'active' : ''}`}
      >
        <span className="tab-icon">↓</span>
        <span className="tab-label">Nhận tệp (Receive)</span>
      </button>
    </div>
  )
}
