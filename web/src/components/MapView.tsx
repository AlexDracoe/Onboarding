import { DomEvent, divIcon } from 'leaflet'
import { useEffect, useMemo, useRef } from 'react'
import { LayersControl, MapContainer, Marker, Polyline, TileLayer, Tooltip, useMap, useMapEvents, ZoomControl } from 'react-leaflet'
import type { VehicleState } from '../api.ts'
import { positionOf, type LatLon } from '../geo.ts'
import type { Home } from '../hooks/useSessionHome.ts'
import './MapView.css'

// ArduPilot SITL's default location (CMAC, Canberra), used until a drone gives us a home.
const FALLBACK_CENTER: LatLon = [-35.363262, 149.165237]

interface Props {
  vehicles: VehicleState[]
  selected: ReadonlySet<number>
  stale: ReadonlySet<number>
  trails: ReadonlyMap<number, LatLon[]>
  home: Home | null
  target: LatLon | null
  onToggle: (id: number) => void
  onPickTarget: (p: LatLon) => void
  onResetHome: () => void
}

export function MapView({ vehicles, selected, stale, trails, home, target, onToggle, onPickTarget, onResetHome }: Props) {
  return (
    <section className="card map-card">
      <MapContainer center={home?.position ?? FALLBACK_CENTER} zoom={18} maxZoom={21} zoomControl={false} className="map">
        <ZoomControl position="bottomright" />
        <LayersControl position="topright">
          <LayersControl.BaseLayer checked name="Satellite">
            <TileLayer
              url="https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"
              attribution="Tiles &copy; Esri"
              maxNativeZoom={19}
              maxZoom={21}
            />
          </LayersControl.BaseLayer>
          <LayersControl.BaseLayer name="Streets">
            <TileLayer
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
              attribution="&copy; OpenStreetMap contributors"
              maxNativeZoom={19}
              maxZoom={21}
            />
          </LayersControl.BaseLayer>
        </LayersControl>

        <FlyToHome home={home} />
        <ClickToPick onPick={onPickTarget} />

        {[...trails].map(([id, points]) => (
          <Polyline key={id} positions={points} pathOptions={{ color: selected.has(id) ? '#3b9eff' : '#ffffff', weight: 2, opacity: 0.55 }} />
        ))}

        {home && <HomeMarker home={home} />}
        {target && <TargetMarker position={target} />}

        {vehicles.map((v) => (
          <DroneMarker key={v.system_id} vehicle={v} selected={selected.has(v.system_id!)} stale={stale.has(v.system_id!)} onClick={() => onToggle(v.system_id!)} />
        ))}

        <MapToolbar home={home} onResetHome={onResetHome} />
      </MapContainer>
    </section>
  )
}

/** Centres the map on home once, when it first becomes known (or changes after a reset). */
function FlyToHome({ home }: { home: Home | null }) {
  const map = useMap()
  const lat = home?.position[0]
  const lon = home?.position[1]
  useEffect(() => {
    if (lat !== undefined && lon !== undefined) map.flyTo([lat, lon], Math.max(map.getZoom(), 18), { duration: 0.8 })
  }, [map, lat, lon])
  return null
}

function ClickToPick({ onPick }: { onPick: (p: LatLon) => void }) {
  useMapEvents({ click: (e) => onPick([e.latlng.lat, e.latlng.lng]) })
  return null
}

function MapToolbar({ home, onResetHome }: { home: Home | null; onResetHome: () => void }) {
  const map = useMap()
  const ref = useRef<HTMLDivElement>(null)
  // Otherwise clicking a button would also count as a map click and move the Go-to target.
  useEffect(() => {
    if (ref.current) DomEvent.disableClickPropagation(ref.current)
  }, [])
  return (
    <div ref={ref} className="map-toolbar leaflet-top leaflet-left">
      <div className="leaflet-control map-tools">
        <button className="btn" disabled={!home} onClick={() => home && map.flyTo(home.position, 18, { duration: 0.6 })} title="Centre on home">
          ⌂ Home
        </button>
        <button className="btn" onClick={onResetHome} title="Use the next connected drone as home">
          Reset home
        </button>
      </div>
      <p className="leaflet-control map-legend">Click the map to set a Go-to target · click a drone to select it</p>
    </div>
  )
}

function DroneMarker({ vehicle, selected, stale, onClick }: { vehicle: VehicleState; selected: boolean; stale: boolean; onClick: () => void }) {
  const position = positionOf(vehicle)
  const heading = Math.round(vehicle.heading ?? 0)
  const armed = vehicle.is_armed === true
  const icon = useMemo(
    () =>
      divIcon({
        className: '',
        iconSize: [44, 44],
        iconAnchor: [22, 22],
        html: `<div class="drone-icon ${selected ? 'is-selected' : ''} ${stale ? 'is-stale' : ''} ${armed ? 'is-armed' : ''}">
                 <svg viewBox="0 0 24 24" style="transform: rotate(${heading}deg)"><path d="M12 2 L20 20 L12 16 L4 20 Z"/></svg>
                 <span>${vehicle.system_id}</span>
               </div>`,
      }),
    [heading, selected, stale, armed, vehicle.system_id],
  )
  if (!position) return null
  const alt = vehicle.position?.relative_altitude
  return (
    <Marker position={position} icon={icon} eventHandlers={{ click: onClick }} zIndexOffset={selected ? 1000 : 0}>
      <Tooltip direction="top" offset={[0, -18]}>
        Drone {vehicle.system_id} · {vehicle.flight_mode ?? '–'} · {alt == null ? '–' : `${alt.toFixed(1)} m`}
      </Tooltip>
    </Marker>
  )
}

const homeIcon = divIcon({ className: '', iconSize: [28, 28], iconAnchor: [14, 14], html: '<div class="home-icon">H</div>' })
const targetIcon = divIcon({ className: '', iconSize: [30, 30], iconAnchor: [15, 15], html: '<div class="target-icon"></div>' })

function HomeMarker({ home }: { home: Home }) {
  return (
    <Marker position={home.position} icon={homeIcon} zIndexOffset={-100}>
      <Tooltip direction="top" offset={[0, -12]}>
        Home (first fix of Drone {home.systemId})
      </Tooltip>
    </Marker>
  )
}

function TargetMarker({ position }: { position: LatLon }) {
  return (
    <Marker position={position} icon={targetIcon} interactive={false} zIndexOffset={-50}>
      <Tooltip permanent direction="right" offset={[14, 0]}>
        Go-to target
      </Tooltip>
    </Marker>
  )
}
