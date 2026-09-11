import type { Diagnostic } from "./api";
export interface Limits {
  timeout_seconds: number;
  max_rows: number;
  max_bytes: number;
  concurrency: number;
}
export interface Probe {
  error?: Diagnostic;
  connected: boolean;
  protection: string;
  permission_status: string;
  evidence: string[];
  server_version?: string;
  checked_at: string;
}
export interface Source {
  id: string;
  name: string;
  kind: string;
  version?: string;
  host?: string;
  port?: number;
  database?: string;
  username?: string;
  password?: string;
  token?: string;
  path?: string;
  tls_mode: string;
  ca_cert?: string;
  options?: Record<string, string>;
  enabled: boolean;
  limits: Limits;
  revision: string;
  query_revision: string;
  capability?: Capability;
  has_secret: boolean;
  auth_mode?: "none" | "password" | "token";
  clear_password?: boolean;
  clear_token?: boolean;
  probe?: Probe;
}
export interface Agent {
  revision: string;
  client_id?: string;
  id: string;
  name: string;
  sources: string[];
  enabled: boolean;
  expires_at: string;
  created_at: string;
  auth_type: string;
  revoked_at?: string;
  activity?: { last_call?: string; last_success?: string; error_code?: string };
}
export interface Capability {
  kind: string;
  name: string;
  family: string;
  tool: string;
  port: number;
  protection: string;
  parameters: boolean;
  pagination: string;
  example: Record<string, unknown>;
  limitations: string[];
  verified_versions: string[];
}
export interface Audit {
  request_id: string;
  native_code?: string;
  preview: boolean;
  id: number;
  at: string;
  agent_id: string;
  source_id: string;
  operation: string;
  fingerprint: string;
  elapsed_ms: number;
  rows: number;
  error_code?: string;
}
export interface QueryResult {
  request_id?: string;
  format: string;
  data: unknown[];
  columns?: { name: string; type: string }[];
  row_count: number;
  elapsed_ms: number;
  truncated: boolean;
  next_cursor?: string;
}
export interface Session {
  initialized: boolean;
  authenticated: boolean;
  csrf: string;
}
export interface Settings {
  version: string;
  public_url: string;
  mcp_url: string;
  database_directory: string;
  audit_retention_days: number;
  global_concurrency: number;
  agent_concurrency: number;
  oauth_access_token_minutes: number;
  oauth_refresh_token_days: number;
}

export interface OAuthClientConfig {
  client_id: string;
  client_name: string;
  redirect_uris: string[];
  token_endpoint_auth_method: string;
  enabled: boolean;
  revision: string;
  created_at: string;
  metadata_document: boolean;
}
