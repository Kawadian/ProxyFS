import { apiFetch, apiFetchRaw, getCsrfToken } from "./client";
import type {
  AuthResponse,
  BackupResult,
  ConfigPreview,
  Credential,
  Dashboard,
  Node,
  PingResult,
  Settings,
  SSHKey,
  TestResult,
  Transfer,
  User,
  UserSSHKey,
  ProtocolsOverview,
  ValidationResult,
  FileEntry,
} from "./types";

export const authApi = {
  setup: (data: { username: string; password: string; display_name?: string }) =>
    apiFetch<AuthResponse>("/auth/setup", { method: "POST", body: JSON.stringify(data) }),

  login: (data: { username: string; password: string }) =>
    apiFetch<AuthResponse>("/auth/login", { method: "POST", body: JSON.stringify(data) }),

  logout: () => apiFetch<void>("/auth/logout", { method: "POST" }),

  me: () => apiFetch<AuthResponse>("/auth/me"),

  reauth: (password: string) =>
    apiFetch<void>("/auth/reauth", {
      method: "POST",
      body: JSON.stringify({ username: "", password }),
    }),
};

export const dashboardApi = {
  get: () => apiFetch<Dashboard>("/dashboard"),
};

export const nodesApi = {
  list: () => apiFetch<Node[]>("/nodes"),
  get: (id: string) => apiFetch<Node>(`/nodes/${id}`),
  create: (node: Partial<Node>) =>
    apiFetch<Node>("/nodes", { method: "POST", body: JSON.stringify(node) }),
  update: (id: string, node: Partial<Node>) =>
    apiFetch<Node>(`/nodes/${id}`, { method: "PATCH", body: JSON.stringify(node) }),
  delete: (id: string) => apiFetch<void>(`/nodes/${id}`, { method: "DELETE" }),
  ping: (id: string) => apiFetch<PingResult>(`/nodes/${id}/ping`, { method: "POST" }),
  test: (id: string) => apiFetch<TestResult>(`/nodes/${id}/test`, { method: "POST" }),
  acceptHostKey: (id: string, fingerprint: string) =>
    apiFetch<Node>(`/nodes/${id}/accept-host-key`, {
      method: "POST",
      body: JSON.stringify({ fingerprint }),
    }),
};

export const credentialsApi = {
  list: () => apiFetch<Credential[]>("/credentials"),
  get: (id: string) => apiFetch<Credential>(`/credentials/${id}`),
  create: (data: { name: string; type: string; username?: string; secret: string }) =>
    apiFetch<Credential>("/credentials", { method: "POST", body: JSON.stringify(data) }),
  update: (id: string, data: { name: string; type: string; username?: string; secret: string }) =>
    apiFetch<Credential>(`/credentials/${id}`, { method: "PATCH", body: JSON.stringify(data) }),
  delete: (id: string) => apiFetch<void>(`/credentials/${id}`, { method: "DELETE" }),
  test: (id: string) => apiFetch<TestResult>(`/credentials/${id}/test`, { method: "POST" }),
};

export const keysApi = {
  list: () => apiFetch<SSHKey[]>("/keys"),
  upload: (data: { name: string; private_key: string; comment?: string }) =>
    apiFetch<SSHKey>("/keys/upload", { method: "POST", body: JSON.stringify(data) }),
  generate: (data: { name: string; comment?: string }) =>
    apiFetch<SSHKey>("/keys/generate", { method: "POST", body: JSON.stringify(data) }),
  delete: (id: string) => apiFetch<void>(`/keys/${id}`, { method: "DELETE" }),
  downloadPrivate: (id: string) =>
    apiFetchRaw(`/keys/${id}/download-private`).then((r) => r.text()),
  rotate: (id: string, name?: string) =>
    apiFetch<SSHKey>(`/keys/${id}/rotate`, {
      method: "POST",
      body: JSON.stringify(name ? { name } : {}),
    }),
};

