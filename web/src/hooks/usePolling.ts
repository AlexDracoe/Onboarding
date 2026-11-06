import { useCallback, useEffect, useRef, useState } from 'react'

interface PollingOptions {
  intervalMs: number
  /** Cap for the backoff while `fn` keeps throwing. The delay doubles each failure, resets on success. */
  maxBackoffMs?: number
  /** false pauses polling (no calls are made). Turning it back on polls immediately. */
  enabled?: boolean
}

export interface PollingResult<T> {
  /** Last successful result (kept while later calls fail, so the UI can show it as stale). */
  data: T | undefined
  /** true while the most recent call threw. */
  failing: boolean
  /** Poll right now (restarts the loop, resets the backoff). */
  refresh: () => void
}

/**
 * Calls `fn` now and then every `intervalMs`, one call at a time (the next is scheduled after the
 * previous finishes, never overlapping). On failure the delay doubles up to `maxBackoffMs`.
 *
 * C++/Go analogy: a thread (goroutine) looping { call; sleep }, cancelled by the cleanup function
 * that useEffect returns (like closing a stop channel / setting a stop flag).
 */
export function usePolling<T>(fn: () => Promise<T>, options: PollingOptions): PollingResult<T> {
  const { intervalMs, maxBackoffMs = 8000, enabled = true } = options
  const [data, setData] = useState<T | undefined>(undefined)
  const [failing, setFailing] = useState(false)
  // Bumping this re-runs the effect below, which polls immediately.
  const [nonce, setNonce] = useState(0)

  // Always call the latest `fn` without restarting the loop when the caller's closure changes.
  const fnRef = useRef(fn)
  useEffect(() => {
    fnRef.current = fn
  })

  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined
    let delay = intervalMs

    const tick = async () => {
      try {
        const result = await fnRef.current()
        if (cancelled) return
        setData(result)
        setFailing(false)
        delay = intervalMs
      } catch {
        if (cancelled) return
        setFailing(true)
        delay = Math.min(delay * 2, maxBackoffMs)
      }
      timer = setTimeout(tick, delay)
    }
    tick()

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [intervalMs, maxBackoffMs, enabled, nonce])

  // Stable identity (like a function pointer that never changes), so callers can list it in effect deps.
  const refresh = useCallback(() => setNonce((n) => n + 1), [])

  return { data, failing, refresh }
}
