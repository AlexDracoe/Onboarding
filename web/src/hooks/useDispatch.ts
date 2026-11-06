import { useCallback, useState } from 'react'
import { ApiError, postCommand } from '../api.ts'

export interface DispatchResult {
  systemId: number
  /** Set when the server accepted the command (202); its progress is in the command log. */
  commandId?: string
  /** Set when the POST itself failed (bad params, server down). */
  error?: string
}

export interface DispatchBatch {
  name: string
  results: DispatchResult[]
}

function describe(e: unknown): string {
  return e instanceof ApiError ? `${e.code}: ${e.message}` : 'Could not reach the server'
}

/** Sends one command to many vehicles at once (one POST each, in parallel). */
export function useDispatch(onSent: () => void) {
  const [sending, setSending] = useState(false)
  const [last, setLast] = useState<DispatchBatch | null>(null)

  const send = useCallback(
    async (systemIds: number[], name: string, params?: Record<string, unknown>) => {
      if (systemIds.length === 0) return
      setSending(true)
      const settled = await Promise.allSettled(systemIds.map((id) => postCommand(id, name, params)))
      const results = settled.map((s, i): DispatchResult =>
        s.status === 'fulfilled'
          ? { systemId: systemIds[i], commandId: s.value.command_id }
          : { systemId: systemIds[i], error: describe(s.reason) },
      )
      setLast({ name, results })
      setSending(false)
      onSent()
    },
    [onSent],
  )

  return { send, sending, last }
}
