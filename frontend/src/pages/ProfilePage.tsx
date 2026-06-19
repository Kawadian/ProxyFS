import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { meApi } from "@/api/endpoints";
import { useAuth } from "@/hooks/useAuth";
import { UserSSHKeysPanel } from "@/components/users/UserSSHKeysPanel";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { useToast } from "@/components/ui/Toast";

export function ProfilePage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();

  const [passwordOpen, setPasswordOpen] = useState(false);
  const [newPassword, setNewPassword] = useState("");

  const passwordMutation = useMutation({
    mutationFn: (password: string) => meApi.changePassword(password),
    onSuccess: () => {
      setPasswordOpen(false);
      setNewPassword("");
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (!user) return null;

  return (
    <div>
      <h1 className="page-title">{t("profile.title")}</h1>

      <div className="card" style={{ marginBottom: "1.5rem" }}>
        <h2 className="text-lg font-semibold" style={{ marginBottom: "1rem" }}>
          {t("profile.account")}
        </h2>
        <dl className="text-sm" style={{ display: "grid", gridTemplateColumns: "140px 1fr", gap: "0.5rem" }}>
          <dt className="text-muted">{t("login.username")}</dt>
          <dd>{user.username}</dd>
          <dt className="text-muted">{t("app.name")}</dt>
          <dd>{user.display_name || "—"}</dd>
          <dt className="text-muted">{t("users.role")}</dt>
          <dd>{t(`users.roles.${user.role}`)}</dd>
        </dl>
        <div style={{ marginTop: "1rem" }}>
          <Button variant="secondary" size="sm" onClick={() => setPasswordOpen(true)}>
            {t("users.changePassword")}
          </Button>
        </div>
      </div>

      <div className="card">
        <h2 className="text-lg font-semibold" style={{ marginBottom: "1rem" }}>
          {t("users.sshKeys")}
        </h2>
        <p className="text-sm text-muted" style={{ marginBottom: "1rem" }}>
          {t("profile.sshKeysDescription")}
        </p>
        <UserSSHKeysPanel username={user.username} selfService />
      </div>

      <Modal open={passwordOpen} onOpenChange={setPasswordOpen} title={t("users.changePassword")}>
        <FormGroup label={t("users.password")}>
          <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setPasswordOpen(false)}>
            {t("app.cancel")}
          </Button>
          <Button
            onClick={() => passwordMutation.mutate(newPassword)}
            disabled={!newPassword || passwordMutation.isPending}
          >
            {t("app.save")}
          </Button>
        </div>
      </Modal>
    </div>
  );
}
