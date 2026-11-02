import { useEffect, useState } from 'react'
import './App.css'
import { ApiError, getCommand, getVehicles, isTerminal, listCommands, postCommand } from './api.ts'
import { CommandHistory } from './CommandHistory.tsx'
import { usePolling } from './usePolling.ts'
import { VehiclePanel } from './VehiclePanel.tsx'
import {MODES, TAKEOFF_MIN_ALT, TAKEOFF_MAX_ALT, GOTO_MIN_ALT, GOTO_MAX_ALT, type FlightMode} from './modes.ts'

//const SYSTEM_ID = 1
type GoToParams = {
    lat: number;
    long: number;
    altitude: number;
  }
export default function App() {
  
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const vehicles = usePolling(()=> getVehicles(), {intervalMs:1000})
  const list = vehicles.data ??[]
 
  const activeId = selectedId ?? list[0]?.system_id ?? null
  
  const vehicle = list.find((v) => v.system_id === activeId)
  const history = usePolling(() => listCommands(50, activeId ?? undefined), {intervalMs:2000})

  const noServer = vehicles.failing
  const noVehicle = !noServer && list.length === 0 && vehicles.data !== undefined
  const connected = !noServer && vehicle !== undefined

  const [trackedId, setTrackedId] = useState<string | null>(null)
  const [postError, setPostError] = useState<string | null>(null)
  const [posting, setPosting] = useState(false)
  const [mode, setMode] = useState<FlightMode | ''>('')
  const [takeoffAltitude, setTakeoffAltitude] = useState('')

  const [goToLatitude, setGoToLatitude] = useState("-35.363262");
  const [goToLongitude, setGoToLongitude] = useState("149.165237");
  const [goToAltitude, setGoToAltitude] = useState("20");


  const tracked = usePolling(() => getCommand(trackedId!), {
    intervalMs: 300,
    enabled: trackedId !== null,
  })
  // `tracked.data` keeps the previous command's record until the new one arrives, so check the id.
  const trackedDone = tracked.data?.id === trackedId && isTerminal(tracked.data.status)
  const refreshHistory = history.refresh
  useEffect(() => {
    if (trackedDone) refreshHistory()
  }, [trackedDone, refreshHistory])



  const pending =
    posting ||
    (trackedId !== null && !trackedDone) ||
    (history.data ?? []).some((r) => r.status === 'PENDING')

  async function send(name: string, params?: Record<string, unknown>) {
    if (activeId === null) return
    setPostError(null)
    setPosting(true)
    try {
      const accepted = await postCommand(activeId, name, params)
      setTrackedId(accepted.command_id)
    } catch (e) {
      setPostError(e instanceof ApiError ? `${e.code}: ${e.message}` : 'Could not reach the server')
    } finally {
      setPosting(false)
    }
  }
  const disabled = !connected || pending

  const altitude = Number(takeoffAltitude)
  const altitudeValid = takeoffAltitude !== '' && Number.isFinite(altitude) && altitude >= TAKEOFF_MIN_ALT && altitude <= TAKEOFF_MAX_ALT
  
  function handleGoTo() {
  const params: GoToParams = {
    lat: Number(goToLatitude),
    long: Number(goToLongitude),
    altitude: Number(goToAltitude),
  };

  if (
    goToLatitude.trim() === "" ||
    goToLongitude.trim() === "" ||
    goToAltitude.trim() === "" ||
    !Number.isFinite(params.lat) ||
    !Number.isFinite(params.long) ||
    !Number.isFinite(params.altitude) ||
    params.lat < -90 ||
    params.lat > 90 ||
    params.long < -180 ||
    params.long > 180 ||
    params.altitude < GOTO_MIN_ALT ||
    params.altitude > GOTO_MAX_ALT
  ) {
    setPostError(`Enter valid latitude, longitude, and an altitude between ${GOTO_MIN_ALT} and ${GOTO_MAX_ALT} m.`);
    return;
  }

  send("goto", params);
}

  return (
    <main>
      <h1>Drone dashboard</h1>
      {noServer && <p className="banner banner-error">No connection to the server. Retrying…</p>}
      {noVehicle && <p className="banner banner-warn">Server is up, waiting for the vehicle's first heartbeat…</p>}
      {vehicles.data === undefined && !noServer && <p className="banner">Connecting…</p>}

      <fieldset disabled={!connected} className={connected ? '' : 'offline'}>
        <VehiclePanel state={vehicle} />

        <section>
          <h2>Selected Drone</h2>
          <select value={activeId ?? ''} onChange={(e) => { setSelectedId(Number(e.target.value)); setTrackedId(null) }}> {list.map((v) => (
              <option key={v.system_id} value={v.system_id ??''}>Drone {v.system_id}</option>
            ))}</select>
          <h2>Commands</h2>
          <button disabled={disabled} onClick={() => send('arm')}>
            Arm
          </button>{' '}
          <button disabled={disabled} onClick={() => send('disarm')}>
            Disarm
          </button>{' '}
          <select value={mode} onChange={(e) => setMode(e.target.value as FlightMode | '')} disabled={disabled}>
            <option value=""> Select mode </option>
            {MODES.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
          </select>
          {' '}
          <button disabled={disabled || mode === ''} onClick={() => send('set_mode', {mode})}>
            SetMode
          </button>
          {' '}
          <h2>Takeoff Inputs</h2>
          <input type="number" 
          min={TAKEOFF_MIN_ALT} 
          max={TAKEOFF_MAX_ALT} 
          step="any" 
          value={takeoffAltitude}onChange={(e) => setTakeoffAltitude(e.target.value)}
          ></input>
          {' '}
          <button disabled={disabled || !altitudeValid} onClick={() => send('takeoff', { altitude })}>
          Takeoff
          </button>
          {' '}
          <h2>Go to inputs</h2>
          <label>Latitude</label>
          <input type="number" min={-90} max={90}
           step="any" value={goToLatitude}
           onChange={(e) => setGoToLatitude(e.target.value)}></input>{' '}

          <label>Longitude</label>
          <input type="number" min={-180} max={180}
           step="any" value={goToLongitude}
           onChange={(e) => setGoToLongitude(e.target.value)}></input>{' '}

          <button disabled={disabled} onClick={handleGoTo}>GO</button>

          <div></div><label>Altitude</label>
          <input type="number" min={GOTO_MIN_ALT} max={GOTO_MAX_ALT}
           step="any" value={goToAltitude}
           onChange={(e) => setGoToAltitude(e.target.value)}></input>{' '}
          {pending && <span className="hint"> command in progress…</span>}
          {postError && <p className="banner banner-error">{postError}</p>}
        </section>
        <CommandHistory records={history.data} />
      </fieldset>
    </main>
  )
}
