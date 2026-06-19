export type Role = "admin" | "operator" | "viewer";

export interface User {
  id: string;
  username: string;
  display_name?: string;
  email?: string;
  role: Role;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  last_login_at?: string;
}

export interface AuthResponse {
  user: User;
  csrf_token: string;
}

export interface Problem {
  type?: string;
  title: string;
  status: number;
  detail?: string;
}

export interface Node {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  credential_id?: string;
  key_id?: string;
  labels?: Record<string, string>;
  enabled: boolean;
  host_key_status: string;
  host_key_fingerprint?: string;
  last_ping_at?: string;
  last_ping_status?: string;
  created_at: string;
  updated_at: string;
}

export interface Credential {
  id: string;
  name: string;
  type: string;
  username?: string;
  created_at: string;
  updated_at: string;
}

export interface SSHKey {
  id: string;
  name: string;
  fingerprint: string;
  public_key: string;
  comment?: string;
  created_at: string;
  updated_at: string;
}

export interface FileEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  mode: string;
  mod_time: string;
}

export interface Transfer {
  id: string;
  node_id: string;
  source_path: string;
  dest_path: string;
  direction: string;
  status: "pending" | "running" | "paused" | "completed" | "failed" | "cancelled";
  bytes_total: number;
  bytes_done: number;
  error?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface ProtocolSettings {
  sftp_enabled: boolean;
  webdav_enabled: boolean;
  smb_enabled: boolean;
}

export interface ProtocolStatus {
  name: string;
  enabled: boolean;
  running: boolean;
  port?: number;
  path?: string;
  message?: string;
}

export interface ProtocolsOverview {
  protocols: ProtocolStatus[];
}

export interface Settings {
  site_name: string;
  session_timeout_min: number;
  max_upload_size_mb: number;
  rate_limit_per_minute: number;
  require_reauth: boolean;
  allow_registration: boolean;
  default_node_port: number;
  backup_retention_days: number;
  protocols?: ProtocolSettings;
}

export interface Dashboard {
  node_count: number;
  active_transfers: number;
  total_users: number;
  storage_used_mb: number;
  recent_errors: number;
}

export interface ValidationResult {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
}

export interface ConfigChange {
  resource: string;
  action: string;
  detail: string;
}

export interface ConfigPreview {
  changes: ConfigChange[];
}

export interface BackupResult {
  path: string;
  size: number;
  created_at: string;
}

export interface PingResult {
  node_id: string;
  reachable: boolean;
  latency_ms: number;
  message?: string;
}

export interface TestResult {
  success: boolean;
  message?: string;
}

export interface UserSSHKey {
  id: string;
  user_id: string;
  name: string;
  fingerprint: string;
  public_key: string;
  created_at: string;
}
