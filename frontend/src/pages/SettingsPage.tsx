import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { settingsApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { LoadingSpinner } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import type { Settings } from "@/api/types";

export function SettingsPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<Settings | null>(null);

  const { data: settings, isLoading } = useQuery({
    queryKey: ["settings"],
    queryFn: settingsApi.get,
  });

  useEffect(() => {
    if (settings) setForm(settings);
  }, [settings]);

  const saveMutation = useMutation({
    mutationFn: () => settingsApi.patch(form!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      toast(t("settings.saved"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (isLoading || !form) return <LoadingSpinner />;

  const update = (key: keyof Settings, value: string | number | boolean) => {
    setForm((prev) => prev ? { ...prev, [key]: value } : prev);
  };

  return (
    <div>
      <h1 className="page-title">{t("settings.title")}</h1>

      <div className="card" style={{ maxWidth: 640 }}>
        <div className="card-body">
          <FormGroup label={t("settings.siteName")}>
            <Input value={form.site_name} onChange={(e) => update("site_name", e.target.value)} />
          </FormGroup>
          <div className="form-row">
            <FormGroup label={t("settings.sessionTimeout")}>
              <Input type="number" value={form.session_timeout_min} onChange={(e) => update("session_timeout_min", parseInt(e.target.value, 10))} />
            </FormGroup>
            <FormGroup label={t("settings.maxUpload")}>
              <Input type="number" value={form.max_upload_size_mb} onChange={(e) => update("max_upload_size_mb", parseInt(e.target.value, 10))} />
            </FormGroup>
          </div>
          <div className="form-row">
            <FormGroup label={t("settings.rateLimit")}>
              <Input type="number" value={form.rate_limit_per_minute} onChange={(e) => update("rate_limit_per_minute", parseInt(e.target.value, 10))} />
            </FormGroup>
            <FormGroup label={t("settings.defaultPort")}>
              <Input type="number" value={form.default_node_port} onChange={(e) => update("default_node_port", parseInt(e.target.value, 10))} />
            </FormGroup>
          </div>
          <FormGroup label={t("settings.backupRetention")}>
            <Input type="number" value={form.backup_retention_days} onChange={(e) => update("backup_retention_days", parseInt(e.target.value, 10))} />
          </FormGroup>
          <div className="form-row">
            <FormGroup label={t("settings.requireReauth")}>
              <Select value={form.require_reauth ? "true" : "false"} onChange={(e) => update("require_reauth", e.target.value === "true")}>
                <option value="true">{t("app.yes")}</option>
                <option value="false">{t("app.no")}</option>
              </Select>
            </FormGroup>
            <FormGroup label={t("settings.allowRegistration")}>
              <Select value={form.allow_registration ? "true" : "false"} onChange={(e) => update("allow_registration", e.target.value === "true")}>
                <option value="true">{t("app.yes")}</option>
                <option value="false">{t("app.no")}</option>
              </Select>
            </FormGroup>
          </div>
          <Button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending} className="mt-4">
            {t("app.save")}
          </Button>
        </div>
      </div>
    </div>
  );
}
