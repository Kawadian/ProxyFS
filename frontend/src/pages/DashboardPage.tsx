import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Server, ArrowLeftRight, Users, HardDrive, AlertTriangle, Activity } from "lucide-react";
import { dashboardApi, transfersApi, protocolsApi } from "@/api/endpoints";
import { healthCheck } from "@/api/client";
import { LoadingSpinner, StatusBadge, formatBytes, formatDate } from "@/components/ui/Badge";

export function DashboardPage() {
  const { t } = useTranslation();

  const { data: dash, isLoading } = useQuery({
    queryKey: ["dashboard"],
    queryFn: dashboardApi.get,
    refetchInterval: 30_000,
  });

  const { data: transfers } = useQuery({
    queryKey: ["transfers"],
    queryFn: transfersApi.list,
    refetchInterval: 10_000,
  });

  const { data: protocols } = useQuery({
    queryKey: ["protocols"],
    queryFn: protocolsApi.get,
    refetchInterval: 15_000,
  });

  const { data: health } = useQuery({
    queryKey: ["health"],
    queryFn: async () => ({
      live: await healthCheck("/health/live"),
      ready: await healthCheck("/health/ready"),
    }),
    refetchInterval: 30_000,
  });

  if (isLoading) return <LoadingSpinner />;

  const recentTransfers = (transfers ?? []).slice(0, 5);

  return (
    <div>
      <h1 className="page-title">{t("dashboard.title")}</h1>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-label flex items-center gap-2">
            <Server size={14} /> {t("dashboard.nodes")}
          </div>
          <div className="stat-value">{dash?.node_count ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label flex items-center gap-2">
            <ArrowLeftRight size={14} /> {t("dashboard.activeTransfers")}
          </div>
          <div className="stat-value">{dash?.active_transfers ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label flex items-center gap-2">
            <Users size={14} /> {t("dashboard.users")}
          </div>
          <div className="stat-value">{dash?.total_users ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label flex items-center gap-2">
            <HardDrive size={14} /> {t("dashboard.storage")}
          </div>
          <div className="stat-value">{dash?.storage_used_mb ?? 0} MB</div>
        </div>
        <div className="stat-card">
          <div className="stat-label flex items-center gap-2">
            <AlertTriangle size={14} /> {t("dashboard.recentErrors")}
          </div>
          <div className="stat-value">{dash?.recent_errors ?? 0}</div>
        </div>
      </div>

      <div className="form-row mb-4">
        <div className="card">
          <div className="card-header">
            <h2 className="card-title flex items-center gap-2">
              <Activity size={18} /> {t("dashboard.health")}
            </h2>
          </div>
          <div className="card-body flex gap-4">
            <div>
              <span className="text-sm text-muted">{t("dashboard.live")}: </span>
              <StatusBadge status={health?.live ? "ok" : "failed"} />
            </div>
            <div>
              <span className="text-sm text-muted">{t("dashboard.ready")}: </span>
              <StatusBadge status={health?.ready ? "ok" : "failed"} />
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="card-title">{t("dashboard.protocols")}</h2>
          </div>
          <div className="card-body flex gap-4 flex-wrap">
            {(protocols?.protocols ?? []).map((p) => (
              <div key={p.name}>
                <span className="text-sm text-muted">{t(`protocols.names.${p.name}`)}: </span>
                <StatusBadge status={p.running ? "ok" : p.enabled ? "pending" : "disabled"} />
              </div>
            ))}
            <div>
              <span className="text-sm text-muted">{t("dashboard.api")}: </span>
              <StatusBadge status={health?.ready ? "ok" : "pending"} />
            </div>
          </div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h2 className="card-title">{t("dashboard.recentTransfers")}</h2>
        </div>
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("transfers.source")}</th>
                <th>{t("transfers.dest")}</th>
                <th>{t("app.status")}</th>
                <th>{t("transfers.progress")}</th>
              </tr>
            </thead>
            <tbody>
              {recentTransfers.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-muted" style={{ textAlign: "center" }}>
                    {t("app.noData")}
                  </td>
                </tr>
              ) : (
                recentTransfers.map((tr) => (
                  <tr key={tr.id}>
                    <td className="truncate" style={{ maxWidth: 200 }}>{tr.source_path}</td>
                    <td className="truncate" style={{ maxWidth: 200 }}>{tr.dest_path}</td>
                    <td><StatusBadge status={tr.status} /></td>
                    <td>
                      {tr.bytes_total > 0
                        ? `${Math.round((tr.bytes_done / tr.bytes_total) * 100)}% (${formatBytes(tr.bytes_done)})`
                        : formatDate(tr.created_at)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
