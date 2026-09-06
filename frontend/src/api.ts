import { sha256 } from "@noble/hashes/sha256";
import { bytesToHex } from "@noble/hashes/utils";
export const segment = encodeURIComponent;
export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
const meanings: Record<string, string> = {
  device_offline: "设备已离线",
  session_changed: "设备会话已更换，请刷新",
  capacity_exhausted: "服务器或维护端口池容量不足",
  idempotency_capacity: "服务器幂等账本已满",
  conflict: "操作冲突，请核对当前资源",
  incompatible: "产物不兼容或无法唯一选择",
  maintenance_disabled: "服务器未启用远程维护",
  not_found: "资源不存在或服务器已重启",
  invalid_request: "输入不符合 API 契约",
  operation_timeout: "服务器操作超时",
  origin_denied: "服务器拒绝跨域访问，请使用同源部署",
};
export function describe(e: unknown) {
  return e instanceof ApiError
    ? `${meanings[e.code] ?? "API 业务错误"} [${e.code}, HTTP ${e.status}] ${e.message}`
    : e instanceof TypeError
      ? "网络连接失败，请检查服务器地址与网络"
      : e instanceof Error
        ? e.message
        : String(e);
}
export function baseUrl(value: string) {
  const u = new URL(value);
  if (
    !["http:", "https:"].includes(u.protocol) ||
    u.username ||
    u.password ||
    u.pathname !== "/" ||
    u.search ||
    u.hash
  )
    throw new Error("服务器地址须为 http(s)://主机:端口，不含账号、路径或查询");
  return u.origin;
}
export function leaseBody(device_id: string, minutes: string) {
  if (minutes === "240") return { device_id };
  const m = /^(\d+)(?:\.(\d{1,8}))?$/.exec(minutes);
  if (!m) throw new Error("请输入正租期（分钟）");
  const scale = 10n ** BigInt(m[2]?.length ?? 0);
  const numerator = (BigInt(m[1]) * scale + BigInt(m[2] ?? 0)) * 60000n;
  if (numerator % scale !== 0n) throw new Error("租期必须为整数毫秒");
  const ms = numerator / scale;
  if (ms < 1n || ms > 9223372036854n) throw new Error("租期超出可表示范围");
  return { device_id, lease_ms: Number(ms) };
}
export interface Mutation {
  label: string;
  method: string;
  path: string;
  body: string | Blob;
  key: string;
  hash?: string;
}
export function mutation(
  label: string,
  path: string,
  body: unknown = {},
  method = "POST",
): Mutation {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return {
    label,
    path,
    method,
    body: JSON.stringify(body),
    key: bytesToHex(bytes),
  };
}
export async function hashBlob(blob: Blob) {
  const h = sha256.create();
  for (let offset = 0; offset < blob.size; offset += 1048576)
    h.update(
      new Uint8Array(await blob.slice(offset, offset + 1048576).arrayBuffer()),
    );
  return bytesToHex(h.digest());
}
export async function importMutation(file: File) {
  if (file.size > 2 ** 30) throw new Error("文件超过 1 GiB 导入上限");
  return {
    ...mutation("导入文件", "assets?name=" + segment(file.name)),
    body: file,
    hash: await hashBlob(file),
  };
}
export class ApiClient {
  constructor(
    readonly base: string,
    readonly signal: AbortSignal,
  ) {}
  async request(path: string, init: RequestInit = {}, raw = false) {
    const deadline = AbortSignal.timeout(raw ? 300000 : 40000);
    const r = await fetch(`${this.base}/api/v1/${path}`, {
      ...init,
      redirect: "error",
      credentials: "omit",
      cache: "no-store",
      signal: AbortSignal.any([this.signal, deadline, ...(init.signal ? [init.signal] : [])]),
    });
    if (raw && r.ok) return r.blob();
    const envelope = await r.json();
    if (!r.ok) {
      if (!envelope.error?.code) throw new Error("API 错误响应无法解析");
      throw new ApiError(envelope.error.code, envelope.error.message, r.status);
    }
    if (!("data" in envelope)) throw new Error("API 响应缺少 data");
    return envelope.data;
  }
  get<T>(path: string, signal?: AbortSignal): Promise<T> {
    return this.request(path, {signal});
  }
  async list<T>(path: string): Promise<T[]> {
    const items: T[] = [];
    for (let offset = 0; ; ) {
      const p = await this.get<{ items: T[]; total: number }>(
        `${path}${path.includes("?") ? "&" : "?"}offset=${offset}&limit=200`,
      );
      items.push(...p.items);
      offset += p.items.length;
      if (offset >= p.total || !p.items.length) return items;
      if (offset >= 10000) throw new Error("列表超过 10000 项，快照未完成");
    }
  }
  execute(m: Mutation) {
    return this.request(m.path, {
      method: m.method,
      body: m.body,
      headers: {
        "Idempotency-Key": m.key,
        "Content-Type": m.hash
          ? "application/octet-stream"
          : "application/json",
        ...(m.hash ? { "X-Content-SHA256": m.hash } : {}),
      },
    });
  }
  async content(asset: { asset_id: string; size: number; sha256: string }) {
    const blob = (await this.request(
      `assets/${segment(asset.asset_id)}/content`,
      {},
      true,
    )) as Blob;
    if (blob.size !== asset.size || (await hashBlob(blob)) !== asset.sha256)
      throw new Error("文件长度或 SHA-256 校验失败");
    return blob;
  }
}
