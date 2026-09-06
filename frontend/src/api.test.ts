import { afterEach, describe, it, expect, vi } from "vitest";
import {
  ApiClient,
  ApiError,
  baseUrl,
  hashBlob,
  leaseBody,
  mutation,
} from "./api";
import { Connection } from "./connection";
afterEach(() => vi.unstubAllGlobals());
describe("API contract migration", () => {
  it("keeps default lease omitted and accepts custom positive duration without a 360-minute cap", () => {
    expect(leaseBody("d", "240")).toEqual({ device_id: "d" });
    expect(leaseBody("d", "1000")).toEqual({
      device_id: "d",
      lease_ms: 60000000,
    });
    expect(leaseBody("d", "0.005")).toEqual({ device_id: "d", lease_ms: 300 });
    for (const v of ["0", "-1", "0.00000001", "153722868"])
      expect(() => leaseBody("d", v)).toThrow();
  });
  it("validates configured origins", () => {
    expect(baseUrl("http://localhost:8080")).toBe("http://localhost:8080");
    for (const v of [
      "file:///x",
      "http://user@host",
      "https://host/api",
      "https://host/?x",
    ])
      expect(() => baseUrl(v)).toThrow();
  });
  it("preserves exact key and bytes after uncertain response and rejects concurrent new mutations", async () => {
    let release: (v: any) => void = () => {};
    const fetch = vi
      .fn()
      .mockImplementationOnce(
        () => new Promise((resolve) => (release = resolve)),
      )
      .mockRejectedValueOnce(new TypeError("lost"))
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { task_id: "same" } })),
      );
    vi.stubGlobal("fetch", fetch);
    const c = new Connection("http://localhost", () => {});
    const m = mutation("exec", "tasks", { command: "echo x", device_id: "d" });
    const first = c.execute(m);
    await Promise.resolve();
    await expect(c.execute(mutation("new", "tasks"))).rejects.toThrow(
      "操作正在进行",
    );
    release(new Response("not-json"));
    await expect(first).rejects.toThrow();
    expect(c.state.pending).toBe(m);
    await expect(c.execute(mutation("new", "tasks"))).rejects.toThrow("原请求");
    await expect(c.execute(m, true)).rejects.toThrow();
    await c.execute(m, true);
    expect(c.state.pending).toBeNull();
    expect(fetch.mock.calls[0][1].body).toBe(fetch.mock.calls[2][1].body);
    expect(fetch.mock.calls[0][1].headers["Idempotency-Key"]).toBe(
      fetch.mock.calls[2][1].headers["Idempotency-Key"],
    );
    await c.dispose();
  });
  it("maps definitive business errors separately from transport uncertainty", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            error: { code: "device_offline", message: "offline" },
          }),
          { status: 409 },
        ),
      ),
    );
    const c = new Connection("http://localhost", () => {});
    await expect(c.execute(mutation("exec", "tasks"))).rejects.toBeInstanceOf(
      ApiError,
    );
    expect(c.state.pending).toBeNull();
    await c.dispose();
  });
  it("paginates snapshots without silently truncating", async () => {
    const f = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ data: { items: [{ id: 1 }], total: 2 } }),
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ data: { items: [{ id: 2 }], total: 2 } }),
        ),
      );
    vi.stubGlobal("fetch", f);
    const a = new ApiClient("http://localhost", new AbortController().signal);
    expect(await a.list("devices")).toHaveLength(2);
    expect(f.mock.calls[1][0]).toContain("offset=1");
  });
  it("checks downloaded content integrity", async () => {
    expect(await hashBlob(new Blob(["abc"]))).toBe(
      "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
    );
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("corrupt")));
    const a = new ApiClient("http://localhost", new AbortController().signal);
    await expect(
      a.content({ asset_id: "asset", size: 3, sha256: "bad" }),
    ).rejects.toThrow("校验失败");
  });
});
