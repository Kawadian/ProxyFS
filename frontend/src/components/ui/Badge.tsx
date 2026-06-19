export function Badge({
  children,
  variant = "neutral",
}: {
  children: React.ReactNode;
  variant?: "success" | "warning" | "danger" | "neutral" | "primary";
}) {
  return <span className={`badge badge-${variant}`}>{children}</span>;
}

export function StatusBadge({ status }: { status: string }) {
  const map: Record<string, "success" | "warning" | "danger" | "neutral" | "primary"> = {
    completed: "success",
    running: "primary",
    pending: "neutral",
    paused: "warning",
    failed: "danger",
    cancelled: "neutral",
    reachable: "success",
    ok: "success",
    enabled: "success",
    disabled: "neutral",
  };
  return <Badge variant={map[status.toLowerCase()] ?? "neutral"}>{status}</Badge>;
}

export function LoadingSpinner() {
  return (
    <div className="loading-center" role="status" aria-label="Loading">
      <div className="spinner" />
    </div>
  );
}

export function EmptyState({
  icon,
  message,
}: {
  icon?: React.ReactNode;
  message: string;
}) {
  return (
    <div className="empty-state">
      {icon}
      <p>{message}</p>
    </div>
  );
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}
