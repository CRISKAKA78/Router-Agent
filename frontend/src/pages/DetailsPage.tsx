import { Clock, Info } from "lucide-react";
import { mutation, segment } from "../api";
import { type Session } from "../models";
import { Card, stateName } from "../components";
import type { Workbench } from "../useWorkbench";
import { stamp } from "../useWorkbench";
export function DetailsPage({ model }: { model: Workbench }) {
  const {
    selected,
    busy,
    state,
    owner,
    setInspect,
    device,
    maintenance,
    enabled,
    run,
    write,
    form,
    field,
  } = model;
  return (
    <>
      <Card
        title="设备与 Session"
        icon={Info}
        action={
          <button
            disabled={!enabled}
            onClick={() =>
              form(
                "断开设备当前会话",
                [field("confirm", "输入设备 ID 确认", "")],
                async (v) => {
                  if (v.confirm !== selected) throw Error("设备 ID 不匹配");
                  await write(
                    mutation(
                      "断开设备",
                      `devices/${segment(selected)}/disconnect`,
                    ),
                  );
                },
              )
            }
          >
            断开设备
          </button>
        }
      >
        <pre>{JSON.stringify(device, null, 2)}</pre>
        <button
          disabled={!device || busy}
          onClick={() =>
            void run("查询会话历史", async () => {
              const c = owner.current!;
              const sessions = await c.track(() =>
                c.api.list<Session>(`devices/${segment(selected)}/sessions`),
              );
              if (c === owner.current)
                setInspect({
                  title: "Session 历史",
                  value: sessions,
                });
            })
          }
        >
          查看 Session 历史
        </button>
      </Card>
      <Card title="维护记录详情" icon={Clock}>
        {maintenance.map((m) => (
          <details key={m.maintenance_id}>
            <summary>
              {stamp(m.created_at)} · {stateName(m.state)} · {m.maintenance_id}
            </summary>
            <pre>{JSON.stringify(m, null, 2)}</pre>
            <button
              disabled={
                busy || m.released || !state.synchronized || !!state.pending
              }
              onClick={() =>
                void run("关闭维护", async () => {
                  await write(
                    mutation(
                      "关闭维护",
                      `maintenance/${segment(m.maintenance_id)}/close`,
                    ),
                  );
                })
              }
            >
              关闭维护
            </button>
          </details>
        ))}
      </Card>
    </>
  );
}
