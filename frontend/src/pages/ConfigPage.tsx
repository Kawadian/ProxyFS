import { useState, useRef } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Download, Upload, CheckCircle, Eye, AlertTriangle } from "lucide-react";
import { configApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { Textarea } from "@/components/ui/Input";
import { ConfirmDialog } from "@/components/ui/Modal";
import { StatusBadge } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import type { ConfigChange, ValidationResult } from "@/api/types";

export function ConfigPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const fileRef = useRef<HTMLInputElement>(null);

  const [yaml, setYaml] = useState("");
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [changes, setChanges] = useState<ConfigChange[] | null>(null);
  const [applyOpen, setApplyOpen] = useState(false);

  const exportMutation = useMutation({
    mutationFn: configApi.export,
    onSuccess: (data) => {
      setYaml(data);
      const blob = new Blob([data], { type: "application/x-yaml" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "lxcfh-config.yaml";
      a.click();
      URL.revokeObjectURL(url);
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const validateMutation = useMutation({
    mutationFn: () => configApi.validate(yaml),
    onSuccess: (result) => {
      setValidation(result);
      toast(result.valid ? t("config.valid") : t("config.invalid"), undefined, result.valid ? "success" : "error");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const previewMutation = useMutation({
    mutationFn: () => configApi.preview(yaml),
    onSuccess: (result) => {
      setChanges(result.changes);
      if (result.changes.length === 0) {
        toast(t("config.noChanges"), undefined, "info");
      }
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const applyMutation = useMutation({
    mutationFn: () => configApi.apply(yaml),
    onSuccess: () => {
      setApplyOpen(false);
      setChanges(null);
      setValidation(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const handleImport = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      setYaml(reader.result as string);
      setValidation(null);
      setChanges(null);
    };
    reader.readAsText(file);
    if (fileRef.current) fileRef.current.value = "";
  };

  return (
    <div>
      <h1 className="page-title">{t("config.title")}</h1>

      <div className="flex gap-2 mb-4 flex-wrap">
        <Button variant="secondary" onClick={() => exportMutation.mutate()} disabled={exportMutation.isPending}>
          <Download size={16} /> {t("config.export")}
        </Button>
        <Button variant="secondary" onClick={() => fileRef.current?.click()}>
          <Upload size={16} /> {t("config.import")}
        </Button>
        <input ref={fileRef} type="file" accept=".yaml,.yml" hidden onChange={handleImport} />
        <Button variant="secondary" onClick={() => validateMutation.mutate()} disabled={!yaml || validateMutation.isPending}>
          <CheckCircle size={16} /> {t("config.validate")}
        </Button>
        <Button variant="secondary" onClick={() => previewMutation.mutate()} disabled={!yaml || previewMutation.isPending}>
          <Eye size={16} /> {t("config.preview")}
        </Button>
        <Button variant="danger" onClick={() => setApplyOpen(true)} disabled={!yaml}>
          <AlertTriangle size={16} /> {t("config.apply")}
        </Button>
      </div>

      {validation && (
        <div className="card mb-4">
          <div className="card-body">
            <div className="flex items-center gap-2 mb-2">
              <StatusBadge status={validation.valid ? "completed" : "failed"} />
              <span>{validation.valid ? t("config.valid") : t("config.invalid")}</span>
            </div>
            {validation.errors?.map((e, i) => (
              <p key={i} className="form-error">{e}</p>
            ))}
            {validation.warnings?.map((w, i) => (
              <p key={i} className="text-sm text-muted">{w}</p>
            ))}
          </div>
        </div>
      )}

      {changes && changes.length > 0 && (
        <div className="card mb-4">
          <div className="card-header">
            <h2 className="card-title">{t("config.changes")}</h2>
          </div>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Resource</th>
                  <th>Action</th>
                  <th>Detail</th>
                </tr>
              </thead>
              <tbody>
                {changes.map((c, i) => (
                  <tr key={i}>
                    <td>{c.resource}</td>
                    <td><StatusBadge status={c.action} /></td>
                    <td>{c.detail}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div className="card">
        <div className="card-body">
          <Textarea
            value={yaml}
            onChange={(e) => { setYaml(e.target.value); setValidation(null); setChanges(null); }}
            rows={24}
            placeholder="# YAML configuration"
            spellCheck={false}
          />
        </div>
      </div>

      <ConfirmDialog
        open={applyOpen}
        onOpenChange={setApplyOpen}
        title={t("config.apply")}
        description={`${t("config.applyConfirm")}\n\n${t("config.applyWarning")}`}
        confirmLabel={t("config.apply")}
        variant="danger"
        onConfirm={() => applyMutation.mutate()}
        loading={applyMutation.isPending}
      />
    </div>
  );
}
