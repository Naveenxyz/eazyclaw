interface StatusCardProps {
  name: string;
  detail?: string;
}

export function StatusCard({ name, detail }: StatusCardProps) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <span className="h-2.5 w-2.5 flex-shrink-0 rounded-full bg-emerald-500" />
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-zinc-100">
          {name}
        </div>
        {detail && (
          <div className="truncate text-xs text-zinc-400">{detail}</div>
        )}
      </div>
    </div>
  );
}
