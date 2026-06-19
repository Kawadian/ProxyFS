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
  ClipboardPaste,
  X,
} from "lucide-react";
import { nodesApi, fsApi, uploadFile } from "@/api/endpoints";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input } from "@/components/ui/Input";
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
  const [clipboard, setClipboard] = useState<{ entry: FileEntry; nodeId: string; nodeName: string; path: string; mode: "copy" | "move" } | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  const { data: nodes, isLoading: nodesLoading } = useQuery({
    queryKey: ["nodes"],
    queryFn: nodesApi.list,
  });

  const { data: files, isLoading } = useQuery({
    queryKey: ["fs", nodeId, currentPath],
    queryFn: () => fsApi.list(nodeId, currentPath),
    enabled: !!nodeId,
  });

  const enabledNodes = (nodes ?? []).filter((n) => n.enabled);
  const currentNode = enabledNodes.find((n) => n.id === nodeId);
  const rootEntries: FileEntry[] = enabledNodes.map((n) => ({
    name: n.name,
    path: n.id,
    is_dir: true,
    size: 0,
    mode: "drwxr-xr-x",
    mod_time: n.last_ping_at ?? n.updated_at,
  }));
  const visibleEntries = nodeId ? (files ?? []) : rootEntries;

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["fs"] });

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

  const pasteMutation = useMutation({
    mutationFn: async () => {
      if (!clipboard || !nodeId) throw new Error("No destination");
      const destPath = currentPath ? `${currentPath}/${clipboard.entry.name}` : clipboard.entry.name;
      await fsApi.copyMove({
        source_node_id: clipboard.nodeId,
        source_path: clipboard.path,
        dest_node_id: nodeId,
        dest_path: destPath,
        mode: clipboard.mode,
      });
    },
    onSuccess: () => {
      invalidate();
      setClipboard(null);
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  const pathParts = currentPath ? currentPath.split("/").filter(Boolean) : [];

  const navigateTo = (index: number) => {
    if (index < -1) {
      setNodeId("");
      setCurrentPath("");
    } else if (index < 0) {
      setCurrentPath("");
    } else {
      setCurrentPath(pathParts.slice(0, index + 1).join("/"));
    }
    setSelected(null);
  };

  const openEntry = (entry: FileEntry) => {
    if (entry.is_dir) {
      if (!nodeId) {
        setNodeId(entry.path);
        setCurrentPath("");
        setSelected(null);
        return;
      }
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

  const copySelection = (mode: "copy" | "move") => {
    if (!selected || !nodeId) return;
    setClipboard({
      entry: selected,
      nodeId,
      nodeName: currentNode?.name ?? nodeId,
      path: selected.path,
      mode,
    });
  };

  const stopRowClick = (e: React.MouseEvent) => {
    e.stopPropagation();
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
          <div>
            <p className="text-sm text-muted">{nodeId ? currentNode?.name : t("files.rootHint")}</p>
            <p className="text-sm">{nodeId ? t("files.nodeRootHint") : t("files.openNodeHint")}</p>
          </div>
          {canWrite && nodeId && (
            <div className="flex gap-2 flex-wrap" style={{ marginTop: "auto" }}>
              {selected && (
                <>
                  <Button variant="secondary" size="sm" onClick={() => copySelection("copy")}>
                    <Copy size={16} /> {t("app.copy")}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => copySelection("move")}>
                    <Move size={16} /> {t("app.move")}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => { setInputValue(selected.name); setRenameOpen(true); }}>
                    <Pencil size={16} /> {t("app.rename")}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setDeleteOpen(true)}>
                    <Trash2 size={16} /> {t("app.delete")}
                  </Button>
                  {!selected.is_dir && (
                    <Button variant="secondary" size="sm" onClick={handleDownload}>
                      <Download size={16} /> {t("app.download")}
                    </Button>
                  )}
                </>
              )}
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

      {clipboard && (
        <div className="card mb-4 clipboard-banner">
          <div className="card-body flex items-center justify-between gap-4 flex-wrap">
            <div>
              <strong>
                {clipboard.mode === "copy" ? t("files.clipboardCopy", { name: clipboard.entry.name }) : t("files.clipboardMove", { name: clipboard.entry.name })}
              </strong>
              <p className="text-sm text-muted">{clipboard.nodeName} / {clipboard.path}</p>
            </div>
            <div className="flex gap-2">
              <Button
                size="sm"
                onClick={() => pasteMutation.mutate()}
                disabled={!nodeId || pasteMutation.isPending}
                title={!nodeId ? t("files.chooseDestination") : undefined}
              >
                <ClipboardPaste size={16} /> {t("files.pasteHere")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setClipboard(null)}>
                <X size={16} /> {t("app.cancel")}
              </Button>
            </div>
          </div>
        </div>
      )}

      <>
          <nav className="breadcrumb" aria-label="Breadcrumb">
            <button type="button" className="breadcrumb-item" onClick={() => navigateTo(-2)}>
              /
            </button>
            {nodeId && (
              <span className="flex items-center">
                <ChevronRight size={14} className="breadcrumb-sep" />
                <button type="button" className="breadcrumb-item" onClick={() => navigateTo(-1)}>
                  {currentNode?.name ?? nodeId}
                </button>
              </span>
            )}
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
            {isLoading || (!nodeId && nodesLoading) ? (
              <LoadingSpinner />
            ) : visibleEntries.length === 0 ? (
              <EmptyState icon={<Folder size={48} />} message={t("files.empty")} />
            ) : (
              <div className="file-grid" style={{ padding: "0.5rem" }}>
                {visibleEntries.map((entry) => (
                  <div
                    key={entry.path}
                    className={`file-item ${selected?.path === entry.path ? "selected" : ""}`}
                  >
                    <div
                      className="file-item-main"
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
                    </div>
                    {selected?.path === entry.path && (
                      <div className="file-actions">
                        {nodeId && !entry.is_dir && (
                          <Button variant="ghost" size="icon" onClick={(e) => { stopRowClick(e); handleDownload(); }} aria-label={t("app.download")}>
                            <Download size={16} />
                          </Button>
                        )}
                        {canWrite && nodeId && (
                          <>
                            <Button variant="ghost" size="icon" onClick={(e) => { stopRowClick(e); setInputValue(entry.name); setRenameOpen(true); }} aria-label={t("app.rename")}>
                              <Pencil size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={(e) => { stopRowClick(e); copySelection("copy"); }} aria-label={t("app.copy")}>
                              <Copy size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={(e) => { stopRowClick(e); copySelection("move"); }} aria-label={t("app.move")}>
                              <Move size={16} />
                            </Button>
                            <Button variant="ghost" size="icon" onClick={(e) => { stopRowClick(e); setDeleteOpen(true); }} aria-label={t("app.delete")}>
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
