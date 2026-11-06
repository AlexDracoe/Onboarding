import type { CommandRecord } from '../api.ts'
import type { DispatchBatch } from '../hooks/useDispatch.ts'
import { StatusPill } from './StatusPill.tsx'

/** Per-drone outcome of the last command sent, followed live from the command log. */
export function DispatchStatus({ last, records, sending }: { last: DispatchBatch | null; records: CommandRecord[] | undefined; sending: boolean }) {
  if (!last && !sending) return null
  const byId = new Map((records ?? []).map((r) => [r.id, r]))
  return (
    <div className="cmd-group dispatch">
      <h3>Last command{last && <span className="mono"> · {last.name}</span>}</h3>
      {sending && <p className="hint">Sending…</p>}
      <ul>
        {last?.results.map((r) => {
          const record = r.commandId ? byId.get(r.commandId) : undefined
          return (
            <li key={r.systemId}>
              <span>Drone {r.systemId}</span>
              {r.error ? <span className="dispatch-error">{r.error}</span> : <StatusPill status={record?.status ?? 'PENDING'} />}
              {record?.error && <span className="dispatch-error">{record.error}</span>}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
