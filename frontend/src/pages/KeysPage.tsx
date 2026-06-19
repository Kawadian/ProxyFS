import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Plus, Trash2, Download, RefreshCw } from "lucide-react";
import { keysApi } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Textarea } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, EmptyState, formatDate } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";

export function KeysPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [uploadOpen, setUploadOpen] = useState(false);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [comment, setComment] = useState("");

  const { data: keys, isLoading } = useQuery({
    queryKey: ["keys"],
    queryFn: keysApi.list,
  });

  const uploadMutation = useMutation({
    mutationFn: () => keysApi.upload({ name, private_key: privateKey, comment: comment || undefined }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
      setUploadOpen(false);
      resetForm();
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const generateMutation = useMutation({
    mutationFn: () => keysApi.generate({ name, comment: comment || undefined }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
      setGenerateOpen(false);
      resetForm();
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => keysApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
      setDeleteId(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const downloadMutation = useMutation({
    mutationFn: (id: string) => keysApi.downloadPrivate(id),
    onSuccess: (pem, id) => {
      const key = keys?.find((k) => k.id === id);
      const blob = new Blob([pem], { type: "application/x-pem-file" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${key?.name ?? "key"}.pem`;
      a.click();
      URL.revokeObjectURL(url);
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const rotateMutation = useMutation({
    mutationFn: (id: string) => keysApi.rotate(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["keys"] });
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const resetForm = () => {
    setName("");
    setPrivateKey("");
    setComment("");
  };

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="page-title" style={{ margin: 0 }}>{t("keys.title")}</h1>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => { resetForm(); setUploadOpen(true); }}>
            <Plus size={16} /> {t("keys.add")}
          </Button>
          <Button onClick={() => { resetForm(); setGenerateOpen(true); }}>
            <Plus size={16} /> {t("keys.generate")}
          </Button>
        </div>
      </div>

      <div className="card">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{t("app.name")}</th>
                <th>{t("keys.fingerprint")}</th>
                <th>Created</th>
                <th>{t("app.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {(keys ?? []).length === 0 ? (
                <tr><td colSpan={4}><EmptyState message={t("app.noData")} /></td></tr>
              ) : (
                (keys ?? []).map((k) => (
                  <tr key={k.id}>
                    <td>{k.name}</td>
                    <td className="text-mono truncate" style={{ maxWidth: 280 }}>{k.fingerprint}</td>
                    <td className="text-sm text-muted">{formatDate(k.created_at)}</td>
                    <td>
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => downloadMutation.mutate(k.id)} aria-label={t("keys.downloadPrivate")}>
                          <Download size={16} />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => rotateMutation.mutate(k.id)} aria-label={t("keys.rotate")}>
                          <RefreshCw size={16} />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => setDeleteId(k.id)} aria-label={t("app.delete")}>
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

      <Modal open={uploadOpen} onOpenChange={setUploadOpen} title={t("keys.add")} size="lg">
        <FormGroup label={t("app.name")}>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </FormGroup>
        <FormGroup label={t("keys.privateKey")}>
          <Textarea value={privateKey} onChange={(e) => setPrivateKey(e.target.value)} rows={8} />
        </FormGroup>
        <FormGroup label="Comment">
          <Input value={comment} onChange={(e) => setComment(e.target.value)} />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setUploadOpen(false)}>{t("app.cancel")}</Button>
          <Button onClick={() => uploadMutation.mutate()} disabled={!name || !privateKey || uploadMutation.isPending}>
            {t("app.upload")}
          </Button>
        </div>
      </Modal>

      <Modal open={generateOpen} onOpenChange={setGenerateOpen} title={t("keys.generate")}>
        <FormGroup label={t("app.name")}>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </FormGroup>
        <FormGroup label="Comment">
          <Input value={comment} onChange={(e) => setComment(e.target.value)} />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setGenerateOpen(false)}>{t("app.cancel")}</Button>
          <Button onClick={() => generateMutation.mutate()} disabled={!name || generateMutation.isPending}>
            {t("keys.generate")}
          </Button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={() => setDeleteId(null)}
        title={t("app.delete")}
        description={t("keys.deleteConfirm", { name: keys?.find((k) => k.id === deleteId)?.name ?? "" })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteId && deleteMutation.mutate(deleteId)}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
