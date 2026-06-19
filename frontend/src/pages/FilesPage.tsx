import { useState, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  Folder,
  File,
  ChevronRight,
  Upload,
  FolderPlus,
  Download,
  Pencil,
  Trash2,
  Copy,
  Move,
} from "lucide-react";
import { nodesApi, fsApi, transfersApi, uploadFile } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { Modal, ConfirmDialog } from "@/components/ui/Modal";
import { LoadingSpinner, EmptyState, formatBytes } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";
import type { FileEntry } from "@/api/types";

export function FilesPage() {
  const { t } = useTranslation();
  const { canWrite } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [nodeId, setNodeId] = useState("");
  const [currentPath, setCurrentPath] = useState("");
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [renameOpen, setRenameOpen] = useState(false);
  const [copyMoveOpen, setCopyMoveOpen] = useState<"copy" | "move" | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  const { data: nodes } = useQuery({
    queryKey: ["nodes"],
    queryFn: nodesApi.list,
  });

  const { data: files, isLoading } = useQuery({
    queryKey: ["fs", nodeId, currentPath],
    queryFn: () => fsApi.list(nodeId, currentPath),
    enabled: !!nodeId,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["fs", nodeId] });

  const mkdirMutation = useMutation({
    mutationFn: (name: string) => {
      const path = currentPath ? `${currentPath}/${name}` : name;
      return fsApi.mkdir(nodeId, path);
    },
    onSuccess: () => { invalidate(); setMkdirOpen(false); toast(t("app.success"), undefined, "success"); },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const renameMutation = useMutation({
    mutationFn: (newName: string) => {
      if (!selected) throw new Error("No selection");
      const dir = currentPath ? `${currentPath}/` : "";
      const from = `${dir}${selected.name}`;
      const to = `${dir}${newName}`;
      return fsApi.rename(nodeId, from, to);
    },
    onSuccess: () => { invalidate(); setRenameOpen(false); toast(t("app.success"), undefined, "success"); },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const deleteMutation = useMutation({
    mutationFn: () => {
      if (!selected) throw new Error("No selection");
      const path = currentPath ? `${currentPath}/${selected.name}` : selected.name;
      return fsApi.delete(nodeId, path);
    },
    onSuccess: () => { invalidate(); setDeleteOpen(false); setSelected(null); toast(t("app.success"), undefined, "success"); },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const transferMutation = useMutation({
    mutationFn: async ({ destPath, mode }: { destPath: string; mode: "copy" | "move" }) => {
      if (!selected) throw new Error("No selection");
      const source = currentPath ? `${currentPath}/${selected.name}` : selected.name;
      await transfersApi.create({
        node_id: nodeId,
        source_path: source,
        dest_path: destPath,
        direction: mode,
      });
      if (mode === "move") {
        await fsApi.delete(nodeId, source);
      }
    },
    onSuccess: () => {
      invalidate();
      setCopyMoveOpen(null);
      queryClient.invalidateQueries({ queryKey: ["transfers"] });
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const pathParts = currentPath ? currentPath.split("/").filter(Boolean) : [];

  const navigateTo = (index: number) => {
    if (index < 0) {
      setCurrentPath("");
    } else {
      setCurrentPath(pathParts.slice(0, index + 1).join("/"));
    }
    setSelected(null);
  };

  const openEntry = (entry: FileEntry) => {
    if (entry.is_dir) {
      setCurrentPath(entry.path);
      setSelected(null);
    } else {
      setSelected(entry);
    }
  };

  const handleDownload = () => {
    if (!selected || selected.is_dir) return;
    const url = fsApi.downloadUrl(nodeId, selected.path);
    const a = document.createElement("a");
    a.href = url;
    a.download = selected.name;
    a.click();
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !nodeId) return;
    const path = currentPath ? `${currentPath}/${file.name}` : file.name;
    setUploadProgress(0);
    try {
      await uploadFile(nodeId, path, file, setUploadProgress);
      invalidate();
      toast(t("app.success"), undefined, "success");
    } catch (err) {
      toast(t("app.error"), err instanceof Error ? err.message : "", "error");
    } finally {
      setUploadProgress(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  return (
    <div>
      <h1 className="page-title">{t("files.title")}</h1>

      <div className="card mb-4">
        <div className="card-body flex items-center gap-4 flex-wrap">
          <FormGroup label={t("files.selectNode")}>
            <Select
              value={nodeId}
              onChange={(e) => { setNodeId(e.target.value); setCurrentPath(""); setSelected(null); }}
            >
              <option value="">{t("files.selectNode")}</option>
              {(nodes ?? []).filter((n) => n.enabled).map((n) => (
                <option key={n.id} value={n.id}>{n.name}</option>
              ))}
            </Select>
          </FormGroup>
          {canWrite && nodeId && (
            <div className="flex gap-2 flex-wrap" style={{ marginTop: "auto" }}>
              <Button variant="secondary" size="sm" onClick={() => { setInputValue(""); setMkdirOpen(true); }}>
                <FolderPlus size={16} /> {t("files.newFolder")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => fileInputRef.current?.click()}>
                <Upload size={16} /> {t("files.uploadFile")}
              </Button>
              <input ref={fileInputRef} type="file" hidden onChange={handleUpload} />
            </div>
          )}
        </div>
        {uploadProgress !== null && (
          <div className="card-body" style={{ paddingTop: 0 }}>
            <p className="text-sm text-muted">{t("files.uploadProgress", { pct: uploadProgress })}</p>
            <div className="progress-bar mt-2">
              <div className="progress-fill" style={{ width: `${uploadProgress}%` }} />
            </div>
          </div>
        )}
      </div>

      {nodeId && (
        <>
          <nav className="breadcrumb" aria-label="Breadcrumb">
            <button type="button" className="breadcrumb-item" onClick={() => navigateTo(-1)}>
              /
            </button>
            {pathParts.map((part, i) => (
              <span key={i} className="flex items-center">
                <ChevronRight size={14} className="breadcrumb-sep" />
                <button type="button" className="breadcrumb-item" onClick={() => navigateTo(i)}>
                  {part}
                </button>
              </span>
            ))}
          </nav>

          <div className="card">
            {isLoading ? (
              <LoadingSpinner />
            ) : (files ?? []).length === 0 ? (
              <EmptyState icon={<Folder size={48} />} message={t("files.empty")} />
            ) : (
              <div className="file-grid" style={{ padding: "0.5rem" }}>
                {(files ?? []).map((entry) => (
                  <div
                    key={entry.path}
                    className={`file-item ${selected?.path === entry.path ? "selected" : ""}`}
                    onClick={() => openEntry(entry)}
                    onDoubleClick={() => entry.is_dir && openEntry(entry)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => e.key === "Enter" && openEntry(entry)}
                  >
                    {entry.is_dir ? (
                      <Folder className="file-icon" size={20} />
                    ) : (
                      <File className="file-icon" size={20} />
                    )}
                    <div className="file-meta">
                      <div className="file-name">{entry.name}</div>
                      <div className="file-size">
                        {entry.is_dir ? "—" : formatBytes(entry.size)} · {entry.mod_time}
                      </div>
                    </div>
                    {selected?.path === entry.path && (
                      <div className="file-actions" onClick={(e) => e.stopPropagation()}>
                        {!entry.is_dir && (
                          <Button variant="ghost" size="icon" onClick={handleDownload} aria-label={t("app.download")}>
                            <Download size={16} />
                          </Button>
                        )}
                        {canWrite && (
                          <>
                            <Button variant="ghost" size="icon" onClick={() => { setInputValue(entry.name); setRenameOpen(true); }} aria-label={t("app.rename")}>
                              <Pencil size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={() => { setInputValue(""); setCopyMoveOpen("copy"); }} aria-label={t("app.copy")}>
                              <Copy size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={() => { setInputValue(""); setCopyMoveOpen("move"); }} aria-label={t("app.move")}>
                              <Move size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={() => setDeleteOpen(true)} aria-label={t("app.delete")}>
                              <Trash2 size={16} />
                            </Button>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}

      <Modal open={mkdirOpen} onOpenChange={setMkdirOpen} title={t("files.newFolder")}>
        <FormGroup label={t("files.folderName")}>
          <Input value={inputValue} onChange={(e) => setInputValue(e.target.value)} autoFocus />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setMkdirOpen(false)}>{t("app.cancel")}</Button>
          <Button onClick={() => mkdirMutation.mutate(inputValue)} disabled={!inputValue || mkdirMutation.isPending}>
            {t("app.create")}
          </Button>
        </div>
      </Modal>

      <Modal open={renameOpen} onOpenChange={setRenameOpen} title={t("app.rename")}>
        <FormGroup label={t("files.renameTo")}>
          <Input value={inputValue} onChange={(e) => setInputValue(e.target.value)} autoFocus />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setRenameOpen(false)}>{t("app.cancel")}</Button>
          <Button onClick={() => renameMutation.mutate(inputValue)} disabled={!inputValue || renameMutation.isPending}>
            {t("app.save")}
          </Button>
        </div>
      </Modal>

      <Modal
        open={!!copyMoveOpen}
        onOpenChange={() => setCopyMoveOpen(null)}
        title={copyMoveOpen === "copy" ? t("app.copy") : t("app.move")}
      >
        <FormGroup label={copyMoveOpen === "copy" ? t("files.copyTo") : t("files.moveTo")}>
          <Input value={inputValue} onChange={(e) => setInputValue(e.target.value)} placeholder="/path/to/destination" autoFocus />
        </FormGroup>
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setCopyMoveOpen(null)}>{t("app.cancel")}</Button>
          <Button
            onClick={() => copyMoveOpen && transferMutation.mutate({ destPath: inputValue, mode: copyMoveOpen })}
            disabled={!inputValue || transferMutation.isPending}
          >
            {t("app.confirm")}
          </Button>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("app.delete")}
        description={t("files.deleteConfirm", { name: selected?.name ?? "" })}
        confirmLabel={t("app.delete")}
        variant="danger"
        onConfirm={() => deleteMutation.mutate()}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}
