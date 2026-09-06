import { useEffect, useRef, useState } from "react";
import {
  Router,
  PanelsTopLeft,
  Terminal,
  Folder,
  Package,
  Settings,
  Sun,
  Moon,
  RefreshCw,
  Search,
  Filter,
  Globe,
  ExternalLink,
  Clock,
  ShieldCheck,
  Activity,
  Send,
  Upload,
  Download,
  Plus,
  ChevronRight,
  MoreVertical,
  X,
  Home,
  Info,
  Power,
  Check,
  Monitor,
} from "lucide-react";
import { Connection, type ConnectionState } from "./connection";
import {
  baseUrl,
  describe,
  importMutation,
  leaseBody,
  mutation,
  segment,
  type Mutation,
} from "./api";
import {
  defaults,
  emptySnapshot,
  type Asset,
  type Compatibility,
  type Device,
  type Maintenance,
  type Profile,
  type Session,
  type TaskDetail,
  type Tool,
  type ToolVersion,
  type Transfer,
} from "./models";
import { Badge, Card, FormDialog, stateName } from "./components";
import { platform } from "./platform";
import { useQuery } from "./useQuery";

type Page = "maintenance" | "overview" | "tasks" | "files" | "tools" | "details" | "settings";
const tabs: [Page, string, typeof Home][] = [
  ["overview", "概览", Home],
  ["tasks", "任务 / Exec", Terminal],
  ["files", "文件管理", Folder],
  ["tools", "工具 / 版本", Package],
  ["details", "设备详情", Info],
];
export const stamp = (s: string | null | undefined) =>
  s ? new Date(s).toLocaleString("zh-CN", { hour12: false }) : "—";

