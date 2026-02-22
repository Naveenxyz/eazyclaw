interface StatusCardProps {
  name: string;
  detail?: string;
}

export function StatusCard({ name, detail }: StatusCardProps) {
  return (
    <div className="glass-card flex items-center gap-3 rounded-lg border border-violet-500/10 bg-[#0f1117] p-4">
      <span className="status-dot connected h-2.5 w-2.5 flex-shrink-0 rounded-full bg-emerald-500 shadow-[0_0_6px_1px] shadow-emerald-500/50" />
      <div className="min-w-0">
        <div className="truncate text-sm font-mono font-medium text-slate-200">
          {name}
        </div>
        {detail && (
          <div className="truncate text-xs text-slate-400">{detail}</div>
        )}
      </div>
    </div>
  );
}
