const TONE: Record<string, string> = {
  PENDING: 'badge badge-warn',
  ACHIEVED: 'badge badge-ok',
}

export function StatusPill({ status }: { status: string }) {
  return (
    <span className={TONE[status] ?? 'badge badge-danger'}>
      {status === 'PENDING' && <span className="dot dot-pulse" />}
      {status}
    </span>
  )
}
