import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Plus, Pencil, Trash2, Activity, Wifi } from "lucide-react";
import { nodesApi, credentialsApi, keysApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, StatusBadge, EmptyState, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";
import type { Node } from "@/api/types";

const emptyNode: Partial<Node> = {
  name: "",
  host: "",
  port: 22,
  username: "",
  enabled: true,
};

export function NodesPage() {
  const { t } = useTranslation();
  const { canWrite, isAdmin } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Partial<Node> | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const { data: nodes, isLoading } = useQuery({
    queryKey: ["nodes"],
    queryFn: nodesApi.list,
  });

  const { data: credentials } = useQuery({
    queryKey: ["credentials"],
    queryFn: credentialsApi.list,
    enabled: formOpen,
  });

  const { data: keys } = useQuery({
    queryKey: ["keys"],
    queryFn: keysApi.list,
    enabled: formOpen,
  });

  const saveMutation = useMutation({
    mutationFn: (node: Partial<Node>) =>
      node.id ? nodesApi.update(node.id, node) : nodesApi.create(node),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["nodes"] });
      setFormOpen(false);
      setEditing(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => nodesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["nodes"] });
      setDeleteId(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const pingMutation = useMutation({
    mutationFn: (id: string) => nodesApi.ping(id),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ["nodes"] });
      toast(
        result.reachable ? t("app.success") : t("app.error"),
        result.message ?? `${result.latency_ms}ms`,
        result.reachable ? "success" : "error",
      );
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const testMutation = useMutation({
    mutationFn: (id: string) => nodesApi.test(id),
    onSuccess: (result) =>
      toast(
        result.success ? t("app.success") : t("app.error"),
        result.message,
        result.success ? "success" : "error",
      ),
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const openCreate = () => {
    setEditing({ ...emptyNode });
    setFormOpen(true);
  };

  const openEdit = (node: Node) => {
    setEditing({ ...node });
    setFormOpen(true);
  };

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="page-title" style={{ margin: 0 }}>{t("nodes.title")}</h1>
        {canWrite && (
          <Button onClick={openCreate}>
            <Plus size={16} /> {t("nodes.add")}
          </Button>
        )}
      </div>

      <div className="card">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("app.name")}</th>
                <th>{t("nodes.host")}</th>
                <th>{t("nodes.port")}</th>
                <th>{t("app.status")}</th>
                <th>{t("nodes.lastPing")}</th>
                <th>{t("app.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {(nodes ?? []).length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <EmptyState message={t("app.noData")} />
                  </td>
                </tr>
              ) : (
                (nodes ?? []).map((node) => (
                  <tr key={node.id}>
                    <td>
                      <Link to={`/nodes/${node.id}`}>{node.name}</Link>
                    </td>
                    <td>{node.host}</td>
                    <td>{node.port}</td>
                    <td>
                      <StatusBadge status={node.enabled ? "enabled" : "disabled"} />
                      {node.last_ping_status && (
                        <StatusBadge status={node.last_ping_status} />
                      )}
                    </td>
                    <td className="text-sm text-muted">
                      {node.last_ping_at ? formatDate(node.last_ping_at) : "—"}
                    </td>
                    <td>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => pingMutation.mutate(node.id)} aria-label={t("nodes.ping")}>
                          <Wifi size={16} />
                        </Button>
                        {canWrite && (
                          <Button variant="ghost" size="icon" onClick={() => testMutation.mutate(node.id)} aria-label={t("nodes.testConnection")}>
                            <Activity size={16} />
                          </Button>
                        )}
                        {canWrite && (
                          <Button variant="ghost" size="icon" onClick={() => openEdit(node)} aria-label={t("app.edit")}>
                            <Pencil size={16} />
                          </Button>
                        )}
                        {isAdmin && (
                          <Button variant="ghost" size="icon" onClick={() => setDeleteId(node.id)} aria-label={t("app.delete")}>
                            <Trash2 size={16} />
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <Modal
        open={formOpen}
        onOpenChange={setFormOpen}
        title={editing?.id ? t("nodes.edit") : t("nodes.add")}
        size="lg"
      >
        {editing && (
          <>
            <div className="form-row">
              <FormGroup label={t("app.name")}>
                <Input value={editing.name ?? ""} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              </FormGroup>
              <FormGroup label={t("nodes.host")}>
                <Input value={editing.host ?? ""} onChange={(e) => setEditing({ ...editing, host: e.target.value })} />
              </FormGroup>
            </div>
            <div className="form-row">
              <FormGroup label={t("nodes.port")}>
                <Input type="number" value={editing.port ?? 22} onChange={(e) => setEditing({ ...editing, port: parseInt(e.target.value, 10) })} />
              </FormGroup>
              <FormGroup label={t("nodes.username")}>
                <Input value={editing.username ?? ""} onChange={(e) => setEditing({ ...editing, username: e.target.value })} />
              </FormGroup>
            </div>
            <div className="form-row">
              <FormGroup label={t("nodes.credential")}>
                <Select
                  value={editing.credential_id ?? ""}
                  onChange={(e) => setEditing({ ...editing, credential_id: e.target.value || undefined })}
                >
                  <option value="">{t("nodes.none")}</option>
                  {(credentials ?? []).map((c) => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </Select>
              </FormGroup>
              <FormGroup label={t("nodes.key")}>
                <Select
                  value={editing.key_id ?? ""}
                  onChange={(e) => setEditing({ ...editing, key_id: e.target.value || undefined })}
                >
                  <option value="">{t("nodes.none")}</option>
                  {(keys ?? []).map((k) => (
                    <option key={k.id} value={k.id}>{k.name}</option>
                  ))}
                </Select>
              </FormGroup>
            </div>
            <FormGroup label={t("app.enabled")}>
              <Select
                value={editing.enabled ? "true" : "false"}
                onChange={(e) => setEditing({ ...editing, enabled: e.target.value === "true" })}
              >
                <option value="true">{t("app.enabled")}</option>
                <option value="false">{t("app.disabled")}</option>
              </Select>
            </FormGroup>
            <div className="dialog-actions">
              <Button variant="secondary" onClick={() => setFormOpen(false)}>{t("app.cancel")}</Button>
              <Button onClick={() => saveMutation.mutate(editing)} disabled={saveMutation.isPending}>
                {t("app.save")}
              </Button>
            </div>
          </>
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={() => setDeleteId(null)}
        title={t("app.delete")}
        description={t("nodes.deleteConfirm", {
          name: nodes?.find((n) => n.id === deleteId)?.name ?? "",
        })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteId && deleteMutation.mutate(deleteId)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
