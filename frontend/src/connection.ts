import { ApiClient, ApiError, describe, type Mutation } from "./api";
import { emptySnapshot, type Snapshot } from "./models";
export interface ConnectionState {
  snapshot: Snapshot;
  status: string;
  synchronized: boolean;
  pending: Mutation | null;
  busy: boolean;
  error: string;
}
export class Connection {
  readonly lifetime = new AbortController();
  readonly api: ApiClient;
  state: ConnectionState = {
    snapshot: emptySnapshot(),
    status: "正在连接…",
    synchronized: false,
    pending: null,
    busy: false,
    error: "",
  };
  private socket?: WebSocket;
  private timer?: ReturnType<typeof setTimeout>;
  private periodic?: ReturnType<typeof setInterval>;
  private firstTimer?: ReturnType<typeof setTimeout>;
  private generation = 0;
  private ready = false;
  private dirty = false;
  private refreshing = false;
  private failures = 0;
  private active = new Set<Promise<unknown>>();
  private closing?: Promise<void>;
  constructor(
    base: string,
    private publish: (s: ConnectionState) => void,
  ) {
    this.api = new ApiClient(base, this.lifetime.signal);
  }
  private emit(p: Partial<ConnectionState>) {
    if (this.lifetime.signal.aborted) return;
    this.state = { ...this.state, ...p };
    this.publish(this.state);
  }
  start() {
    this.connect();
    this.periodic = setInterval(() => this.invalidate(), 5000);
  }
  private connect() {
    if (this.lifetime.signal.aborted) return;
    const ws = new WebSocket(
      this.api.base.replace(/^http/, "ws") + "/api/v1/events",
    );
    this.socket = ws;
    let first = true;
    this.firstTimer = setTimeout(() => ws.close(), 10000);
    ws.onmessage = (e) => {
      try {
        if (typeof e.data !== "string" || e.data.length > 8192) throw Error();
        const event = JSON.parse(e.data);
        if (first && event.type !== "resync_required") throw Error();
        if (event.type === "resync_required") {
          first = false;
          clearTimeout(this.firstTimer);
          this.generation++;
          this.ready = true;
          this.failures = 0;
          this.emit({ status: "正在同步 HTTP 快照…", synchronized: false });
          this.invalidate();
        } else if (event.type === "resource_changed") this.invalidate();
      } catch {
        ws.close();
      }
    };
    ws.onclose = () => {
      clearTimeout(this.firstTimer);
      this.ready = false;
      this.generation++;
      this.emit({ synchronized: false, status: "连接中断，正在重连…" });
      if (!this.lifetime.signal.aborted)
        this.timer = setTimeout(
          () => this.connect(),
          Math.min(30000, 1000 * 2 ** this.failures++),
        );
    };
    ws.onerror = () => ws.close();
  }
  invalidate() {
    this.dirty = true;
    if (!this.refreshing && this.ready && !this.lifetime.signal.aborted) {
      this.refreshing = true;
      void this.track(() => this.refresh());
    }
  }
  private async refresh() {
    this.refreshing = true;
    try {
      while (this.dirty && this.ready && !this.lifetime.signal.aborted) {
        this.dirty = false;
        const generation = this.generation;
        try {
          const devices =
            await this.api.list<Snapshot["devices"][number]>("devices");
          const tasks = await this.api.list<Snapshot["tasks"][number]>("tasks");
          const assets = await this.api.list<Snapshot["assets"][number]>(
            "assets?include_archived=true",
          );
          const tools = await this.api.list<Snapshot["tools"][number]>(
            "tools?include_archived=true",
          );
          let maintenance: Snapshot["maintenance"] = [],
            maintenanceError: string | undefined;
          try {
            maintenance = await this.api.list("maintenance");
          } catch (e) {
            if (e instanceof ApiError && e.code === "maintenance_disabled")
              maintenanceError = describe(e);
            else throw e;
          }
          if (this.ready && generation === this.generation)
            this.emit({
              snapshot: {
                devices,
                tasks,
                assets,
                tools,
                maintenance,
                maintenanceError,
                fetchedAt: Date.now(),
              },
              status: "已连接",
              synchronized: true,
              error: "",
            });
        } catch (e) {
          this.emit({
            synchronized: false,
            status: "快照刷新失败，正在恢复…",
            error: describe(e),
          });
        }
      }
    } finally {
      this.refreshing = false;
    }
  }
  track<T>(action: () => Promise<T>): Promise<T> {
    if (this.lifetime.signal.aborted)
      return Promise.reject(new Error("连接已关闭"));
    const p = Promise.resolve().then(action);
    this.active.add(p);
    void p.then(
      () => this.active.delete(p),
      () => this.active.delete(p),
    );
    return p;
  }
  async execute(m: Mutation, retry = false) {
    if (this.state.busy) throw new Error("操作正在进行");
    if (this.state.pending && (!retry || m !== this.state.pending))
      throw new Error("请先处理响应不确定的原请求");
    this.emit({ busy: true, pending: m });
    try {
      const data = await this.track(() => this.api.execute(m));
      this.emit({ pending: null });
      this.invalidate();
      return data;
    } catch (e) {
      if (e instanceof ApiError) this.emit({ pending: null });
      throw e;
    } finally {
      this.emit({ busy: false });
    }
  }
  abandon() {
    if (this.state.busy) throw new Error("操作仍在进行");
    this.emit({ pending: null });
  }
  dispose() {
    if (this.closing) return this.closing;
    this.lifetime.abort();
    clearTimeout(this.timer);
    clearTimeout(this.firstTimer);
    clearInterval(this.periodic);
    const ws = this.socket;
    const closed = new Promise<void>((resolve) => {
      if (!ws || ws.readyState === WebSocket.CLOSED) return resolve();
      ws.addEventListener("close", () => resolve(), { once: true });
      ws.close();
    });
    this.closing = Promise.allSettled([...this.active, closed]).then(() => {});
    return this.closing;
  }
}
