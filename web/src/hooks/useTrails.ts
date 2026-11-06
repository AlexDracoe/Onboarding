import { useState } from 'react'
import type { VehicleState } from '../api.ts'
import { positionOf, type LatLon } from '../geo.ts'

const MAX_POINTS = 150

/** Recent positions per vehicle, for drawing a trail behind each drone on the map. */
export function useTrails(snapshot: VehicleState[] | undefined): ReadonlyMap<number, LatLon[]> {
  const [trails, setTrails] = useState<ReadonlyMap<number, LatLon[]>>(new Map())
  const [seen, setSeen] = useState(snapshot)

  // Extend the trails once per new poll result (React's "adjust state when a prop changes" pattern).
  if (snapshot !== seen) {
    setSeen(snapshot)
    const next = new Map(trails)
    for (const v of snapshot ?? []) {
      const p = positionOf(v)
      if (v.system_id == null || !p) continue
      const trail = trails.get(v.system_id) ?? []
      const last = trail[trail.length - 1]
      if (last && last[0] === p[0] && last[1] === p[1]) continue
      next.set(v.system_id, [...trail, p].slice(-MAX_POINTS))
    }
    setTrails(next)
  }

  return trails
}
