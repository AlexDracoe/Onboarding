import type { CommandRecord } from './api.ts'

const time = (unixSeconds: number) => new Date(unixSeconds * 1000).toLocaleTimeString()

export function CommandHistory({ records }: { records: CommandRecord[] | undefined }) {
  return (
    <section>
      <h2>Command history</h2>
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
          {(records ?? []).map((r) => (
            <tr key={r.id}>
              <td>{time(r.created_at)}</td>
              <td>{r.system_id}</td>
              <td>{r.cmd.name}</td>
              <td className={`status status-${r.status}`}>{r.status}</td>
              <td>{r.status === 'PENDING' ? '…' : `${(r.updated_at - r.created_at).toFixed(1)} s`}</td>
              <td>{r.error ?? ''}</td>
            </tr>
          ))}
          {records?.length === 0 && (
            <tr>
              <td colSpan={6}>No commands yet</td>
            </tr>
          )}
        </tbody>
      </table>
    </section>
  )
}
