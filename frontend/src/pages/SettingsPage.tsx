import { Terminal, Settings, Sun, Moon, Power, Monitor } from "lucide-react";
import { baseUrl } from "../api";
import { Card } from "../components";
import { platform } from "../platform";
import type { Workbench } from "../useWorkbench";
export function SettingsPage({ model }: { model: Workbench }) {
  const { profile, setProfile, loaded, busy, owner, disconnect, connect, run } =
    model;
  return (
    <div className="settings-page">
      <div className="page-heading">
        <span className="eyebrow">WORKSPACE SETTINGS</span>
        <h1>系统设置</h1>
        <p>连接你的管理服务器，按习惯调整工作台。</p>
      </div>
      <Card title="服务器连接" icon={Settings}>
        <label>
          管理服务器地址
          <input
            value={profile.server_url}
            onChange={(e) =>
              setProfile({ ...profile, server_url: e.target.value })
            }
            placeholder="http://127.0.0.1:8080"
          />
        </label>
        <div className="button-row">
          <button
            className="primary"
            disabled={busy || !loaded}
            onClick={() =>
              void run("连接服务器", async () => {
                baseUrl(profile.server_url);
                await platform.saveProfile(profile);
                await connect(profile);
              })
            }
          >
            <Power size={17} />
            保存并连接
          </button>
          <button
            disabled={busy || !owner.current}
            onClick={() => void run("断开连接", disconnect)}
          >
            断开连接
          </button>
        </div>
      </Card>
      <Card title="外观" icon={Sun}>
        <div className="theme-options">
          {(["Default", "Light", "Dark"] as const).map((t, i) => (
            <button
              className={profile.theme === t ? "selected" : ""}
              key={t}
              onClick={() =>
                void run("保存外观", async () => {
                  const p = { ...profile, theme: t };
                  setProfile(p);
                  await platform.saveProfile(p);
                })
              }
            >
              {[<Monitor />, <Sun />, <Moon />][i]}
              {["跟随系统", "浅色", "深色"][i]}
            </button>
          ))}
        </div>
      </Card>
      <Card title="外部客户端" icon={Terminal}>
        <label>
          SSH 用户名
          <input
            value={profile.ssh_user}
            onChange={(e) =>
              setProfile({ ...profile, ssh_user: e.target.value })
            }
          />
        </label>
        <div className="button-row">
          <button
            onClick={() =>
              void run("保存客户端设置", () => platform.saveProfile(profile))
            }
          >
            保存用户名
          </button>
          {(["ssh", "telnet"] as const).map((s) => (
            <button
              key={s}
              onClick={() =>
                void run("选择客户端", async () =>
                  setProfile(await platform.chooseClient(s)),
                )
              }
            >
              选择 {s === "ssh" ? "SSH" : "Telnet"} / PuTTY
            </button>
          ))}
        </div>
        <p className="subtle">
          SSH/Telnet 默认使用页面内终端，依赖本机命令行客户端。新建会话菜单可调用已选择的外部客户端；GUI PuTTY 仅作为外部入口。Web 在系统浏览器打开。
        </p>
      </Card>
    </div>
  );
}
