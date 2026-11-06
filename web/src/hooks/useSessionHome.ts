import { useCallback, useEffect, useState } from 'react'
import type { VehicleState } from '../api.ts'
import { positionOf, type LatLon } from '../geo.ts'

const KEY = 'drone-dashboard.home'

export interface Home {
  systemId: number
  position: LatLon
}

interface Stored {
  /** The first drone seen this session; remembered at once, even before it has a GPS fix. */
  firstId: number | null
  home: Home | null
}

const EMPTY: Stored = { firstId: null, home: null }

function load(): Stored {
  try {
    const raw = sessionStorage.getItem(KEY)
    if (raw) return JSON.parse(raw) as Stored
  } catch {
    // Storage blocked or corrupt: start fresh.
  }
  return EMPTY
}

/**
 * The map's home: where the first drone seen in this browser session was at its first GPS fix.
 * Kept in sessionStorage, so it survives reloads of this tab but a new tab starts over.
 */
export function useSessionHome(vehicles: VehicleState[]) {
  const [stored, setStored] = useState<Stored>(load)

  if (!stored.home) {
    const firstId = stored.firstId ?? vehicles[0]?.system_id ?? null
    if (firstId !== null) {
      const position = positionOf(vehicles.find((v) => v.system_id === firstId))
      if (position) setStored({ firstId, home: { systemId: firstId, position } })
      else if (firstId !== stored.firstId) setStored({ firstId, home: null })
    }
  }

  useEffect(() => {
    try {
      sessionStorage.setItem(KEY, JSON.stringify(stored))
    } catch {
      // Not fatal; home just won't survive a reload.
    }
  }, [stored])

  /** Forget the home; the first drone in the list becomes the new home. */
  const reset = useCallback(() => setStored(EMPTY), [])

  return { home: stored.home, reset }
}
