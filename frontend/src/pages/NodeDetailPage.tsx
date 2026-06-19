import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Wifi, Activity, ShieldCheck } from "lucide-react";
import { nodesApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { LoadingSpinner, StatusBadge, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";

export function NodeDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const { canWrite } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: node, isLoading } = useQuery({
    queryKey: ["nodes", id],
    queryFn: () => nodesApi.get(id!),
    enabled: !!id,
  });

  const pingMutation = useMutation({
    mutationFn: () => nodesApi.ping(id!),
    onSuccess: (r) => {
      queryClient.invalidateQueries({ queryKey: ["nodes", id] });
      toast(r.reachable ? t("app.success") : t("app.error"), r.message, r.reachable ? "success" : "error");
    },
  });

  const testMutation = useMutation({
    mutationFn: () => nodesApi.test(id!),
    onSuccess: (r) => toast(r.success ? t("app.success") : t("app.error"), r.message, r.success ? "success" : "error"),
  });

  const acceptKeyMutation = useMutation({
    mutationFn: (fp: string) => nodesApi.acceptHostKey(id!, fp),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["nodes", id] });
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (isLoading) return <LoadingSpinner />;
  if (!node) return <p>{t("errors.notFound")}</p>;

  return (
    <div>
      <Link to="/nodes" className="flex items-center gap-2 text-sm text-muted mb-4">
        <ArrowLeft size={16} /> {t("app.back")}
      </Link>
      <div className="flex justify-between items-center mb-4">
        <h1 className="page-title" style={{ margin: 0 }}>{node.name}</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => pingMutation.mutate()} disabled={pingMutation.isPending}>
            <Wifi size={16} /> {t("nodes.ping")}
          </Button>
          {canWrite && (
            <Button variant="secondary" size="sm" onClick={() => testMutation.mutate()} disabled={testMutation.isPending}>
              <Activity size={16} /> {t("nodes.testConnection")}
            </Button>
          )}
        </div>
      </div>

      <div className="card">
        <div className="card-body">
          <dl className="form-row" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(250px, 1fr))" }}>
            <div>
              <dt className="text-sm text-muted">{t("nodes.host")}</dt>
              <dd>{node.host}:{node.port}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted">{t("nodes.username")}</dt>
              <dd>{node.username}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted">{t("app.status")}</dt>
              <dd><StatusBadge status={node.enabled ? "enabled" : "disabled"} /></dd>
            </div>
            <div>
              <dt className="text-sm text-muted">{t("nodes.hostKey")}</dt>
              <dd>
                <StatusBadge status={node.host_key_status || "pending"} />
                {node.host_key_fingerprint && (
                  <code className="text-mono" style={{ display: "block", marginTop: 4 }}>{node.host_key_fingerprint}</code>
                )}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted">{t("nodes.lastPing")}</dt>
              <dd>{node.last_ping_at ? formatDate(node.last_ping_at) : "—"}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted">Created</dt>
              <dd>{formatDate(node.created_at)}</dd>
            </div>
          </dl>

          {canWrite && node.host_key_status !== "accepted" && node.host_key_fingerprint && (
            <Button
              className="mt-4"
              variant="secondary"
              onClick={() => acceptKeyMutation.mutate(node.host_key_fingerprint!)}
              disabled={acceptKeyMutation.isPending}
            >
              <ShieldCheck size={16} /> {t("nodes.acceptHostKey")}
            </Button>
          )}

          {node.labels && Object.keys(node.labels).length > 0 && (
            <div className="mt-4">
              <h3 className="text-sm text-muted">{t("nodes.labels")}</h3>
              <div className="flex gap-2 flex-wrap mt-2">
                {Object.entries(node.labels).map(([k, v]) => (
                  <span key={k} className="badge badge-neutral">{k}={v}</span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
