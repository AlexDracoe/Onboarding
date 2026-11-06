import { useState } from 'react'
import { TAKEOFF_MAX_ALT, TAKEOFF_MIN_ALT } from '../modes.ts'
import type { Send } from './CommandPanel.tsx'

export function TakeoffForm({ disabled, onSend }: { disabled: boolean; onSend: Send }) {
  const [text, setText] = useState('10')
  const altitude = Number(text)
  const valid = text !== '' && Number.isFinite(altitude) && altitude >= TAKEOFF_MIN_ALT && altitude <= TAKEOFF_MAX_ALT
  return (
    <div className="cmd-group">
      <h3>Takeoff</h3>
      <div className="row">
        <label className="field">
          <span>
            Altitude (m, {TAKEOFF_MIN_ALT}–{TAKEOFF_MAX_ALT})
          </span>
          <input className="input" type="number" min={TAKEOFF_MIN_ALT} max={TAKEOFF_MAX_ALT} step="any" value={text} onChange={(e) => setText(e.target.value)} />
        </label>
        <button className="btn btn-primary" disabled={disabled || !valid} onClick={() => onSend('takeoff', { altitude })}>
          Takeoff
        </button>
      </div>
      <p className="hint">Needs GUIDED mode and armed.</p>
    </div>
  )
}
