export function Line({ className = '' }) {
  return <div className={`h-4 bg-surface-2 rounded animate-pulse-soft ${className}`} />
}

export function Block({ className = '' }) {
  return <div className={`h-20 bg-surface-2 rounded-lg animate-pulse-soft ${className}`} />
}

export function StatCard() {
  return (
    <div className="card p-5 space-y-3">
      <Line className="w-20 h-3" />
      <Line className="w-14 h-8" />
    </div>
  )
}

export function MemoryCard() {
  return (
    <div className="card p-4 space-y-3">
      <div className="flex justify-between items-start">
        <Line className="w-2/3 h-5" />
        <Line className="w-16 h-5" />
      </div>
      <Line className="w-full" />
      <Line className="w-4/5" />
      <div className="flex gap-2 pt-1">
        <Line className="w-14 h-5" />
        <Line className="w-14 h-5" />
        <Line className="w-10 h-5" />
      </div>
    </div>
  )
}

const Skeleton = { Line, Block, StatCard, MemoryCard }
export default Skeleton
