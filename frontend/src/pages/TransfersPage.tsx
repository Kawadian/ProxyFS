import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Pause, Play, XCircle, RotateCcw, Trash2 } from "lucide-react";
import { transfersApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, StatusBadge, EmptyState, formatBytes, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";

export function TransfersPage() {
  const { t } = useTranslation();
  const { canWrite } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const { data: transfers, isLoading } = useQuery({
    queryKey: ["transfers"],
    queryFn: transfersApi.list,
    refetchInterval: 5_000,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["transfers"] });

  const actionMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: "pause" | "resume" | "cancel" | "retry" }) => {
      const fns = {
        pause: transfersApi.pause,
        resume: transfersApi.resume,
        cancel: transfersApi.cancel,
        retry: transfersApi.retry,
      };
      return fns[action](id);
    },
    onSuccess: () => { invalidate(); toast(t("app.success"), undefined, "success"); },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => transfersApi.delete(id),
    onSuccess: () => { invalidate(); setDeleteId(null); toast(t("app.success"), undefined, "success"); },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <h1 className="page-title">{t("transfers.title")}</h1>

      <div className="card">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("transfers.source")}</th>
                <th>{t("transfers.dest")}</th>
                <th>{t("transfers.direction")}</th>
                <th>{t("app.status")}</th>
                <th>{t("transfers.progress")}</th>
                <th>Created</th>
                {canWrite && <th>{t("app.actions")}</th>}
              </tr>
            </thead>
            <tbody>
              {(transfers ?? []).length === 0 ? (
                <tr><td colSpan={canWrite ? 7 : 6}><EmptyState message={t("app.noData")} /></td></tr>
              ) : (
                (transfers ?? []).map((tr) => {
                  const pct = tr.bytes_total > 0 ? Math.round((tr.bytes_done / tr.bytes_total) * 100) : 0;
                  return (
                    <tr key={tr.id}>
                      <td className="truncate" style={{ maxWidth: 180 }}>{tr.source_path}</td>
                      <td className="truncate" style={{ maxWidth: 180 }}>{tr.dest_path}</td>
                      <td>{tr.direction}</td>
                      <td><StatusBadge status={tr.status} /></td>
                      <td>
                        <div style={{ minWidth: 100 }}>
                          <div className="text-sm">{pct}% ({formatBytes(tr.bytes_done)})</div>
                          <div className="progress-bar mt-1">
                            <div className="progress-fill" style={{ width: `${pct}%` }} />
                          </div>
                        </div>
                      </td>
                      <td className="text-sm text-muted">{formatDate(tr.created_at)}</td>
                      {canWrite && (
                        <td>
                          <div className="flex gap-1">
                            {tr.status === "running" && (
                              <Button variant="ghost" size="icon" onClick={() => actionMutation.mutate({ id: tr.id, action: "pause" })} aria-label={t("transfers.pause")}>
                                <Pause size={16} />
                              </Button>
                            )}
                            {tr.status === "paused" && (
                              <Button variant="ghost" size="icon" onClick={() => actionMutation.mutate({ id: tr.id, action: "resume" })} aria-label={t("transfers.resume")}>
                                <Play size={16} />
                              </Button>
                            )}
                            {["running", "paused", "pending"].includes(tr.status) && (
                              <Button variant="ghost" size="icon" onClick={() => actionMutation.mutate({ id: tr.id, action: "cancel" })} aria-label={t("transfers.cancel")}>
                                <XCircle size={16} />
                              </Button>
                            )}
                            {["failed", "cancelled"].includes(tr.status) && (
                              <Button variant="ghost" size="icon" onClick={() => actionMutation.mutate({ id: tr.id, action: "retry" })} aria-label={t("transfers.retry")}>
                                <RotateCcw size={16} />
                              </Button>
                            )}
                            <Button variant="ghost" size="icon" onClick={() => setDeleteId(tr.id)} aria-label={t("app.delete")}>
                              <Trash2 size={16} />
                            </Button>
                          </div>
                        </td>
                      )}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={() => setDeleteId(null)}
        title={t("app.delete")}
        description={t("transfers.deleteConfirm")}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteId && deleteMutation.mutate(deleteId)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
