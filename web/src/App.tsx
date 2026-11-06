import { useState } from 'react'
import './App.css'
import { listCommands } from './api.ts'
import { CommandHistory } from './components/CommandHistory.tsx'
import { CommandPanel } from './components/CommandPanel.tsx'
import { FleetPanel } from './components/FleetPanel.tsx'
import { Header } from './components/Header.tsx'
import { MapView } from './components/MapView.tsx'
import type { LatLon } from './geo.ts'
import { useDispatch } from './hooks/useDispatch.ts'
import { useFleet } from './hooks/useFleet.ts'
import { usePolling } from './hooks/usePolling.ts'
import { useSelection } from './hooks/useSelection.ts'
import { useSessionHome } from './hooks/useSessionHome.ts'
import { useTrails } from './hooks/useTrails.ts'

export default function App() {
  const { vehicles, snapshot, status, stale } = useFleet()
  const { home, reset: resetHome } = useSessionHome(vehicles)
  const trails = useTrails(snapshot)
  const { selected, toggle, set: setSelected } = useSelection()
  const [gotoTarget, setGotoTarget] = useState<LatLon | null>(null)

  const history = usePolling(() => listCommands(100), { intervalMs: 1000 })
  const busy = new Set((history.data ?? []).filter((r) => r.status === 'PENDING').map((r) => r.system_id))
  const dispatch = useDispatch(history.refresh)

  // Commands go to every selected drone that is connected, still sending heartbeats and not mid-command.
  const targets = vehicles.map((v) => v.system_id!).filter((id) => selected.has(id) && !stale.has(id) && !busy.has(id))

  return (
    <div className="app">
      <Header status={status} vehicleCount={vehicles.length} selectedCount={selected.size} />
      <main className="layout">
        <FleetPanel vehicles={vehicles} selected={selected} stale={stale} busy={busy} homeId={home?.systemId ?? null} onToggle={toggle} onSelect={setSelected} />
        <MapView
          vehicles={vehicles}
          selected={selected}
          stale={stale}
          trails={trails}
          home={home}
          target={gotoTarget}
          onToggle={toggle}
          onPickTarget={setGotoTarget}
          onResetHome={resetHome}
        />
        <CommandPanel
          selected={selected}
          vehicles={vehicles}
          stale={stale}
          busy={busy}
          targets={targets}
          sending={dispatch.sending}
          onSend={(name, params) => dispatch.send(targets, name, params)}
          goto={gotoTarget}
          onGotoChange={setGotoTarget}
          last={dispatch.last}
          records={history.data}
        />
        <CommandHistory records={history.data} selected={selected} />
      </main>
    </div>
  )
}
