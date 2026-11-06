import { useRef } from 'react'
import { getVehicles, type VehicleState } from '../api.ts'
import { usePolling } from './usePolling.ts'

/** A vehicle whose heartbeat count has not moved for this long is shown as "link lost". */
const STALE_MS = 3000

export type LinkStatus = 'connecting' | 'offline' | 'waiting' | 'online'

export interface Fleet {
  /** Vehicles with a system ID, sorted by it. */
  vehicles: VehicleState[]
  /** The latest raw poll result; a new array each poll, so it can be used to detect a fresh poll. */
  snapshot: VehicleState[] | undefined
  status: LinkStatus
  stale: ReadonlySet<number>
}

const EMPTY: ReadonlySet<number> = new Set()

/** Polls GET /vehicles and works out server status and which vehicles stopped sending heartbeats. */
export function useFleet(): Fleet {
  const lastChange = useRef(new Map<number, { count: number; at: number }>())

  // Staleness is worked out right after each fetch, so it is part of the polled result.
  const poll = usePolling(async () => {
    const snapshot = await getVehicles()
    const now = Date.now()
    const stale = new Set<number>()
    for (const v of snapshot) {
      if (v.system_id == null) continue
      const prev = lastChange.current.get(v.system_id)
      if (!prev || prev.count !== v.heartbeat_count) {
        lastChange.current.set(v.system_id, { count: v.heartbeat_count, at: now })
      } else if (now - prev.at > STALE_MS) {
        stale.add(v.system_id)
      }
    }
    return { snapshot, stale }
  }, { intervalMs: 1000 })

  const snapshot = poll.data?.snapshot
  const vehicles = (snapshot ?? [])
    .filter((v) => v.system_id != null)
    .sort((a, b) => a.system_id! - b.system_id!)

  let status: LinkStatus = 'online'
  if (poll.failing) status = 'offline'
  else if (snapshot === undefined) status = 'connecting'
  else if (vehicles.length === 0) status = 'waiting'

  return { vehicles, snapshot, status, stale: poll.data?.stale ?? EMPTY }
}
