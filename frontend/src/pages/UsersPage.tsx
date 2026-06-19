import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Plus, Pencil, Trash2, KeyRound } from "lucide-react";
import { usersApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, StatusBadge, EmptyState, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { UserSSHKeysPanel } from "@/components/users/UserSSHKeysPanel";
import type { User } from "@/api/types";

export function UsersPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [sshKeysUser, setSshKeysUser] = useState<User | null>(null);
  const [editing, setEditing] = useState<{
    id?: string;
    username: string;
    password: string;
    display_name: string;
    email: string;
    role: string;
    enabled: boolean;
  } | null>(null);
  const [newPassword, setNewPassword] = useState("");

  const { data: users, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: usersApi.list,
  });

  const saveMutation = useMutation({
    mutationFn: (data: NonNullable<typeof editing>) =>
      data.id
        ? usersApi.update(data.id, {
            display_name: data.display_name,
            email: data.email,
            role: data.role as User["role"],
            enabled: data.enabled,
          })
        : usersApi.create({
            username: data.username,
            password: data.password,
            display_name: data.display_name || undefined,
            email: data.email || undefined,
            role: data.role,
          }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setFormOpen(false);
      setEditing(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const passwordMutation = useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) =>
      usersApi.changePassword(id, password),
    onSuccess: () => {
      setPasswordOpen(null);
      setNewPassword("");
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => usersApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setDeleteId(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const openCreate = () => {
    setEditing({ username: "", password: "", display_name: "", email: "", role: "viewer", enabled: true });
    setFormOpen(true);
  };

  const openEdit = (u: User) => {
    setEditing({
      id: u.id,
      username: u.username,
      password: "",
      display_name: u.display_name ?? "",
      email: u.email ?? "",
      role: u.role,
      enabled: u.enabled,
    });
    setFormOpen(true);
  };

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="page-title" style={{ margin: 0 }}>{t("users.title")}</h1>
        <Button onClick={openCreate}>
          <Plus size={16} /> {t("users.add")}
        </Button>
      </div>

      <div className="card">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("login.username")}</th>
                <th>{t("app.name")}</th>
                <th>{t("users.role")}</th>
                <th>{t("app.status")}</th>
                <th>Last login</th>
                <th>{t("app.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {(users ?? []).length === 0 ? (
                <tr><td colSpan={6}><EmptyState message={t("app.noData")} /></td></tr>
              ) : (
                (users ?? []).map((u) => (
                  <tr key={u.id}>
                    <td>{u.username}</td>
                    <td>{u.display_name || "—"}</td>
                    <td>{t(`users.roles.${u.role}`)}</td>
                    <td><StatusBadge status={u.enabled ? "enabled" : "disabled"} /></td>
                    <td className="text-sm text-muted">{u.last_login_at ? formatDate(u.last_login_at) : "—"}</td>
                    <td>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => setSshKeysUser(u)} aria-label={t("users.sshKeys")}>
                          <KeyRound size={16} />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => openEdit(u)} aria-label={t("app.edit")}>
                          <Pencil size={16} />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setPasswordOpen(u.id)}>
                          {t("users.changePassword")}
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => setDeleteId(u.id)} aria-label={t("app.delete")}>
                          <Trash2 size={16} />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <Modal open={formOpen} onOpenChange={setFormOpen} title={editing?.id ? t("users.edit") : t("users.add")} size="lg">
        {editing && (
          <>
            {!editing.id && (
              <FormGroup label={t("login.username")}>
                <Input value={editing.username} onChange={(e) => setEditing({ ...editing, username: e.target.value })} />
              </FormGroup>
            )}
            {!editing.id && (
              <FormGroup label={t("users.password")}>
                <Input type="password" value={editing.password} onChange={(e) => setEditing({ ...editing, password: e.target.value })} />
              </FormGroup>
            )}
            <FormGroup label={t("app.name")}>
              <Input value={editing.display_name} onChange={(e) => setEditing({ ...editing, display_name: e.target.value })} />
            </FormGroup>
            <FormGroup label={t("users.email")}>
              <Input type="email" value={editing.email} onChange={(e) => setEditing({ ...editing, email: e.target.value })} />
            </FormGroup>
            <div className="form-row">
              <FormGroup label={t("users.role")}>
                <Select value={editing.role} onChange={(e) => setEditing({ ...editing, role: e.target.value })}>
                  <option value="admin">{t("users.roles.admin")}</option>
                  <option value="operator">{t("users.roles.operator")}</option>
                  <option value="viewer">{t("users.roles.viewer")}</option>
                </Select>
              </FormGroup>
              {editing.id && (
                <FormGroup label={t("app.status")}>
                  <Select
                    value={editing.enabled ? "true" : "false"}
                    onChange={(e) => setEditing({ ...editing, enabled: e.target.value === "true" })}
                  >
                    <option value="true">{t("app.enabled")}</option>
                    <option value="false">{t("app.disabled")}</option>
                  </Select>
                </FormGroup>
              )}
            </div>
            <div className="dialog-actions">
              <Button variant="secondary" onClick={() => setFormOpen(false)}>{t("app.cancel")}</Button>
              <Button onClick={() => saveMutation.mutate(editing)} disabled={saveMutation.isPending}>
                {t("app.save")}
              </Button>
            </div>
          </>
        )}
      </Modal>

      <Modal open={!!passwordOpen} onOpenChange={() => setPasswordOpen(null)} title={t("users.changePassword")}>
        <FormGroup label={t("users.password")}>
          <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setPasswordOpen(null)}>{t("app.cancel")}</Button>
          <Button
            onClick={() => passwordOpen && passwordMutation.mutate({ id: passwordOpen, password: newPassword })}
            disabled={!newPassword || passwordMutation.isPending}
          >
            {t("app.save")}
          </Button>
        </div>
      </Modal>

      <Modal
        open={!!sshKeysUser}
        onOpenChange={() => setSshKeysUser(null)}
        title={t("users.sshKeysFor", { name: sshKeysUser?.username ?? "" })}
        size="lg"
      >
        {sshKeysUser && (
          <UserSSHKeysPanel userId={sshKeysUser.id} username={sshKeysUser.username} />
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={() => setDeleteId(null)}
        title={t("app.delete")}
        description={t("users.deleteConfirm", { name: users?.find((u) => u.id === deleteId)?.username ?? "" })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteId && deleteMutation.mutate(deleteId)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
