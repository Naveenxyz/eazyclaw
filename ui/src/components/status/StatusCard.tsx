interface StatusCardProps {
  name: string;
  detail?: string;
}

export function StatusCard({ name, detail }: StatusCardProps) {
  return (
    <div className="card card-accent flex items-center gap-3 p-4 border-l-2 border-accent">
      <div className="status-indicator online" />
      <div className="min-w-0">
        <div className="truncate text-sm font-mono font-medium text-fg">
          {name}
        </div>
        {detail && (
          <div className="truncate text-xs text-fg-3">{detail}</div>
        )}
      </div>
    </div>
  );
}
