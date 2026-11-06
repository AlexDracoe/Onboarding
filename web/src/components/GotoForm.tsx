import { useState } from 'react'
import type { LatLon } from '../geo.ts'
import { GOTO_MAX_ALT, GOTO_MIN_ALT } from '../modes.ts'
import type { Send } from './CommandPanel.tsx'

interface Props {
  disabled: boolean
  onSend: Send
  /** Shared with the map: clicking the map sets it, typing here moves the marker. */
  target: LatLon | null
  onTargetChange: (p: LatLon | null) => void
}

const parse = (s: string) => (s.trim() === '' ? NaN : Number(s))

export function GotoForm({ disabled, onSend, target, onTargetChange }: Props) {
  const [altText, setAltText] = useState('20')
  // Local text so half-typed numbers ("-35.") are not thrown away; synced from the map when it changes.
  const [latText, setLatText] = useState('')
  const [lonText, setLonText] = useState('')
  const [synced, setSynced] = useState<LatLon | null>(null)
  if (target !== synced) {
    setSynced(target)
    if (target) {
      setLatText(target[0].toFixed(7))
      setLonText(target[1].toFixed(7))
    }
  }

  function edit(lat: string, lon: string) {
    setLatText(lat)
    setLonText(lon)
    const la = parse(lat)
    const lo = parse(lon)
    const next: LatLon | null = Number.isFinite(la) && Number.isFinite(lo) && Math.abs(la) <= 90 && Math.abs(lo) <= 180 ? [la, lo] : null
    setSynced(next)
    onTargetChange(next)
  }

  const altitude = parse(altText)
  const altValid = Number.isFinite(altitude) && altitude >= GOTO_MIN_ALT && altitude <= GOTO_MAX_ALT
  const valid = target !== null && altValid

  return (
    <div className="cmd-group">
      <h3>Go to</h3>
      <div className="goto-grid">
        <label className="field">
          <span>Latitude</span>
          <input className="input" type="number" step="any" min={-90} max={90} value={latText} placeholder="click map" onChange={(e) => edit(e.target.value, lonText)} />
        </label>
        <label className="field">
          <span>Longitude</span>
          <input className="input" type="number" step="any" min={-180} max={180} value={lonText} placeholder="click map" onChange={(e) => edit(latText, e.target.value)} />
        </label>
        <label className="field">
          <span>Alt (m)</span>
          <input className="input" type="number" step="any" min={GOTO_MIN_ALT} max={GOTO_MAX_ALT} value={altText} onChange={(e) => setAltText(e.target.value)} />
        </label>
      </div>
      <div className="row">
        <button className="btn btn-primary grow" disabled={disabled || !valid} onClick={() => target && onSend('goto', { lat: target[0], long: target[1], altitude })}>
          Go
        </button>
        <button className="btn" disabled={target === null} onClick={() => edit('', '')}>
          Clear
        </button>
      </div>
      {!altValid && (
        <p className="hint">
          Altitude must be {GOTO_MIN_ALT}–{GOTO_MAX_ALT} m.
        </p>
      )}
      <p className="hint">All targets fly to the same point; the backend rejects targets over 500 m from a drone.</p>
    </div>
  )
}
