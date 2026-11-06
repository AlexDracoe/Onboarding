import type { VehicleState } from './api.ts'

export type LatLon = [lat: number, lon: number]

/** Where the vehicle is, or null before a GPS fix (ArduPilot reports 0,0 until then). */
export function positionOf(v: VehicleState | undefined): LatLon | null {
  const lat = v?.position?.latitude
  const lon = v?.position?.longitude
  if (lat == null || lon == null || (lat === 0 && lon === 0)) return null
  return [lat, lon]
}

/** Great-circle distance in metres (haversine). */
export function distanceM([lat1, lon1]: LatLon, [lat2, lon2]: LatLon): number {
  const R = 6_371_000
  const rad = Math.PI / 180
  const dLat = (lat2 - lat1) * rad
  const dLon = (lon2 - lon1) * rad
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(a))
}
