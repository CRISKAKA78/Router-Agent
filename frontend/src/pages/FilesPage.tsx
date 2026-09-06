import { Folder, Upload, Download } from "lucide-react";
import { importMutation, mutation, segment } from "../api";
import { Card } from "../components";
import { platform } from "../platform";
import type { Workbench } from "../useWorkbench";
export function FilesPage({ model }: { model: Workbench }) {
  const { busy, state, owner, enabled, run, write, upload, download } = model;
  return (
    <Card
      title="文件仓库"
      icon={Folder}
      action={
        <div className="button-row">
          <button
            disabled={!owner.current || busy || !!state.pending}
            onClick={() =>
              void run("导入文件", async () => {
                const file = await platform.pickFile();
                if (file) await write(await importMutation(file));
              })
            }
          >
            <Upload size={17} />
            导入文件
          </button>
          <button className="primary" disabled={!enabled} onClick={download}>
            <Download size={17} />
            从设备下载
          </button>
        </div>
      }
    >
      <p className="subtle">
        导入仓库后上传至设备；设备下载完成后，在任务结果中显式导入。
      </p>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>文件名</th>
              <th>大小</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {state.snapshot.assets.map((a) => (
              <tr key={a.asset_id}>
                <td>
                  <strong>{a.name}</strong>
                  <small className="mono">{a.asset_id}</small>
                </td>
                <td>{(a.size / 1024).toFixed(1)} KB</td>
                <td>{a.archived ? "已归档" : "可用"}</td>
                <td>
                  <div className="button-row">
                    <button
                      disabled={!enabled || a.archived}
                      onClick={() => upload(a)}
                    >
                      上传
                    </button>
                    <button
                      disabled={busy || a.archived}
                      onClick={() =>
                        void run("另存文件", async () => {
                          const c = owner.current!;
                          const blob = await c.track(() => c.api.content(a));
                          if (c === owner.current)
                            await platform.saveFile(a.name, blob);
                        })
                      }
                    >
                      另存
                    </button>
                    <button
                      disabled={busy || a.archived || !!state.pending}
                      onClick={() =>
                        void run("归档文件", async () => {
                          await write(
                            mutation(
                              "归档文件",
                              `assets/${segment(a.asset_id)}/archive`,
                            ),
                          );
                        })
                      }
                    >
                      归档
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
