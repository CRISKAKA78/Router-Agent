import {
  Terminal,
  Folder,
  Package,
  Globe,
  ExternalLink,
  Clock,
  ShieldCheck,
  Activity,
  Send,
  Upload,
  ChevronRight,
  Power,
} from "lucide-react";
import { leaseBody, mutation, segment } from "../api";
import { Badge, Card } from "../components";
import type { Workbench } from "../useWorkbench";
import { stamp } from "../useWorkbench";
export function OverviewPage({ model }: { model: Workbench }) {
  const {
    setPage,
    selected,
    minutes,
    setMinutes,
    state,
    setInspect,
    device,
    maintenance,
    active,
    enabled,
    run,
    write,
    exec,
    open,
    remaining,
    seconds,
  } = model;
  return (
    <>
      <div className="overview-grid">
        <Card
          title="远程维护"
          icon={ShieldCheck}
          action={
            active ? (
              <Badge state={active.state} />
            ) : (
              <span className="badge muted">未开启</span>
            )
          }
          className="maintenance-card"
        >
          <p className="subtle">
            通过临时通信通道访问设备的 Web 配置页面或进行 SSH / Telnet 维护
          </p>
          <div className="countdown">
            <div className="clock-icon">
              <Clock size={29} />
            </div>
            <div>
              <span>
                {active && !active.released ? "剩余时间" : "随时连接你的设备"}
              </span>
              <strong>
                {active && !active.released
                  ? `${Math.floor(seconds / 3600)} 小时 ${Math.floor(seconds / 60) % 60} 分 ${seconds % 60} 秒`
                  : "开启安全的远程维护"}
              </strong>
            </div>
          </div>
          <div className="progress">
            <i
              style={{
                width:
                  active && !active.released
                    ? `${Math.min(100, (remaining / Math.max(1, new Date(active.expires_at).getTime() - new Date(active.created_at).getTime())) * 100)}%`
                    : "0%",
              }}
            />
          </div>
          <div className="time-meta">
            <span>
              {active && !active.released
                ? "到期后由服务器关闭入口"
                : "默认维护时长 240 分钟"}
            </span>
            <span>
              {active ? stamp(active.expires_at) : "支持自定义正租期"}
            </span>
          </div>
          <div className="endpoints">
            {(["web", "ssh", "telnet"] as const).map((service, i) => {
              const ep = active?.endpoints.find((e) => e.service === service);
              const Icon = [Globe, Terminal, Folder][i];
              return (
                <div className="endpoint" key={service}>
                  <Icon size={25} />
                  <strong>{["Web", "SSH", "Telnet"][i]}</strong>
                  <span className="endpoint-address">
                    {ep ? ep.url || ep.address : "开启维护后可用"}
                  </span>
                  <button
                    disabled={
                      !enabled || active?.released || ep?.state !== "ready"
                    }
                    onClick={() =>
                      void run("打开 " + service, () => open(active!, service))
                    }
                  >
                    {i === 0 ? "打开" : "连接"}
                    {i === 0 && <ExternalLink size={16} />}
                  </button>
                </div>
              );
            })}
          </div>
          <div className="lease-row">
            <label>
              维护时长
              <input
                aria-label="维护时长（分钟）"
                value={minutes}
                onChange={(e) => setMinutes(e.target.value)}
              />
              <span>分钟</span>
            </label>
            <button
              className="primary"
              disabled={!enabled || (!!active && !active.released)}
              onClick={() =>
                void run("开启远程维护", async () => {
                  await write(
                    mutation(
                      "开启远程维护",
                      "maintenance",
                      leaseBody(selected, minutes),
                    ),
                  );
                })
              }
            >
              <Power size={17} />
              开启维护
            </button>
            <button
              className="danger"
              disabled={!enabled || !active || active.released}
              onClick={() =>
                void run("关闭维护", async () => {
                  await write(
                    mutation(
                      "关闭维护",
                      `maintenance/${segment(active!.maintenance_id)}/close`,
                    ),
                  );
                })
              }
            >
              关闭维护
            </button>
          </div>
          {state.snapshot.maintenanceError && (
            <p className="subtle">{state.snapshot.maintenanceError}</p>
          )}
        </Card>
        <div className="side-cards">
          <Card title="设备状态" icon={Activity}>
            <dl>
              <dt>在线状态</dt>
              <dd>{device ? <Badge state={device.status} /> : "—"}</dd>
              <dt>连接次数</dt>
              <dd>{device?.total_sessions ?? "—"}</dd>
              <dt>最后上线</dt>
              <dd>{stamp(device?.last_online_at)}</dd>
              <dt>当前维护连接</dt>
              <dd>{active?.connections ?? 0}</dd>
              <dt>系统信息</dt>
              <dd>{device ? "Linux / " + device.registration.arch : "—"}</dd>
              <dt>版本信息</dt>
              <dd>{device?.registration.probe_version || "—"}</dd>
            </dl>
          </Card>
          <Card title="快捷操作" icon={Send}>
            <div className="quick-grid">
              <button disabled={!enabled} onClick={exec}>
                <Terminal size={17} />
                新建 Exec 任务
              </button>
              <button disabled={!enabled} onClick={() => setPage("files")}>
                <Upload size={17} />
                上传文件
              </button>
              <button disabled={!enabled} onClick={() => setPage("tools")}>
                <Package size={17} />
                投放工具
              </button>
              <button disabled={!device} onClick={() => setPage("tasks")}>
                <Folder size={17} />
                查看任务
              </button>
            </div>
          </Card>
        </div>
      </div>
      <Card
        title={`最近的维护记录 (${maintenance.length})`}
        icon={Clock}
        action={
          <button className="text-button" onClick={() => setPage("details")}>
            查看全部 <ChevronRight size={15} />
          </button>
        }
      >
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>状态</th>
                <th>开始时间</th>
                <th>到期时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {maintenance.slice(0, 4).map((m) => (
                <tr key={m.maintenance_id}>
                  <td>
                    <Badge state={m.state} />
                  </td>
                  <td>{stamp(m.created_at)}</td>
                  <td>{stamp(m.expires_at)}</td>
                  <td>
                    <button
                      onClick={() =>
                        setInspect({
                          title: "维护详情",
                          value: m,
                        })
                      }
                    >
                      详情
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!maintenance.length && (
            <p className="empty-row">开启第一次远程维护，记录会显示在这里。</p>
          )}
        </div>
      </Card>
    </>
  );
}
