import type {VehicleState} from './api.ts'
import {VehiclePanel} from './VehiclePanel.tsx'

export function VehicleList({
  vehicles,
  selectedId,
}: {
  vehicles: VehicleState[] | undefined
  selectedId: number | null
}) {
  return (
    <section>
      <h2>Vehicles</h2>
      {(vehicles ?? []).map((v) => (
        <details key={v.system_id} className={v.system_id === selectedId ? 'selected' : ''}>
          <summary>
            Drone {v.system_id} · {v.flight_mode ?? '–'} · {v.is_armed ? 'ARMED' : 'disarmed'}
          </summary>
          <VehiclePanel state={v} />
        </details>
      ))}
      {vehicles?.length === 0 && <p className="hint">No vehicles yet</p>}
    </section>
  )
}