export const meApi = {
  changePassword: (password: string) =>
    apiFetch<void>("/me/password", {
      method: "PUT",
      body: JSON.stringify({ password }),
    }),
  listSSHKeys: () => apiFetch<UserSSHKey[]>("/me/ssh-keys"),
  addSSHKey: (data: { name: string; public_key: string }) =>
    apiFetch<UserSSHKey>("/me/ssh-keys", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteSSHKey: (keyId: string) =>
    apiFetch<void>(`/me/ssh-keys/${keyId}`, { method: "DELETE" }),
};

export const usersApi = {
  list: () => apiFetch<User[]>("/users"),
  get: (id: string) => apiFetch<User>(`/users/${id}`),
  create: (data: {
    username: string;
    password: string;
    display_name?: string;
    email?: string;
    role?: string;
  }) => apiFetch<User>("/users", { method: "POST", body: JSON.stringify(data) }),
  update: (id: string, data: Partial<User>) =>
    apiFetch<User>(`/users/${id}`, { method: "PATCH", body: JSON.stringify(data) }),
  delete: (id: string) => apiFetch<void>(`/users/${id}`, { method: "DELETE" }),
  changePassword: (id: string, password: string) =>
    apiFetch<void>(`/users/${id}/password`, {
      method: "PUT",
      body: JSON.stringify({ password }),
    }),
  listSSHKeys: (userId: string) => apiFetch<UserSSHKey[]>(`/users/${userId}/ssh-keys`),
  addSSHKey: (userId: string, data: { name: string; public_key: string }) =>
    apiFetch<UserSSHKey>(`/users/${userId}/ssh-keys`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteSSHKey: (userId: string, keyId: string) =>
    apiFetch<void>(`/users/${userId}/ssh-keys/${keyId}`, { method: "DELETE" }),
};

export const fsApi = {
  list: (nodeId: string, path = "") => {
    const params = new URLSearchParams({ node_id: nodeId, path });
    return apiFetch<FileEntry[]>(`/fs/list?${params}`);
  },
  stat: (nodeId: string, path: string) => {
    const params = new URLSearchParams({ node_id: nodeId, path });
    return apiFetch(`/fs/stat?${params}`);
  },
  downloadUrl: (nodeId: string, path: string) => {
    const params = new URLSearchParams({ node_id: nodeId, path });
    return `/api/v1/fs/download?${params}`;
  },
  mkdir: (nodeId: string, path: string) =>
    apiFetch<void>("/fs/mkdir", {
      method: "POST",
      body: JSON.stringify({ node_id: nodeId, path }),
    }),
  rename: (nodeId: string, from: string, to: string) =>
    apiFetch<void>("/fs/rename", {
      method: "POST",
      body: JSON.stringify({ node_id: nodeId, from, to }),
    }),
  delete: (nodeId: string, path: string) => {
    const params = new URLSearchParams({ node_id: nodeId, path });
    return apiFetch<void>(`/fs/delete?${params}`, { method: "DELETE" });
  },
  readText: (nodeId: string, path: string) => {
    const params = new URLSearchParams({ node_id: nodeId, path });
    return apiFetch<{ content: string }>(`/fs/text?${params}`);
  },
  writeText: (nodeId: string, path: string, content: string) =>
    apiFetch<void>("/fs/text", {
      method: "PUT",
      body: JSON.stringify({ node_id: nodeId, path, content }),
    }),
};

export const transfersApi = {
  list: () => apiFetch<Transfer[]>("/transfers"),
  get: (id: string) => apiFetch<Transfer>(`/transfers/${id}`),
  create: (transfer: Partial<Transfer>) =>
    apiFetch<Transfer>("/transfers", { method: "POST", body: JSON.stringify(transfer) }),
  delete: (id: string) => apiFetch<void>(`/transfers/${id}`, { method: "DELETE" }),
  pause: (id: string) => apiFetch<Transfer>(`/transfers/${id}/pause`, { method: "POST" }),
  resume: (id: string) => apiFetch<Transfer>(`/transfers/${id}/resume`, { method: "POST" }),
  cancel: (id: string) => apiFetch<Transfer>(`/transfers/${id}/cancel`, { method: "POST" }),
  retry: (id: string) => apiFetch<Transfer>(`/transfers/${id}/retry`, { method: "POST" }),
};

export const configApi = {
  export: () => apiFetchRaw("/config/export").then((r) => r.text()),
  validate: (yaml: string) =>
    apiFetch<ValidationResult>("/config/validate", {
      method: "POST",
      headers: { "Content-Type": "application/x-yaml" },
      body: yaml,
    }),
  preview: (yaml: string) =>
    apiFetch<ConfigPreview>("/config/preview", {
      method: "POST",
      headers: { "Content-Type": "application/x-yaml" },
      body: yaml,
    }),
  apply: (yaml: string) =>
    apiFetch<void>("/config/apply", {
      method: "POST",
      headers: { "Content-Type": "application/x-yaml" },
      body: yaml,
    }),
};

export const protocolsApi = {
  get: () => apiFetch<ProtocolsOverview>("/protocols"),
  setEnabled: (name: string, enabled: boolean) =>
    apiFetch<ProtocolsOverview>(`/protocols/${name}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
};

export const settingsApi = {
  get: () => apiFetch<Settings>("/settings"),
  patch: (settings: Partial<Settings>) =>
    apiFetch<Settings>("/settings", { method: "PATCH", body: JSON.stringify(settings) }),
};

export const backupApi = {
  create: () => apiFetch<BackupResult>("/backup", { method: "POST" }),
  restore: (yaml: string) =>
    apiFetch<void>("/backup/restore", {
      method: "POST",
      headers: { "Content-Type": "application/x-yaml" },
      body: yaml,
    }),
};

export async function uploadFile(
  nodeId: string,
  path: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<void> {
  const csrf = getCsrfToken();

  const createRes = await fetch("/api/v1/uploads", {
    method: "POST",
    credentials: "include",
    headers: {
      "Upload-Length": String(file.size),
      "Upload-Node-ID": nodeId,
      "Upload-Path": path,
      "Tus-Resumable": "1.0.0",
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
    },
  });

  if (!createRes.ok) {
    throw new Error(`Upload failed: ${createRes.status}`);
  }

  const location = createRes.headers.get("Location");
  if (!location) throw new Error("No upload location returned");

  const chunkSize = 1024 * 1024;
  let offset = 0;

  while (offset < file.size) {
    const chunk = file.slice(offset, offset + chunkSize);
    const patchRes = await fetch(location, {
      method: "PATCH",
      credentials: "include",
      headers: {
        "Upload-Offset": String(offset),
        "Content-Type": "application/offset+octet-stream",
        "Tus-Resumable": "1.0.0",
        ...(csrf ? { "X-CSRF-Token": csrf } : {}),
      },
      body: chunk,
    });

    if (!patchRes.ok) {
      throw new Error(`Upload chunk failed: ${patchRes.status}`);
    }

    const newOffset = parseInt(patchRes.headers.get("Upload-Offset") ?? String(offset + chunk.size), 10);
    offset = newOffset;
    onProgress?.(Math.round((offset / file.size) * 100));
  }
}
