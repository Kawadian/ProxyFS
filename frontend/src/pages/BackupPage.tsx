import { useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Database, Upload } from "lucide-react";
import { backupApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/Modal";
import { formatBytes, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import type { BackupResult } from "@/api/types";

export function BackupPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const fileRef = useRef<HTMLInputElement>(null);
  const [lastBackup, setLastBackup] = useState<BackupResult | null>(null);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoreYaml, setRestoreYaml] = useState<string | null>(null);

  const backupMutation = useMutation({
    mutationFn: backupApi.create,
    onSuccess: (result) => {
      setLastBackup(result);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const restoreMutation = useMutation({
    mutationFn: (yaml: string) => backupApi.restore(yaml),
    onSuccess: () => {
      setRestoreOpen(false);
      setRestoreYaml(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      setRestoreYaml(reader.result as string);
      setRestoreOpen(true);
    };
    reader.readAsText(file);
    if (fileRef.current) fileRef.current.value = "";
  };

  return (
    <div>
      <h1 className="page-title">{t("backup.title")}</h1>

      <div className="form-row">
        <div className="card">
          <div className="card-body">
            <h2 className="card-title mb-4">{t("backup.create")}</h2>
            <p className="text-sm text-muted mb-4">{t("backup.createDescription")}</p>
            <Button onClick={() => backupMutation.mutate()} disabled={backupMutation.isPending}>
              <Database size={16} />
              {backupMutation.isPending ? t("backup.creating") : t("backup.create")}
            </Button>
          </div>
        </div>

        <div className="card">
          <div className="card-body">
            <h2 className="card-title mb-4">{t("backup.restore")}</h2>
            <p className="text-sm text-muted mb-4">{t("backup.restoreDescription")}</p>
            <Button variant="secondary" onClick={() => fileRef.current?.click()}>
              <Upload size={16} /> {t("backup.restore")}
            </Button>
            <input ref={fileRef} type="file" accept=".yaml,.yml" hidden onChange={handleFileSelect} />
          </div>
        </div>
      </div>

      {lastBackup && (
        <div className="card mt-4">
          <div className="card-header">
            <h2 className="card-title">{t("backup.lastBackup")}</h2>
          </div>
          <div className="card-body">
            <dl className="form-row">
              <div>
                <dt className="text-sm text-muted">Path</dt>
                <dd className="text-mono">{lastBackup.path}</dd>
              </div>
              <div>
                <dt className="text-sm text-muted">{t("backup.size")}</dt>
                <dd>{formatBytes(lastBackup.size)}</dd>
              </div>
              <div>
                <dt className="text-sm text-muted">{t("backup.created")}</dt>
                <dd>{formatDate(lastBackup.created_at)}</dd>
              </div>
            </dl>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={restoreOpen}
        onOpenChange={setRestoreOpen}
        title={t("backup.restore")}
        description={t("backup.restoreConfirm")}
        confirmLabel={t("backup.restore")}
        variant="danger"
        onConfirm={() => restoreYaml && restoreMutation.mutate(restoreYaml)}
        loading={restoreMutation.isPending}
      />
    </div>
  );
}
