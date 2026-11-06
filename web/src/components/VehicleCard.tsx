import type { VehicleState } from '../api.ts'
import { VehicleDetails } from './VehicleDetails.tsx'

interface Props {
  vehicle: VehicleState
  selected: boolean
  stale: boolean
  busy: boolean
  isHome: boolean
  expanded: boolean
  onToggleSelect: () => void
  onToggleExpand: () => void
}

export function VehicleCard({ vehicle: v, selected, stale, busy, isHome, expanded, onToggleSelect, onToggleExpand }: Props) {
  const alt = v.position?.relative_altitude
  return (
    <li className={`vcard ${selected ? 'is-selected' : ''} ${stale ? 'is-stale' : ''}`}>
      <div className="vcard-row">
        <label className="vcard-check">
          <input type="checkbox" checked={selected} onChange={onToggleSelect} />
          <span className="vcard-name">Drone {v.system_id}</span>
        </label>
        <button className="btn btn-ghost" onClick={onToggleExpand} aria-expanded={expanded} title="Details">
          {expanded ? '▴' : '▾'}
        </button>
      </div>
      <div className="vcard-badges">
        {stale ? (
          <span className="badge badge-danger">LINK LOST</span>
        ) : (
          <span className={v.is_armed ? 'badge badge-danger' : 'badge'}>{v.is_armed ? 'ARMED' : 'disarmed'}</span>
        )}
        <span className="badge badge-accent">{v.flight_mode ?? '–'}</span>
        {busy && (
          <span className="badge badge-warn">
            <span className="dot dot-pulse" />
            busy
          </span>
        )}
        {isHome && <span className="badge badge-home">HOME</span>}
        <span className="vcard-alt mono">{alt == null ? '–' : `${alt.toFixed(1)} m`}</span>
      </div>
      {expanded && <VehicleDetails state={v} />}
    </li>
  )
}
