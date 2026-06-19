import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";
import { usersApi, meApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Textarea } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, EmptyState, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import type { UserSSHKey } from "@/api/types";

interface UserSSHKeysPanelProps {
  username: string;
  userId?: string;
  selfService?: boolean;
}

export function UserSSHKeysPanel({ userId, username, selfService = false }: UserSSHKeysPanelProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [addOpen, setAddOpen] = useState(false);
  const [deleteKey, setDeleteKey] = useState<UserSSHKey | null>(null);
  const [name, setName] = useState("");
  const [publicKey, setPublicKey] = useState("");

  const queryKey = selfService ? ["me", "ssh-keys"] : ["users", userId, "ssh-keys"];

  const { data: keys, isLoading } = useQuery({
    queryKey,
    queryFn: () =>
      selfService ? meApi.listSSHKeys() : usersApi.listSSHKeys(userId!),
    enabled: selfService || !!userId,
  });

  const addMutation = useMutation({
    mutationFn: () =>
      selfService
        ? meApi.addSSHKey({ name, public_key: publicKey })
        : usersApi.addSSHKey(userId!, { name, public_key: publicKey }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setAddOpen(false);
      setName("");
      setPublicKey("");
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (keyId: string) =>
      selfService ? meApi.deleteSSHKey(keyId) : usersApi.deleteSSHKey(userId!, keyId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setDeleteKey(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <p className="text-sm text-muted" style={{ marginBottom: "1rem" }}>
        {t("users.sshKeysHint", { username })}
      </p>

      <div style={{ marginBottom: "1rem" }}>
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus size={16} /> {t("users.addSSHKey")}
        </Button>
      </div>

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>{t("app.name")}</th>
              <th>{t("keys.fingerprint")}</th>
              <th>{t("users.addedAt")}</th>
              <th>{t("app.actions")}</th>
            </tr>
          </thead>
          <tbody>
            {(keys ?? []).length === 0 ? (
              <tr>
                <td colSpan={4}>
                  <EmptyState message={t("users.noSSHKeys")} />
                </td>
              </tr>
            ) : (
              (keys ?? []).map((k) => (
                <tr key={k.id}>
                  <td>{k.name}</td>
                  <td className="text-sm font-mono">{k.fingerprint}</td>
                  <td className="text-sm text-muted">{formatDate(k.created_at)}</td>
                  <td>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setDeleteKey(k)}
                      aria-label={t("app.delete")}
                    >
                      <Trash2 size={16} />
                    </Button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <Modal open={addOpen} onOpenChange={setAddOpen} title={t("users.addSSHKey")} size="lg">
        <FormGroup label={t("app.name")}>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="laptop" />
        </FormGroup>
        <FormGroup label={t("keys.publicKey")}>
          <Textarea
            value={publicKey}
            onChange={(e) => setPublicKey(e.target.value)}
            rows={6}
            placeholder="ssh-ed25519 AAAA... user@host"
          />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setAddOpen(false)}>
            {t("app.cancel")}
          </Button>
          <Button
            onClick={() => addMutation.mutate()}
            disabled={!name || !publicKey.trim() || addMutation.isPending}
          >
            {t("app.save")}
          </Button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteKey}
        onOpenChange={() => setDeleteKey(null)}
        title={t("app.delete")}
        description={t("users.deleteSSHKeyConfirm", { name: deleteKey?.name ?? "" })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteKey && deleteMutation.mutate(deleteKey.id)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
