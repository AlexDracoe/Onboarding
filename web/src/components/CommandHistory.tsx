import { useState } from 'react'
import type { CommandRecord } from '../api.ts'
import { StatusPill } from './StatusPill.tsx'
import './CommandHistory.css'

const time = (unixSeconds: number) => new Date(unixSeconds * 1000).toLocaleTimeString()

export function CommandHistory({ records, selected }: { records: CommandRecord[] | undefined; selected: ReadonlySet<number> }) {
  const [onlySelected, setOnlySelected] = useState(false)
  const rows = (records ?? []).filter((r) => !onlySelected || selected.has(r.system_id))

  return (
    <section className="card history">
      <div className="card-title">
        <span>Command history</span>
        <label className="history-filter">
          <input type="checkbox" checked={onlySelected} onChange={(e) => setOnlySelected(e.target.checked)} />
          Selected drones only
        </label>
      </div>
      <div className="history-scroll">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Drone</th>
              <th>Command</th>
              <th>Status</th>
              <th>Took</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id}>
                <td className="mono">{time(r.created_at)}</td>
                <td>{r.system_id}</td>
                <td className="mono">{r.cmd.name}</td>
                <td>
                  <StatusPill status={r.status} />
                </td>
                <td className="mono">{r.status === 'PENDING' ? '…' : `${(r.updated_at - r.created_at).toFixed(1)} s`}</td>
                <td className="history-error">{r.error ?? ''}</td>
              </tr>
            ))}
            {records !== undefined && rows.length === 0 && (
              <tr>
                <td colSpan={6} className="hint">
                  No commands yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  )
}
