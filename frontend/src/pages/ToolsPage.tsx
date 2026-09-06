import { Package, Plus } from "lucide-react";
import { mutation, segment } from "../api";
import { Badge, Card } from "../components";
import { platform } from "../platform";
import type { Workbench } from "../useWorkbench";
import { positive, boolean } from "../useWorkbench";
export function ToolsPage({ model }: { model: Workbench }) {
  const {
    setPage,
    selected,
    busy,
    state,
    owner,
    toolId,
    setToolId,
    versions,
    version,
    setVersion,
    matches,
    enabled,
    run,
    write,
    form,
    field,
    publish,
    selectedTool,
  } = model;
  return (
    <>
      <Card
        title="工具仓库"
        icon={Package}
        action={
          <button
            className="primary"
            disabled={!owner.current || busy || !!state.pending}
            onClick={() =>
              form(
                "创建工具",
                [field("name", "工具名称"), field("description", "描述")],
                async (v) => {
                  const t = await write(mutation("创建工具", "tools", v));
                  setToolId(t.tool_id);
                },
              )
            }
          >
            <Plus size={17} />
            创建工具
          </button>
        }
      >
        <div className="tool-grid">
          {state.snapshot.tools.map((t) => (
            <button
              key={t.tool_id}
              className={
                "tool-item " + (toolId === t.tool_id ? "selected" : "")
              }
              onClick={() => setToolId(t.tool_id)}
            >
              <Package />
              <strong>{t.name}</strong>
              <span>{t.description || "暂无描述"}</span>
              <small>{t.archived ? "已归档" : "可用"}</small>
            </button>
          ))}
        </div>
      </Card>
      {selectedTool && (
        <Card
          title={selectedTool.name + " · 版本与产物"}
          icon={Package}
          action={
            <div className="button-row">
              <button
                disabled={busy || selectedTool.archived || !!state.pending}
                onClick={() => publish(selectedTool)}
              >
                发布版本
              </button>
              <button
                disabled={busy || selectedTool.archived || !!state.pending}
                onClick={() =>
                  void run("归档工具", async () => {
                    await write(
                      mutation("归档工具", `tools/${segment(toolId)}/archive`),
                    );
                  })
                }
              >
                归档工具
              </button>
            </div>
          }
        >
          <label>
            版本
            <select
              value={version}
              onChange={(e) => setVersion(e.target.value)}
            >
              <option value="">请选择版本</option>
              {versions.map((v) => (
                <option key={v.version} value={v.version}>
                  {v.version}
                  {v.archived ? "（已归档）" : ""}
                </option>
              ))}
            </select>
          </label>
          {version && (
            <button
              disabled={
                busy ||
                !!state.pending ||
                versions.find((v) => v.version === version)?.archived
              }
              onClick={() =>
                void run("归档版本", async () => {
                  await write(
                    mutation(
                      "归档版本",
                      `tools/${segment(toolId)}/versions/${segment(version)}/archive`,
                    ),
                  );
                })
              }
            >
              归档当前版本
            </button>
          )}
          <div className="artifact-list">
            {matches.map((m) => (
              <div className="artifact" key={m.artifact.artifact_id}>
                <div>
                  <Badge state={m.status} />
                  <strong className="mono">{m.artifact.artifact_id}</strong>
                  <p>
                    {m.artifact.platform} · {m.artifact.rules.arch.join(", ")} ·{" "}
                    {m.artifact.rules.libc.join(", ")} · {m.artifact.mode}
                  </p>
                  <details>
                    <summary>兼容判断与产物详情</summary>
                    <pre>{JSON.stringify(m, null, 2)}</pre>
                  </details>
                </div>
                <button
                  className="primary"
                  disabled={
                    !enabled ||
                    m.status !== "compatible" ||
                    selectedTool.archived ||
                    versions.find((v) => v.version === version)?.archived
                  }
                  onClick={() =>
                    form(
                      "投放工具",
                      [
                        field("path", "设备目标路径", "/tmp/tool"),
                        field("timeout", "超时（秒）", "60"),
                        field(
                          "overwrite",
                          "覆盖已有文件（true / false）",
                          "false",
                        ),
                      ],
                      async (v) => {
                        await write(
                          mutation("投放工具", "deployments", {
                            device_id: selected,
                            tool_id: toolId,
                            version,
                            artifact_id: m.artifact.artifact_id,
                            remote_path: v.path,
                            timeout_seconds: positive(v.timeout),
                            overwrite: boolean(v.overwrite),
                          }),
                        );
                        setPage("tasks");
                      },
                    )
                  }
                >
                  投放
                </button>
              </div>
            ))}
          </div>
          {!selected && version && (
            <pre>
              {JSON.stringify(
                versions.find((v) => v.version === version),
                null,
                2,
              )}
            </pre>
          )}
        </Card>
      )}
    </>
  );
}