export function useWorkbench() {
  const autoConnect = useRef(location.hash === "#connect");
  const [profile, setProfile] = useState<Profile>(defaults),
    [loaded, setLoaded] = useState(false),
    [page, setPage] = useState<Page>(() => {
      const p = location.hash.slice(1);
      return [
        "overview",
        "maintenance",
        "tasks",
        "files",
        "tools",
        "details",
        "settings",
      ].includes(p)
        ? (p as Page)
        : "overview";
    }),
    [selected, setSelected] = useState(""),
    [query, setQuery] = useState(""),
    [onlineOnly, setOnlineOnly] = useState(false),
    [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [now, setNow] = useState(Date.now()),
    [minutes, setMinutes] = useState("240");
  const [state, setState] = useState<ConnectionState>({
    snapshot: emptySnapshot(),
    status: "未连接",
    synchronized: false,
    pending: null,
    busy: false,
    error: "",
  });
  const owner = useRef<Connection | null>(null),
    actionGate = useRef(false),
    actionNotice = useRef(""),
    lastAction = useRef({ label: "", at: 0 }),
    generation = useRef(0);
  const [dialog, setDialog] = useState<{
      title: string;
      fields: {
        key: string;
        label: string;
        value: string;
        multiline?: boolean;
      }[];
      run: (v: Record<string, string>) => Promise<void>;
    } | null>(null),
    [inspect, setInspect] = useState<{ title: string; value: unknown } | null>(
      null,
    );
  const [task, setTask] = useState<TaskDetail | null>(null),
    [transfer, setTransfer] = useState<Transfer | null>(null),
    [toolId, setToolId] = useState(""),
    [versions, setVersions] = useState<ToolVersion[]>([]),
    [version, setVersion] = useState(""),
    [matches, setMatches] = useState<Compatibility[]>([]);
  useEffect(() => {
    history.replaceState(null, "", "#" + page);
  }, [page]);
  const device = state.snapshot.devices.find((d) => d.device_id === selected);
  const maintenance = state.snapshot.maintenance
    .filter((m) => m.device_id === selected)
    .sort((a, b) => b.created_at.localeCompare(a.created_at));
  const active = maintenance.find((m) => !m.released) ?? maintenance[0];
  const enabled =
    !!device &&
    device.status === "online" &&
    state.synchronized &&
    !busy &&
    !state.busy &&
    !state.pending;
  async function disconnect() {
    generation.current++;
    const old = owner.current;
    owner.current = null;
    setState({
      snapshot: emptySnapshot(),
      status: "未连接",
      synchronized: false,
      pending: null,
      busy: false,
      error: "",
    });
    setTask(null);
    setTransfer(null);
    setVersions([]);
    setMatches([]);
    setSelected("");
    setDialog(null);
    setInspect(null);
    try { await platform.closeTerminals(); }
    finally { await old?.dispose(); }
  }
  async function connect(p: Profile) {
    const url = baseUrl(p.server_url);
    await disconnect();
    await platform.connect(p);
    const c = new Connection(url, (s) => {
      if (owner.current === c) setState(s);
    });
    owner.current = c;
    c.start();
  }
  useEffect(() => {
    let mounted = true;
    void platform
      .loadProfile()
      .then((p) => {
        if (mounted) {
          setProfile(p);
          setLoaded(true);
          if (autoConnect.current) {
            autoConnect.current = false;
            void connect(p).catch((e) => setError(describe(e)));
          }
        }
      })
      .catch((e) => setError(describe(e)));
    const timer = setInterval(() => setNow(Date.now()), 1000);
    window.workbenchShutdown = async (nativeClosing = false) => {
      generation.current++;
      if (!nativeClosing) await platform.closeTerminals();
      platform.dispose();
      await owner.current?.dispose();
    };
    return () => {
      mounted = false;
      clearInterval(timer);
      void owner.current?.dispose();
    };
  }, []);
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const apply = () =>
      (document.documentElement.dataset.theme =
        profile.theme === "Dark" ||
        (profile.theme === "Default" && media.matches)
          ? "dark"
          : "light");
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [profile.theme]);
  useEffect(() => {
    if (!state.snapshot.devices.some((d) => d.device_id === selected))
      setSelected(state.snapshot.devices[0]?.device_id ?? "");
  }, [state.snapshot.devices, selected]);
  useEffect(() => {
    setTask(null);
    setTransfer(null);
    setMatches([]);
    setInspect(null);
    setDialog(null);
  }, [selected]);
  const taskQuery = useQuery(
    owner.current,
    task?.task_id ?? "",
    state.snapshot.fetchedAt,
    async (c) => {
      const detail = await c.api.get<TaskDetail>(
        "tasks/" + segment(task!.task_id),
      );
      const file =
        detail.type === "exec"
          ? null
          : await c.api.get<Transfer>(
              "tasks/" + segment(detail.task_id) + "/transfer",
            );
      return { detail, file };
    },
    setError,
  );
  useEffect(() => {
    if (taskQuery) {
      setTask(taskQuery.detail);
      setTransfer(taskQuery.file);
    }
  }, [taskQuery]);
  const versionQuery = useQuery(
    owner.current,
    toolId,
    state.snapshot.fetchedAt,
    (c) =>
      c.api.list<ToolVersion>(
        "tools/" + segment(toolId) + "/versions?include_archived=true",
      ),
    setError,
  );
  useEffect(() => {
    setVersions(versionQuery ?? []);
    if (versionQuery)
      setVersion((current) =>
        versionQuery.some((v) => v.version === current)
          ? current
          : (versionQuery[0]?.version ?? ""),
      );
  }, [versionQuery]);
  const matchQuery = useQuery(
    owner.current,
    toolId && version && selected
      ? JSON.stringify([toolId, version, selected])
      : "",
    state.snapshot.fetchedAt,
    (c) =>
      c.api.list<Compatibility>(
        "tools/" +
          segment(toolId) +
          "/versions/" +
          segment(version) +
          "/compatibility?device_id=" +
          segment(selected),
      ),
    setError,
  );
  useEffect(() => {
    setMatches(matchQuery ?? []);
  }, [matchQuery]);
  async function run(label: string, fn: () => Promise<void>) {
    if (
      actionGate.current ||
      (lastAction.current.label === label &&
        Date.now() - lastAction.current.at < 400)
    )
      return;
    actionGate.current = true;
    actionNotice.current = "";
    setBusy(true);
    setError("");
    const g = generation.current;
    try {
      await fn();
      if (g === generation.current) setNotice(actionNotice.current || label + "完成");
    } catch (e) {
      if (g === generation.current) setError(describe(e));
    } finally {
      actionGate.current = false;
      lastAction.current = { label, at: Date.now() };
      setBusy(false);
    }
  }
  async function write(m: Mutation) {
    const c = owner.current;
    if (!c) throw new Error("请先连接服务器");
    const data = await c.execute(m);
    if (owner.current !== c) throw new Error("连接已切换");
    if (data?.dispatch_uncertain) {
      actionNotice.current = `派发响应不确定，已保留任务 ${data.task_id}；请查询原任务`;
      setNotice(actionNotice.current);
    }
    return data;
  }
  function form(
    title: string,
    fields: {
      key: string;
      label: string;
      value: string;
      multiline?: boolean;
    }[],
    fn: (v: Record<string, string>) => Promise<void>,
  ) {
    setDialog({ title, fields, run: fn });
  }
  const field = (key: string, label: string, value = "") => ({
    key,
    label,
    value,
  });
  function exec() {
    form(
      "新建 Exec 任务",
      [
        { ...field("command", "命令", "uname -a"), multiline: true },
        field("cwd", "工作目录（可选）"),
        field("timeout", "超时（秒）", "30"),
        { ...field("env", "环境变量 JSON（可选）", "{}"), multiline: true },
      ],
      async (v) => {
        const env = JSON.parse(v.env);
        if (
          !env ||
          Array.isArray(env) ||
          typeof env !== "object" ||
          Object.values(env).some((x) => typeof x !== "string")
        )
          throw Error("环境变量须为字符串映射");
        await write(
          mutation("创建任务", "tasks", {
            device_id: selected,
            command: v.command,
            timeout_seconds: positive(v.timeout),
            ...(v.cwd ? { cwd: v.cwd } : {}),
            env,
          }),
        );
        setPage("tasks");
      },
    );
  }
  function upload(asset: Asset) {
    form(
      "上传文件到设备",
      [
        field("path", "设备目标路径", "/tmp/" + asset.name),
        field("mode", "文件权限", "0644"),
        field("timeout", "超时（秒）", "60"),
        field("overwrite", "覆盖已有文件（true / false）", "false"),
      ],
      async (v) => {
        await write(
          mutation("上传文件", "uploads", {
            device_id: selected,
            asset_id: asset.asset_id,
            remote_path: v.path,
            mode: v.mode,
            timeout_seconds: positive(v.timeout),
            overwrite: boolean(v.overwrite),
          }),
        );
        setPage("tasks");
      },
    );
  }
  function download() {
    form(
      "从设备下载文件",
      [
        field("path", "设备文件路径", "/tmp/result.txt"),
        field("name", "仓库文件名称", "result.txt"),
        field("timeout", "超时（秒）", "60"),
      ],
      async (v) => {
        await write(
          mutation("下载文件", "downloads", {
            device_id: selected,
            remote_path: v.path,
            name: v.name,
            timeout_seconds: positive(v.timeout),
          }),
        );
        setPage("tasks");
      },
    );
  }
  function publish(tool: Tool) {
    form(
      "发布不可变工具版本",
      [
        field("version", "版本标签"),
        {
          ...field(
            "artifacts",
            "产物列表 JSON",
            JSON.stringify(
              [
                {
                  asset_id:
                    state.snapshot.assets.find((a) => !a.archived)?.asset_id ??
                    "",
                  platform: "linux",
                  mode: "0755",
                  rules: { arch: ["any"], libc: ["any"] },
                },
              ],
              null,
              2,
            ),
          ),
          multiline: true,
        },
      ],
      async (v) => {
        await write(
          mutation(
            "发布版本",
            `tools/${segment(tool.tool_id)}/versions/${segment(v.version)}`,
            { artifacts: JSON.parse(v.artifacts) },
            "PUT",
          ),
        );
      },
    );
  }
  async function endpointFor(m: Maintenance, service: string) {
    const c = owner.current;
    if (!c) throw Error("请连接服务器");
    const current = await c.track(() =>
      c.api.get<Maintenance>("maintenance/" + segment(m.maintenance_id)),
    );
    const d = await c.track(() =>
      c.api.get<Device>("devices/" + segment(m.device_id)),
    );
    if (
      owner.current !== c ||
      current.released ||
      current.state !== "ready" ||
      d.current_session?.session_id !== current.session_id
    )
      throw Error("维护已关闭或会话已变化，请刷新");
    const endpoint = current.endpoints.find((e) => e.service === service);
    if (!endpoint || endpoint.state !== "ready") throw Error("维护入口不可用");
    return {endpoint, current};
  }
  async function open(m: Maintenance, service: string) {
    const {endpoint}=await endpointFor(m,service);
    await (
      service === "web"
        ? platform.openWeb
        : service === "ssh"
          ? platform.openSSH
          : platform.openTelnet
    )(endpoint);
  }
  const devices = state.snapshot.devices.filter(
    (d) =>
      (!onlineOnly || d.status === "online") &&
      `${d.device_id} ${d.registration.hostname} ${d.registration.model}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const selectedTool = state.snapshot.tools.find((t) => t.tool_id === toolId);
  const remaining = active
    ? Math.max(0, new Date(active.expires_at).getTime() - now)
    : 0;
  const seconds = Math.floor(remaining / 1000);
  return {
    profile,
    setProfile,
    loaded,
    page,
    setPage,
    selected,
    setSelected,
    query,
    setQuery,
    onlineOnly,
    setOnlineOnly,
    notice,
    setNotice,
    error,
    setError,
    busy,
    minutes,
    setMinutes,
    state,
    owner,
    dialog,
    setDialog,
    inspect,
    setInspect,
    task,
    setTask,
    transfer,
    toolId,
    setToolId,
    versions,
    version,
    setVersion,
    matches,
    device,
    maintenance,
    active,
    enabled,
    disconnect,
    connect,
    run,
    write,
    form,
    field,
    exec,
    upload,
    download,
    publish,
    open,
    endpointFor,
    devices,
    selectedTool,
    remaining,
    seconds,
  };
}
export type Workbench = ReturnType<typeof useWorkbench>;

export function positive(value: string) {
  const n = Number(value);
  if (!Number.isInteger(n) || n < 1 || n > 4294967295)
    throw Error("超时必须为正整数秒");
  return n;
}
export function boolean(value: string) {
  if (!["true", "false"].includes(value))
    throw Error("覆盖选项须为 true 或 false");
  return value === "true";
}
