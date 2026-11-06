import type { LinkStatus } from '../hooks/useFleet.ts'
import './Header.css'

const LABEL: Record<LinkStatus, { text: string; tone: string }> = {
  connecting: { text: 'Connecting…', tone: 'badge' },
  offline: { text: 'Server unreachable · retrying', tone: 'badge badge-danger' },
  waiting: { text: 'Waiting for first heartbeat', tone: 'badge badge-warn' },
  online: { text: 'Online', tone: 'badge badge-ok' },
}

export function Header({ status, vehicleCount, selectedCount }: { status: LinkStatus; vehicleCount: number; selectedCount: number }) {
  const { text, tone } = LABEL[status]
  return (
    <header className="header">
      <div className="brand">
        <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden>
          <path d="M12 2 L20 20 L12 16 L4 20 Z" fill="var(--accent)" />
        </svg>
        <h1>Drone Control</h1>
      </div>
      <div className="header-stats">
        <span className="stat">
          <b>{vehicleCount}</b> vehicles
        </span>
        <span className="stat">
          <b>{selectedCount}</b> selected
        </span>
        <span className={tone}>
          <span className={`dot ${status === 'online' ? '' : 'dot-pulse'}`} />
          {text}
        </span>
      </div>
    </header>
  )
}
