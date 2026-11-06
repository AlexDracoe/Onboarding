import { useCallback, useState } from 'react'

/** A set of selected system IDs. */
export function useSelection() {
  const [selected, setSelected] = useState<ReadonlySet<number>>(new Set())

  const toggle = useCallback((id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const set = useCallback((ids: Iterable<number>) => setSelected(new Set(ids)), [])

  return { selected, toggle, set }
}
