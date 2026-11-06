import type { VehicleState } from '../api.ts'

const fmt = (v: number | null | undefined, digits = 1) => (v == null ? '–' : v.toFixed(digits))

/** Full telemetry table for one vehicle. */
export function VehicleDetails({ state }: { state: VehicleState }) {
  const rows: [string, string][] = [
    ['Status', state.system_status ?? '–'],
    ['Type', state.vehicle_type ?? '–'],
    ['Autopilot', state.autopilot ?? '–'],
    ['Heartbeats', String(state.heartbeat_count)],
    ['Latitude', fmt(state.position?.latitude, 6)],
    ['Longitude', fmt(state.position?.longitude, 6)],
    ['Altitude (AMSL)', `${fmt(state.position?.altitude)} m`],
    ['Altitude (rel)', `${fmt(state.position?.relative_altitude)} m`],
    ['Heading', `${fmt(state.heading, 0)}°`],
    ['Velocity x/y/z', [state.velocity?.velocity_x, state.velocity?.velocity_y, state.velocity?.velocity_z].map((v) => fmt(v)).join(' / ')],
  ]
  return (
    <dl className="details">
      {rows.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd className="mono">{value}</dd>
        </div>
      ))}
    </dl>
  )
}
