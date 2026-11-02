import type { VehicleState } from './api.ts'

const fmt = (v: number | null | undefined, digits = 1) => (v == null ? '–' : v.toFixed(digits))

export function VehiclePanel({ state }: { state: VehicleState | null | undefined }) {
  const rows: [string, string][] = [
    ['Armed', state?.is_armed == null ? '–' : state.is_armed ? 'YES' : 'no'],
    ['Mode', state?.flight_mode ?? '–'],
    ['Status', state?.system_status ?? '–'],
    ['Type', state?.vehicle_type ?? '–'],
    ['Heartbeats', state ? String(state.heartbeat_count) : '–'],
    ['Latitude', fmt(state?.position?.latitude, 6)],
    ['Longitude', fmt(state?.position?.longitude, 6)],
    ['Altitude (rel)', fmt(state?.position?.relative_altitude)],
    ['Heading', fmt(state?.heading, 0)],
    ['Velocity x/y/z', [state?.velocity?.velocity_x, state?.velocity?.velocity_y, state?.velocity?.velocity_z].map((v) => fmt(v)).join(' / ')],
  ]
  return (
    <section>
      <h2>Vehicle</h2>
      <table>
        <tbody>
          {rows.map(([label, value]) => (
            <tr key={label}>
              <th>{label}</th>
              <td>{value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
