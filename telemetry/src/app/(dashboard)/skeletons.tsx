function Block({ className }: { className: string }) {
  return <div className={`skeleton rounded-[6px] ${className}`} />;
}

function CardSkeleton({ height, className = "" }: { height: number; className?: string }) {
  return (
    <div className={`rounded-card border border-border bg-surface p-4 ${className}`}>
      <Block className="mb-3 h-6 w-32" />
      <div className="skeleton rounded-[8px] opacity-60" style={{ height }} />
    </div>
  );
}

/** Mirrors the overview geometry so nothing jumps when data streams in. */
export function OverviewSkeleton() {
  return (
    <div className="grid gap-4" aria-busy="true" aria-label="Loading overview">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        {Array.from({ length: 6 }, (_, i) => (
          <div key={i} className="rounded-card border border-border bg-surface px-4 py-3.5">
            <Block className="h-[18px] w-20" />
            <Block className="mt-1.5 h-9 w-16" />
          </div>
        ))}
      </div>
      <CardSkeleton height={260} />
      <div className="grid gap-4 md:grid-cols-2">
        <CardSkeleton height={180} />
        <CardSkeleton height={180} />
      </div>
      <div className="grid gap-4 lg:grid-cols-3">
        <CardSkeleton height={290} className="lg:col-span-2" />
        <CardSkeleton height={290} />
      </div>
    </div>
  );
}

export function TableSkeleton() {
  return (
    <div className="overflow-hidden rounded-card border border-border bg-surface" aria-busy="true" aria-label="Loading instances">
      <div className="h-[38px] border-b border-border" />
      {Array.from({ length: 10 }, (_, i) => (
        <div key={i} className="flex h-12 items-center gap-6 border-b border-[#262626] px-4 last:border-0">
          <Block className="h-4 w-24" />
          <Block className="h-4 w-20" />
          <Block className="h-4 w-28" />
          <Block className="ml-auto h-4 w-16" />
        </div>
      ))}
    </div>
  );
}
