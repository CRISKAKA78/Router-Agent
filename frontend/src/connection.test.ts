import { afterEach, it, expect, vi } from "vitest";
import { Connection } from "./connection";
class Socket extends EventTarget {
  static CLOSED = 3;
  static all: Socket[] = [];
  readyState = 1;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor() {
    super();
    Socket.all.push(this);
  }
  message(type: string) {
    this.onmessage?.({ data: JSON.stringify({ type }) });
  }
  close() {
    if (this.readyState === 3) return;
    this.readyState = 3;
    this.onclose?.();
    this.dispatchEvent(new Event("close"));
  }
}
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  Socket.all = [];
});
const response = () =>
  new Response(JSON.stringify({ data: { items: [], total: 0 } }));
it("keeps invalidations received during a blocked snapshot and publishes only after resync", async () => {
  vi.stubGlobal("WebSocket", Socket);
  let release: (r: Response) => void = () => {};
  let calls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(() => {
      calls++;
      return calls === 1
        ? new Promise<Response>((r) => (release = r))
        : Promise.resolve(response());
    }),
  );
  const c = new Connection("http://localhost", () => {});
  c.start();
  expect(calls).toBe(0);
  Socket.all[0].message("resync_required");
  await vi.waitFor(() => expect(calls).toBe(1));
  Socket.all[0].message("resource_changed");
  Socket.all[0].message("resource_changed");
  release(response());
  await vi.waitFor(() => expect(calls).toBe(10));
  expect(c.state.synchronized).toBe(true);
  await c.dispose();
});
it("cancels blocked HTTP and joins the socket before disposal resolves without late publication", async () => {
  vi.stubGlobal("WebSocket", Socket);
  const publish = vi.fn();
  vi.stubGlobal(
    "fetch",
    vi.fn(
      (_url, init) =>
        new Promise<Response>((_resolve, reject) =>
          init.signal.addEventListener("abort", () =>
            reject(new DOMException("Aborted", "AbortError")),
          ),
        ),
    ),
  );
  const c = new Connection("http://localhost", publish);
  c.start();
  Socket.all[0].message("resync_required");
  await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
  const before = publish.mock.calls.length;
  const closing = c.dispose();
  expect(c.dispose()).toBe(closing);
  await closing;
  expect(Socket.all[0].readyState).toBe(3);
  expect(publish).toHaveBeenCalledTimes(before);
  await expect(c.track(async () => 1)).rejects.toThrow("连接已关闭");
});
it("rejects an invalid first event and reconnects with a new full snapshot", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("WebSocket", Socket);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => response()),
  );
  const c = new Connection("http://localhost", () => {});
  c.start();
  Socket.all[0].message("resource_changed");
  expect(c.state.synchronized).toBe(false);
  await vi.advanceTimersByTimeAsync(1000);
  expect(Socket.all).toHaveLength(2);
  Socket.all[1].message("resync_required");
  await vi.advanceTimersByTimeAsync(1);
  expect(fetch).toHaveBeenCalledTimes(5);
  expect(c.state.synchronized).toBe(true);
  await c.dispose();
});
