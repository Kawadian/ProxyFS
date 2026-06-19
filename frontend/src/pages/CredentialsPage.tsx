import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Plus, Pencil, Trash2, Activity } from "lucide-react";
import { credentialsApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, EmptyState, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";
import type { Credential } from "@/api/types";

export function CredentialsPage() {
  const { t } = useTranslation();
  const { isAdmin } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<{ id?: string; name: string; type: string; username: string; secret: string } | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const { data: credentials, isLoading } = useQuery({
    queryKey: ["credentials"],
    queryFn: credentialsApi.list,
  });

  const saveMutation = useMutation({
    mutationFn: (data: { id?: string; name: string; type: string; username: string; secret: string }) =>
      data.id
        ? credentialsApi.update(data.id, data)
        : credentialsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["credentials"] });
      setFormOpen(false);
      setEditing(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => credentialsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["credentials"] });
      setDeleteId(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const testMutation = useMutation({
    mutationFn: (id: string) => credentialsApi.test(id),
    onSuccess: (r) => toast(r.success ? t("app.success") : t("app.error"), r.message, r.success ? "success" : "error"),
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const openCreate = () => {
    setEditing({ name: "", type: "password", username: "", secret: "" });
    setFormOpen(true);
  };

  const openEdit = (c: Credential) => {
    setEditing({ id: c.id, name: c.name, type: c.type, username: c.username ?? "", secret: "" });
    setFormOpen(true);
  };

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="page-title" style={{ margin: 0 }}>{t("credentials.title")}</h1>
        {isAdmin && (
          <Button onClick={openCreate}>
            <Plus size={16} /> {t("credentials.add")}
          </Button>
        )}
      </div>

      <div className="card">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("app.name")}</th>
                <th>{t("credentials.type")}</th>
                <th>{t("nodes.username")}</th>
                <th>Updated</th>
                <th>{t("app.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {(credentials ?? []).length === 0 ? (
                <tr><td colSpan={5}><EmptyState message={t("app.noData")} /></td></tr>
              ) : (
                (credentials ?? []).map((c) => (
                  <tr key={c.id}>
                    <td>{c.name}</td>
                    <td>{c.type}</td>
                    <td>{c.username || "—"}</td>
                    <td className="text-sm text-muted">{formatDate(c.updated_at)}</td>
                    <td>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => testMutation.mutate(c.id)} aria-label={t("app.test")}>
                          <Activity size={16} />
                        </Button>
                        {isAdmin && (
                          <>
                            <Button variant="ghost" size="icon" onClick={() => openEdit(c)} aria-label={t("app.edit")}>
                              <Pencil size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={() => setDeleteId(c.id)} aria-label={t("app.delete")}>
                              <Trash2 size={16} />
                            </Button>
                          </>
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

      <Modal open={formOpen} onOpenChange={setFormOpen} title={editing?.id ? t("credentials.edit") : t("credentials.add")}>
        {editing && (
          <>
            <FormGroup label={t("app.name")}>
              <Input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
            </FormGroup>
            <FormGroup label={t("credentials.type")}>
              <Select value={editing.type} onChange={(e) => setEditing({ ...editing, type: e.target.value })}>
                <option value="password">password</option>
                <option value="token">token</option>
              </Select>
            </FormGroup>
            <FormGroup label={t("nodes.username")}>
              <Input value={editing.username} onChange={(e) => setEditing({ ...editing, username: e.target.value })} />
            </FormGroup>
            <FormGroup label={t("credentials.secret")}>
              <Input
                type="password"
                value={editing.secret}
                onChange={(e) => setEditing({ ...editing, secret: e.target.value })}
                placeholder={editing.id ? "(unchanged if empty)" : ""}
              />
            </FormGroup>
            <div className="dialog-actions">
              <Button variant="secondary" onClick={() => setFormOpen(false)}>{t("app.cancel")}</Button>
              <Button
                onClick={() => saveMutation.mutate(editing)}
                disabled={!editing.name || (!editing.id && !editing.secret) || saveMutation.isPending}
              >
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
        description={t("credentials.deleteConfirm", { name: credentials?.find((c) => c.id === deleteId)?.name ?? "" })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteId && deleteMutation.mutate(deleteId)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
