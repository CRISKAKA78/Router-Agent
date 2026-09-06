import { Terminal, Plus } from "lucide-react";
import { mutation, segment } from "../api";
import { type TaskDetail } from "../models";
import { Badge, Card } from "../components";
import type { Workbench } from "../useWorkbench";
import { stamp } from "../useWorkbench";
export function TasksPage({ model }: { model: Workbench }) {
  const {
    selected,
    state,
    owner,
    setInspect,
    task,
    setTask,
    transfer,
    enabled,
    run,
    write,
    exec,
  } = model;
  return (
    <>
      <Card
        title="设备任务"
        icon={Terminal}
        action={
          <button className="primary" disabled={!enabled} onClick={exec}>
            <Plus size={17} />
            新建 Exec
          </button>
        }
      >
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>类型</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>任务编号</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {state.snapshot.tasks
                .filter((t) => t.device_id === selected)
                .map((t) => (
                  <tr key={t.task_id}>
                    <td>{t.type}</td>
                    <td>
                      <Badge state={t.state} />
                    </td>
                    <td>{stamp(t.created_at)}</td>
                    <td className="mono">{t.task_id}</td>
                    <td>
                      <button
                        onClick={() =>
                          void run("查询任务", async () => {
                            const c = owner.current!;
                            const d = await c.track(() =>
                              c.api.get<TaskDetail>(
                                "tasks/" + segment(t.task_id),
                              ),
                            );
                            if (c === owner.current) setTask(d);
                          })
                        }
                      >
                        结果
                      </button>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </Card>
      {task && (
        <Card
          title="任务结果"
          icon={Terminal}
          action={<Badge state={task.state} />}
        >
          <p className="mono">{task.command || task.type}</p>
          <div className="terminal">
            <small>STDOUT</small>
            <pre>{task.result?.stdout ?? "尚未收到最终 RESULT"}</pre>
            <small>STDERR</small>
            <pre>{task.result?.stderr || "—"}</pre>
          </div>
          <p className="subtle">
            退出码 {task.result?.exit_code ?? "—"} · 输出截断{" "}
            {task.result?.truncated ? "是" : "否"} · 派发次数{" "}
            {task.dispatch_count}
          </p>
          {transfer && (
            <p>
              文件已提交 {transfer.committed ? "是" : "否"} · Worker 已释放{" "}
              {transfer.released ? "是" : "否"} · 文件失败{" "}
              {transfer.failed ? "是" : "否"}（独立于任务 RESULT）
            </p>
          )}
          <div className="button-row">
            <button
              disabled={!enabled}
              onClick={() =>
                void run("重发原任务", async () => {
                  await write(
                    mutation(
                      "重发原任务",
                      `tasks/${segment(task.task_id)}/resend`,
                    ),
                  );
                })
              }
            >
              重发 / 查询原任务
            </button>
            {task.type === "download" && (
              <>
                <button
                  disabled={
                    !enabled || !transfer?.committed || !transfer?.released
                  }
                  onClick={() =>
                    void run("导入已提交下载", async () => {
                      const result = await write(
                        mutation(
                          "导入已提交下载",
                          `downloads/${segment(task.task_id)}/complete`,
                        ),
                      );
                      setInspect({
                        title: "下载导入结果",
                        value: result,
                      });
                    })
                  }
                >
                  导入已提交下载
                </button>
                <button
                  disabled={!enabled || !transfer?.released}
                  onClick={() =>
                    void run("清理下载暂存", async () => {
                      await write(
                        mutation(
                          "清理下载暂存",
                          `downloads/${segment(task.task_id)}/cleanup`,
                        ),
                      );
                    })
                  }
                >
                  清理暂存
                </button>
              </>
            )}
          </div>
        </Card>
      )}
    </>
  );
}
