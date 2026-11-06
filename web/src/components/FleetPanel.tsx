import { useState } from 'react'
import type { VehicleState } from '../api.ts'
import { VehicleCard } from './VehicleCard.tsx'
import './FleetPanel.css'

interface Props {
  vehicles: VehicleState[]
  selected: ReadonlySet<number>
  stale: ReadonlySet<number>
  busy: ReadonlySet<number>
  homeId: number | null
  onToggle: (id: number) => void
  onSelect: (ids: number[]) => void
}

export function FleetPanel({ vehicles, selected, stale, busy, homeId, onToggle, onSelect }: Props) {
  const [expanded, setExpanded] = useState<number | null>(null)
  const ids = vehicles.map((v) => v.system_id!)
  const allSelected = ids.length > 0 && ids.every((id) => selected.has(id))

  return (
    <section className="card fleet">
      <div className="card-title">
        <span>Fleet</span>
        <button className="btn btn-ghost" disabled={ids.length === 0} onClick={() => onSelect(allSelected ? [] : ids)}>
          {allSelected ? 'Clear' : 'Select all'}
        </button>
      </div>
      <ul className="fleet-list">
        {vehicles.map((v) => {
          const id = v.system_id!
          return (
            <VehicleCard
              key={id}
              vehicle={v}
              selected={selected.has(id)}
              stale={stale.has(id)}
              busy={busy.has(id)}
              isHome={homeId === id}
              expanded={expanded === id}
              onToggleSelect={() => onToggle(id)}
              onToggleExpand={() => setExpanded(expanded === id ? null : id)}
            />
          )
        })}
      </ul>
      {vehicles.length === 0 && <p className="hint fleet-empty">No vehicles yet. Start SITL or connect a drone.</p>}
    </section>
  )
}
