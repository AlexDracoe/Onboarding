import type { VehicleState } from '../api.ts'
import type { LatLon } from '../geo.ts'
import type { DispatchBatch } from '../hooks/useDispatch.ts'
import type { CommandRecord } from '../api.ts'
import { DispatchStatus } from './DispatchStatus.tsx'
import { GotoForm } from './GotoForm.tsx'
import { ModeForm } from './ModeForm.tsx'
import { TakeoffForm } from './TakeoffForm.tsx'
import './CommandPanel.css'

export type Send = (name: string, params?: Record<string, unknown>) => void

interface Props {
  selected: ReadonlySet<number>
  vehicles: VehicleState[]
  stale: ReadonlySet<number>
  busy: ReadonlySet<number>
  /** The selected drones that can take a command right now (online, not stale, not busy). */
  targets: number[]
  sending: boolean
  onSend: Send
  goto: LatLon | null
  onGotoChange: (p: LatLon | null) => void
  last: DispatchBatch | null
  records: CommandRecord[] | undefined
}

export function CommandPanel({ selected, vehicles, stale, busy, targets, sending, onSend, goto, onGotoChange, last, records }: Props) {
  const disabled = targets.length === 0 || sending
  const present = new Set(vehicles.map((v) => v.system_id))
  const selectedIds = [...selected].filter((id) => present.has(id)).sort((a, b) => a - b)

  return (
    <section className="card commands">
      <div className="card-title">
        <span>Commands</span>
        <span className="badge badge-accent">
          {targets.length} target{targets.length === 1 ? '' : 's'}
        </span>
      </div>

      <div className="commands-body">
        <div className="targets">
          {selectedIds.length === 0 ? (
            <p className="hint">Select one or more drones in the fleet list or on the map.</p>
          ) : (
            selectedIds.map((id) => {
              const why = stale.has(id) ? 'link lost' : busy.has(id) ? 'busy' : null
              return (
                <span key={id} className={why ? 'badge' : 'badge badge-accent'} title={why ?? 'ready'}>
                  Drone {id}
                  {why && ` · ${why}`}
                </span>
              )
            })
          )}
        </div>

        <div className="cmd-group">
          <h3>Quick actions</h3>
          <div className="btn-grid">
            <button className="btn btn-ok" disabled={disabled} onClick={() => onSend('arm')}>
              Arm
            </button>
            <button className="btn btn-danger" disabled={disabled} onClick={() => onSend('disarm')}>
              Disarm
            </button>
            <button className="btn btn-warn" disabled={disabled} onClick={() => onSend('set_mode', { mode: 'RTL' })}>
              Return home
            </button>
            <button className="btn btn-warn" disabled={disabled} onClick={() => onSend('set_mode', { mode: 'LAND' })}>
              Land
            </button>
          </div>
        </div>

        <ModeForm disabled={disabled} onSend={onSend} />
        <TakeoffForm disabled={disabled} onSend={onSend} />
        <GotoForm disabled={disabled} onSend={onSend} target={goto} onTargetChange={onGotoChange} />

        <DispatchStatus last={last} records={records} sending={sending} />
      </div>
    </section>
  )
}
