import { defaults, type Endpoint, type Profile } from "./models";
import { hashBlob } from "./api";
declare global {
  interface Window {
    chrome?: {
      webview?: {
        postMessage: (v: unknown) => void;
        addEventListener: (
          type: string,
          fn: (e: { data: any }) => void,
        ) => void;
      };
    };
    workbenchShutdown?: () => Promise<void>;
  }
}
const bridge = window.chrome?.webview;
let sequence = 0;
const requests = new Map<
  string,
  { resolve: (v: any) => void; reject: (e: Error) => void }
>();
const pickers = new Set<() => void>();
bridge?.addEventListener("message", (e) => {
  const pending = requests.get(e.data.id);
  if (pending) {
    requests.delete(e.data.id);
    e.data.error
      ? pending.reject(new Error(e.data.error))
      : pending.resolve(e.data.result);
  }
});
export function nativeCall<T>(method: string, args: unknown = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    if (!bridge) return reject(new Error("当前环境不支持此平台操作"));
    const id = String(++sequence);
    requests.set(id, { resolve, reject });
    bridge.postMessage({ id, method, args });
  });
}
export const platform = {
  dispose() {
    for (const cancel of pickers) cancel();
    pickers.clear();
    for (const request of requests.values())
      request.reject(new Error("应用已关闭"));
    requests.clear();
  },
  async loadProfile(): Promise<Profile> {
    if (bridge) return nativeCall("getProfile");
    try {
      return {
        ...defaults,
        server_url: location.origin,
        ...JSON.parse(localStorage.getItem("workbench.profile") ?? "{}"),
      };
    } catch {
      return { ...defaults, server_url: location.origin };
    }
  },
  async saveProfile(profile: Profile) {
    if (bridge) return nativeCall<void>("saveProfile", profile);
    localStorage.setItem("workbench.profile", JSON.stringify(profile));
  },
  async connect(profile: Profile) {
    if (bridge) await nativeCall("connect", { server_url: profile.server_url });
  },
  async chooseClient(service: "ssh" | "telnet") {
    if (!bridge)
      throw new Error("浏览器环境请使用本机 SSH / Telnet 客户端连接显示的地址");
    return nativeCall<Profile>("chooseClient", { service });
  },
  async openWeb(endpoint: Endpoint) {
    if (bridge) return nativeCall<void>("openEndpoint", endpoint);
    if (!endpoint.url || !/^http:\/\//.test(endpoint.url))
      throw new Error("无效 Web 地址");
    window.open(endpoint.url, "_blank", "noopener,noreferrer");
  },
  async openSSH(endpoint: Endpoint) {
    if (bridge) return nativeCall<void>("openEndpoint", endpoint);
    throw new Error(`请在本机 SSH 客户端连接 ${endpoint.address}`);
  },
  async openTelnet(endpoint: Endpoint) {
    if (bridge) return nativeCall<void>("openEndpoint", endpoint);
    throw new Error(`请在本机 Telnet 客户端连接 ${endpoint.address}`);
  },
  pickFile(): Promise<File | null> {
    return new Promise((resolve) => {
      const input = document.createElement("input");
      input.type = "file";
      input.hidden = true;
      const done = (file: File | null) => {
        pickers.delete(cancel);
        input.remove();
        resolve(file);
      };
      const cancel = () => done(null);
      pickers.add(cancel);
      input.onchange = () => done(input.files?.[0] ?? null);
      input.oncancel = cancel;
      document.body.append(input);
      input.click();
    });
  },
  async saveFile(name: string, blob: Blob) {
    const safeName = name.replace(/[\\/:*?"<>|]/g, "_");
    if (bridge) {
      const handle = await nativeCall<string | null>("beginSave", {
        name: safeName,
        size: blob.size,
        sha256: await hashBlob(blob),
      });
      if (!handle) return;
      try {
        for (let offset = 0; offset < blob.size; offset += 65536) {
          const bytes = new Uint8Array(
            await blob.slice(offset, offset + 65536).arrayBuffer(),
          );
          await nativeCall("saveChunk", {
            handle,
            offset,
            base64: btoa(String.fromCharCode(...bytes)),
          });
        }
        await nativeCall("finishSave", { handle });
      } catch (e) {
        await nativeCall("cancelSave", { handle }).catch(() => {});
        throw e;
      }
      return;
    }
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = safeName;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 60000);
  },
};
