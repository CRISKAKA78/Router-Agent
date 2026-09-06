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
  ShieldCheck,
  ChevronRight,
  MoreVertical,
  X,
  Home,
  Info,
  Check,
  Monitor,
} from "lucide-react";
import { describe } from "./api";
import { Badge, FormDialog } from "./components";
import { platform } from "./platform";
import { useWorkbench } from "./useWorkbench";
import { OverviewPage } from "./pages/OverviewPage";
import { TasksPage } from "./pages/TasksPage";
import { FilesPage } from "./pages/FilesPage";
import { ToolsPage } from "./pages/ToolsPage";
import { DetailsPage } from "./pages/DetailsPage";
import { SettingsPage } from "./pages/SettingsPage";

type Page = "overview" | "tasks" | "files" | "tools" | "details" | "settings";
const tabs: [Page, string, typeof Home][] = [
  ["overview", "概览", Home],
  ["tasks", "任务 / Exec", Terminal],
  ["files", "文件管理", Folder],
  ["tools", "工具 / 版本", Package],
  ["details", "设备详情", Info],
];
const stamp = (s: string | null | undefined) =>
  s ? new Date(s).toLocaleString("zh-CN", { hour12: false }) : "—";

export function App() {
  const model = useWorkbench();
  const {
    profile,
    setProfile,
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
    state,
    owner,
    dialog,
    setDialog,
    inspect,
    setInspect,
    device,
    run,
    form,
    field,
    devices,
  } = model;
  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <div className="brand-icon">
            <Router size={30} />
          </div>
          <div>
            <strong>路由器远程维护工作台</strong>
            <small>Router Maintenance Workbench</small>
          </div>
        </div>
        <div className="top-actions">
          <span
            className={"connection-dot " + (state.synchronized ? "online" : "")}
          />
          <span className="connection-text">
            {state.status}
            <small>
              {state.synchronized ? "实时更新已开启" : "连接后开始管理设备"}
            </small>
          </span>
          <button
            className="icon-button"
            aria-label="系统设置"
            onClick={() => setPage("settings")}
          >
            <Settings />
          </button>
          <button
            className="icon-button"
            aria-label="切换主题"
            onClick={() => {
              const p = {
                ...profile,
                theme:
                  document.documentElement.dataset.theme === "dark"
                    ? ("Light" as const)
                    : ("Dark" as const),
              };
              setProfile(p);
              void platform.saveProfile(p).catch((e) => setError(describe(e)));
            }}
          >
            {document.documentElement.dataset.theme === "dark" ? (
              <Sun />
            ) : (
              <Moon />
            )}
          </button>
        </div>
      </header>
      <div className="workspace">
        <nav className="rail" aria-label="一级导航">
          {(
            [
              ["overview", "设备列表", PanelsTopLeft],
              ["tasks", "任务 / Exec", Terminal],
              ["files", "文件管理", Folder],
              ["tools", "工具仓库", Package],
              ["settings", "系统设置", Settings],
            ] as [Page, string, typeof Home][]
          ).map(([id, label, Icon]) => (
            <button
              key={id}
              className={
                page === id || (id === "overview" && page === "details")
                  ? "active"
                  : ""
              }
              onClick={() => setPage(id)}
            >
              <Icon size={21} />
              <span>{label}</span>
            </button>
          ))}
          <div className="rail-footer">
            <div className="avatar">
              <ShieldCheck />
            </div>
            <div>
              设备运维<small>让设备更稳定</small>
            </div>
          </div>
          <small className="version">v0.6.0 · Shared Frontend</small>
        </nav>
        <aside className="device-panel">
          <div className="panel-title">
            <h2>
              设备列表 <span>({state.snapshot.devices.length})</span>
            </h2>
            <button
              className="icon-button"
              aria-label="刷新设备"
              disabled={!owner.current}
              onClick={() => owner.current?.invalidate()}
            >
              <RefreshCw size={18} />
            </button>
          </div>
          <div className="search-row">
            <label className="search">
              <Search size={17} />
              <input
                aria-label="搜索设备"
                placeholder="搜索设备名称、ID 或主机名…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </label>
            <button
              className={"icon-button " + (onlineOnly ? "selected" : "")}
              aria-label="仅在线设备"
              aria-pressed={onlineOnly}
              onClick={() => setOnlineOnly(!onlineOnly)}
            >
              <Filter size={18} />
            </button>
          </div>
          <div className="devices">
            {devices.map((d) => (
              <button
                className={
                  "device-item " + (selected === d.device_id ? "selected" : "")
                }
                key={d.device_id}
                disabled={busy}
                onClick={() => {
                  setSelected(d.device_id);
                  if (page === "settings") setPage("overview");
                }}
              >
                <span
                  className={
                    "connection-dot " + (d.status === "online" ? "online" : "")
                  }
                />
                <div>
                  <strong>{d.registration.hostname || d.device_id}</strong>
                  <small>{d.device_id}</small>
                  <span>
                    {d.registration.model || "Linux"} ·{" "}
                    {d.registration.arch || "架构未知"}
                  </span>
                </div>
                <Badge state={d.status} />
              </button>
            ))}
            {!devices.length && (
              <div className="empty">
                <Monitor />
                <p>
                  {owner.current
                    ? "暂无匹配设备"
                    : "连接服务器后，设备将显示在这里"}
                </p>
                {!owner.current && (
                  <button onClick={() => setPage("settings")}>
                    配置连接 <ChevronRight size={15} />
                  </button>
                )}
              </div>
            )}
          </div>
        </aside>
        <main className="main-panel">
          <div className="notices" aria-live="polite">
            {(error || state.error) && (
              <div className="alert error">
                <Info size={18} />
                <span>{error || state.error}</span>
                <button aria-label="关闭错误" onClick={() => setError("")}>
                  <X size={16} />
                </button>
              </div>
            )}
            {notice && (
              <div className="alert">
                <Check size={16} />
                <span>{notice}</span>
                <button aria-label="关闭提示" onClick={() => setNotice("")}>
                  <X size={16} />
                </button>
              </div>
            )}
            {state.pending && !state.busy && (
              <div className="alert warning">
                <span>
                  “{state.pending.label}”响应不确定。确认 Server
                  没有重启后才能重试原请求。
                </span>
                <button
                  disabled={busy}
                  onClick={() =>
                    form(
                      "确认原请求重试",
                      [field("confirmed", "已确认服务器未重启（输入 是）")],
                      async (v) => {
                        if (v.confirmed !== "是")
                          throw Error("需明确确认 Server 未重启");
                        await owner.current!.execute(state.pending!, true);
                      },
                    )
                  }
                >
                  核对并重试
                </button>
                <button
                  onClick={() =>
                    form(
                      "放弃原请求",
                      [field("confirmed", "已核对服务器结果（输入 是）")],
                      async (v) => {
                        if (v.confirmed !== "是") throw Error("请先核对");
                        owner.current?.abandon();
                      },
                    )
                  }
                >
                  核对后放弃
                </button>
              </div>
            )}
          </div>
          {page === "settings" ? (
            <SettingsPage model={model} />
          ) : (
            <>
              <div className="device-header">
                <div>
                  <div className="title-line">
                    <h1>
                      {device?.registration.hostname ||
                        device?.device_id ||
                        "设备工作台"}
                    </h1>
                    {device && <Badge state={device.status} />}
                  </div>
                  <div className="device-meta">
                    <span>
                      设备 ID <b>{device?.device_id ?? "尚未选择"}</b>
                    </span>
                    <span>
                      系统{" "}
                      <b>
                        {device
                          ? "Linux / " + (device.registration.arch || "未知")
                          : "—"}
                      </b>
                    </span>
                    <span>
                      主机名 <b>{device?.registration.hostname || "—"}</b>
                    </span>
                    <span>
                      最后在线 <b>{stamp(device?.last_online_at)}</b>
                    </span>
                  </div>
                </div>
                <div className="button-row">
                  <button
                    disabled={!owner.current || busy}
                    onClick={() => owner.current?.invalidate()}
                  >
                    <RefreshCw size={17} />
                    刷新
                  </button>
                  <button
                    aria-label="设备更多操作"
                    disabled={!device || busy}
                    onClick={() => setPage("details")}
                  >
                    <MoreVertical size={18} />
                    <span>更多操作</span>
                  </button>
                </div>
              </div>
              <div className="tabs" role="tablist">
                {tabs.map(([id, label, Icon]) => (
                  <button
                    key={id}
                    role="tab"
                    aria-selected={page === id}
                    className={page === id ? "active" : ""}
                    onClick={() => setPage(id)}
                  >
                    <Icon size={20} />
                    {label}
                  </button>
                ))}
              </div>
              <div className="page-content">
                {page === "overview" && <OverviewPage model={model} />}
                {page === "tasks" && <TasksPage model={model} />}
                {page === "files" && <FilesPage model={model} />}
                {page === "tools" && <ToolsPage model={model} />}
                {page === "details" && <DetailsPage model={model} />}
              </div>
            </>
          )}
        </main>
      </div>
      {dialog && (
        <FormDialog
          error={error}
          title={dialog.title}
          fields={dialog.fields}
          busy={busy}
          onClose={() => setDialog(null)}
          onSubmit={(v) =>
            void run(dialog.title, async () => {
              await dialog.run(v);
              setDialog(null);
            })
          }
        />
      )}{" "}
      {inspect && (
        <div className="modal-backdrop">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label={inspect.title}
          >
            <div className="card-heading">
              <h2>{inspect.title}</h2>
              <button aria-label="关闭详情" onClick={() => setInspect(null)}>
                <X />
              </button>
            </div>
            <pre>{JSON.stringify(inspect.value, null, 2)}</pre>
          </section>
        </div>
      )}
    </div>
  );
}
