import { useState } from 'react'
import { MODES, type FlightMode } from '../modes.ts'
import type { Send } from './CommandPanel.tsx'

export function ModeForm({ disabled, onSend }: { disabled: boolean; onSend: Send }) {
  const [mode, setMode] = useState<FlightMode | ''>('')
  return (
    <div className="cmd-group">
      <h3>Flight mode</h3>
      <div className="row">
        <select className="input" value={mode} onChange={(e) => setMode(e.target.value as FlightMode | '')}>
          <option value="">Select mode…</option>
          {MODES.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
        <button className="btn btn-primary" disabled={disabled || mode === ''} onClick={() => onSend('set_mode', { mode })}>
          Set
        </button>
      </div>
    </div>
  )
}
