export interface Registration {
  device_id: string;
  hostname: string;
  serial: string;
  model: string;
  firmware: string;
  probe_version: string;
  arch: string;
  kernel: string;
  libc: string;
  boot_id: string;
  capabilities: string[];
}
export interface Session {
  session_id: string;
  registration: Registration;
  started_at: string;
  last_seen_at: string;
  ended_at: string | null;
  end_reason: string;
}
export interface Device {
  device_id: string;
  registration: Registration;
  status: string;
  current_session: Session | null;
  latest_session: Session | null;
  first_seen_at: string | null;
  last_seen_at: string | null;
  last_online_at: string | null;
  last_offline_at: string | null;
  total_sessions: number;
  evicted_sessions: number;
}
export interface TaskSummary {
  task_id: string;
  device_id: string;
  type: string;
  state: string;
  created_at: string;
}
export interface TaskDetail extends TaskSummary {
  command: string;
  cwd: string;
  timeout_seconds: number;
  last_session_id: string;
  dispatch_count: number;
  result: null | {
    status: string;
    exit_code: number;
    stdout: string;
    stderr: string;
    truncated: boolean;
    started_at: string;
    finished_at: string;
  };
}
export interface Transfer {
  task_id: string;
  transfer_id: string;
  committed: boolean;
  released: boolean;
  failed: boolean;
  size: number;
  sha256: string;
}
export interface Asset {
  asset_id: string;
  name: string;
  size: number;
  sha256: string;
  archived: boolean;
  created_at: string;
}
export interface Tool {
  tool_id: string;
  name: string;
  description: string;
  archived: boolean;
  created_at: string;
}
export interface Rules {
  arch: string[];
  libc: string[];
  models?: string[];
  kernels?: string[];
  required_capabilities?: string[];
}
export interface Artifact {
  artifact_id: string;
  asset_id: string;
  platform: string;
  mode: string;
  rules: Rules;
}
export interface ToolVersion {
  tool_id: string;
  version: string;
  artifacts: Artifact[];
  archived: boolean;
  created_at: string;
}
export interface Compatibility {
  artifact: Artifact;
  status: string;
  checks: { field: string; status: string; reason: string }[];
}
export interface Endpoint {
  service: "web" | "ssh" | "telnet";
  host: string;
  port: number;
  address: string;
  state: string;
  url?: string;
}
export interface Maintenance {
  maintenance_id: string;
  device_id: string;
  session_id: string;
  state: string;
  reason: string;
  created_at: string;
  expires_at: string;
  released: boolean;
  reusable_after: string | null;
  connections: number;
  endpoints: Endpoint[];
}
export interface Snapshot {
  devices: Device[];
  tasks: TaskSummary[];
  assets: Asset[];
  tools: Tool[];
  maintenance: Maintenance[];
  maintenanceError?: string;
  fetchedAt: number;
}
export interface Profile {
  schema_version: number;
  server_url: string;
  theme: "Default" | "Light" | "Dark";
  ssh_user: string;
  ssh_executable: string;
  telnet_executable: string;
  ssh_use_putty: boolean;
  telnet_use_putty: boolean;
}
export const defaults: Profile = {
  schema_version: 1,
  server_url: "http://127.0.0.1:8080",
  theme: "Default",
  ssh_user: "root",
  ssh_executable: "",
  telnet_executable: "",
  ssh_use_putty: false,
  telnet_use_putty: false,
};
export const emptySnapshot = (): Snapshot => ({
  devices: [],
  tasks: [],
  assets: [],
  tools: [],
  maintenance: [],
  fetchedAt: 0,
});
