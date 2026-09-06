import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import path from "node:path";
const out = path.resolve(process.argv[2] ?? "../build/react-shell/test-output");
await mkdir(out, { recursive: true });
let browser;
for (let attempt = 0; !browser; attempt++) {
  try {
    browser = await chromium.connectOverCDP("http://127.0.0.1:19222");
  } catch (e) {
    if (attempt >= 100) throw e;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
}
const page = browser.contexts()[0].pages()[0];
page.setDefaultTimeout(12000);
await page.waitForURL(/\/__workbench\/index\.html/, { timeout: 30000 });
await page.getByRole("heading", { name: "远程维护", exact: true }).waitFor();
await page.addInitScript(() => {
  window.__testSockets = [];
  const Original = window.WebSocket;
  window.WebSocket = class extends Original {
    constructor(...args) {
      super(...args);
      window.__testSockets.push(this);
    }
  };
});
const failures = [];
page.on("pageerror", (e) => failures.push(e.message));
let checks = 0;
function check(value, label) {
  if (!value) throw Error(label);
  checks++;
  console.log("PASS " + label);
}
const api = async (p) =>
  (await (await fetch("http://127.0.0.1:18080/api/v1/" + p)).json()).data;
const nav = (label) =>
  page
    .getByRole("navigation")
    .getByRole("button", { name: label, exact: true })
    .click();
const tab = (label) =>
  page.getByRole("tab", { name: label, exact: true }).click();
const submit = () =>
  page
    .getByRole("dialog")
    .getByRole("button", { name: "确定", exact: true })
    .click();
try {
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "远程维护", exact: true }),
  ).toBeVisible();
  check(true, "production React resources loaded in actual WebView2");
  await nav("系统设置");
  await page.getByRole("button", { name: "保存并连接" }).click();
  await expect(page.locator(".connection-text")).toContainText("已连接");
  await nav("设备列表");
  await expect(page.locator(".device-item")).toContainText("ui-device");
  check(true, "native profile bridge and real HTTP / WebSocket connection");
  const bridge = async (method, args = {}) =>
    page.evaluate(
      ({ method, args }) =>
        new Promise((resolve) => {
          const id = "99001";
          const handler = (e) => {
            if (e.data.id === id) {
              window.chrome.webview.removeEventListener("message", handler);
              resolve(e.data);
            }
          };
          window.chrome.webview.addEventListener("message", handler);
          window.chrome.webview.postMessage({ id, method, args });
        }),
      { method, args },
    );
  check(
    (await bridge("exec", { command: "calc" })).error,
    "arbitrary command bridge denied",
  );
  check(
    (
      await bridge("openEndpoint", {
        service: "web",
        host: "localhost",
        port: 80,
        address: "localhost:80",
        state: "ready",
        url: "file:///C:/test",
      })
    ).error,
    "illegal endpoint denied",
  );
  const nativeProfile = (await bridge("getProfile")).result;
  check(
    (
      await bridge("saveProfile", {
        ...nativeProfile,
        ssh_executable: "C:/Windows/System32/cmd.exe",
      })
    ).error,
    "script cannot configure arbitrary executable",
  );
  const local = page.url();
  await page.evaluate(() => {
    location.href = "https://example.com/";
  });
  await expect(page).toHaveURL(local);
  check(true, "external navigation denied");
  // Start from no active lease to make this scenario reproducible.
  for (const m of (await api("maintenance")).items.filter((m) => !m.released))
    await fetch(
      `http://127.0.0.1:18080/api/v1/maintenance/${m.maintenance_id}/close`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
        },
        body: "{}",
      },
    );
  await expect(
    page.getByRole("button", { name: "开启维护", exact: true }),
  ).toBeEnabled();
  await page
    .getByRole("button", { name: "开启维护", exact: true })
    .evaluate((b) => {
      b.click();
      b.click();
    });
  await expect(page.locator(".maintenance-card .badge")).toHaveText("已开启");
  let ms = (await api("maintenance")).items;
  const m = ms.find((x) => !x.released);
  check(
    ms.filter((x) => !x.released).length === 1,
    "duplicate clicks create one maintenance",
  );
  check(
    new Date(m.expires_at) - new Date(m.created_at) === 14400000,
    "default lease exactly 240 minutes",
  );
  check(m.endpoints.length === 3, "Web SSH Telnet endpoints present");
  await page.screenshot({ path: path.join(out, "overview-light.png") });
  await page.locator(".endpoint").nth(0).getByRole("button").click();
  await page.locator(".endpoint").nth(1).getByRole("button").click();
  await page.locator(".endpoint").nth(2).getByRole("button").click();
  await expect
    .poll(async () => await readFile(path.join(out, "launches.txt"), "utf8"))
    .toContain("web\nssh\ntelnet\n");
  check(
    true,
    "three entry buttons recheck API then invoke the native launch boundary",
  );
  await page.getByRole("button", { name: "切换主题" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({ path: path.join(out, "overview-dark.png") });
  await page.getByRole("button", { name: "切换主题" }).click();
  check(true, "light and dark themes");
  const cdp = await browser.contexts()[0].newCDPSession(page);
  await cdp.send("Emulation.setDeviceMetricsOverride", {
    width: 960,
    height: 720,
    deviceScaleFactor: 2,
    mobile: false,
  });
  await page.screenshot({ path: path.join(out, "overview-960-dpi2.png") });
  check(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "960px at scale 2 has no page overflow",
  );
  await cdp.send("Emulation.clearDeviceMetricsOverride");
  await page
    .getByRole("button", { name: "新建 Exec 任务", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByLabel("命令", { exact: true })
    .fill("echo shared-frontend");
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "结果", exact: true }).click();
  await expect(page.locator(".terminal")).toContainText("中文结果");
  check(true, "Exec / ACK / final result through shared UI");
  await page.screenshot({ path: path.join(out, "tasks.png") });
  await tab("文件管理");
  const file = path.join(out, "fixture.txt");
  await writeFile(file, "shared frontend file\n中文");
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "导入文件", exact: true }).click();
  await (await chooser).setFiles(file);
  await expect(page.locator("tbody")).toContainText("fixture.txt");
  check(true, "native file selection and raw upload");
  const saved = path.join(out, "saved.txt");
  await page.getByRole("button", { name: "另存", exact: true }).click();
  await promisify(execFile)(
    "powershell.exe",
    [
      "-NoProfile",
      "-ExecutionPolicy",
      "Bypass",
      "-File",
      path.resolve("../windows/RouterWorkbench.Tests/select-file.ps1"),
      "-ProcessId",
      process.argv[3],
      "-TargetPath",
      saved,
      "-Title",
      "另存文件",
    ],
    { windowsHide: true },
  );
  await expect
    .poll(async () => await readFile(saved, "utf8").catch(() => ""))
    .toBe("shared frontend file\n中文");
  check(true, "Windows save dialog and content bytes match");
  await page.getByRole("button", { name: "上传", exact: true }).click();
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect
    .poll(async () =>
      (await api("tasks")).items.some(
        (t) => t.type === "upload" && t.state === "success",
      ),
    )
    .toBe(true);
  check(true, "asset upload task success");
  await tab("文件管理");
  await page.getByRole("button", { name: "从设备下载", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByLabel("设备文件路径")
    .fill("/tmp/fixture.txt");
  await page
    .getByRole("dialog")
    .getByLabel("仓库文件名称")
    .fill("downloaded.txt");
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect
    .poll(async () =>
      (await api("tasks")).items.some(
        (t) => t.type === "download" && t.state === "success",
      ),
    )
    .toBe(true);
  const dt = (await api("tasks")).items.find((t) => t.type === "download");
  await page
    .locator("tbody tr")
    .filter({ hasText: dt.task_id })
    .getByRole("button", { name: "结果" })
    .click();
  await expect(
    page.getByRole("button", { name: "导入已提交下载" }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "导入已提交下载" }).click();
  await expect(
    page.getByRole("dialog", { name: "下载导入结果" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "关闭详情" }).click();
  check(
    (await api("assets")).items.some((a) => a.name === "downloaded.txt"),
    "download committed/released and explicit stable asset import",
  );
  await tab("工具 / 版本");
  await page.getByRole("button", { name: "创建工具" }).click();
  await page.getByRole("dialog").getByLabel("工具名称").fill("网络诊断工具");
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "发布版本" }).click();
  await page.getByRole("dialog").getByLabel("版本标签").fill("1.0");
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator(".artifact")).toContainText("兼容");
  await page.getByRole("button", { name: "投放", exact: true }).click();
  await submit();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect
    .poll(
      async () =>
        (await api("tasks")).items.filter(
          (t) => t.type === "upload" && t.state === "success",
        ).length,
    )
    .toBe(2);
  check(true, "tool/version/artifact/compatibility/deployment real API flow");
  await tab("工具 / 版本");
  await page.screenshot({ path: path.join(out, "tools.png") });
  await tab("概览");
  const before = (await api("devices")).items[0].current_session.session_id;
  await cdp.send("Network.enable");
  await cdp.send("Network.emulateNetworkConditions", {
    offline: true,
    latency: 0,
    downloadThroughput: 0,
    uploadThroughput: 0,
  });
  await page.evaluate(() => window.__testSockets.forEach((s) => s.close()));
  await expect(page.locator(".connection-text")).toContainText("连接中断");
  await fetch("http://127.0.0.1:18082/replace");
  await cdp.send("Network.emulateNetworkConditions", {
    offline: false,
    latency: 0,
    downloadThroughput: -1,
    uploadThroughput: -1,
  });
  await expect(page.locator(".connection-text")).toContainText("已连接", {
    timeout: 20000,
  });
  await expect(page.locator(".maintenance-card .badge")).toHaveText("已关闭", {
    timeout: 20000,
  });
  check(
    (await api("devices")).items[0].current_session.session_id !== before,
    "Session replacement and HTTP snapshot recovery",
  );
  check(
    await page.evaluate(() => window.__testSockets.length >= 2),
    "WebSocket closed and a new socket connected",
  );
  await page.getByLabel("维护时长（分钟）").fill("0.005");
  await page.getByRole("button", { name: "开启维护", exact: true }).click();
  await expect
    .poll(async () =>
      (await api("maintenance")).items.some(
        (x) =>
          new Date(x.expires_at) - new Date(x.created_at) === 300 && x.released,
      ),
    )
    .toBe(true);
  check(true, "custom 300ms lease expires on server");
  await page.getByLabel("维护时长（分钟）").fill("1000");
  await page.getByRole("button", { name: "开启维护", exact: true }).click();
  await expect(page.locator(".maintenance-card .badge")).toHaveText("已开启");
  check(
    (await api("maintenance")).items.some(
      (x) => new Date(x.expires_at) - new Date(x.created_at) === 60000000,
    ),
    "custom lease above 360 minutes",
  );
  await page.getByRole("button", { name: "关闭维护", exact: true }).click();
  await expect(page.locator(".maintenance-card .badge")).toHaveText("已关闭");
  check(true, "explicit maintenance close");
  await nav("系统设置");
  await page.getByLabel("管理服务器地址").fill("http://127.0.0.1:18083");
  await page.getByRole("button", { name: "保存并连接" }).click();
  await expect(page).toHaveURL(/18083/);
  await expect(page.locator(".device-item")).toHaveCount(0);
  check(true, "server switch unloads old device/task state");
  await nav("系统设置");
  await page.getByLabel("管理服务器地址").fill("http://127.0.0.1:18080");
  await page.getByRole("button", { name: "保存并连接" }).click();
  await expect(page).toHaveURL(/18080/);
  await expect(page.locator(".connection-text")).toContainText("已连接", {
    timeout: 20000,
  });
  check(true, "switch back reconnects new owner");
  check(!failures.length, "no React runtime errors");
  await writeFile(
    path.join(out, "ui-result.txt"),
    `PASS ${checks} native WebView2 integration checks\n`,
  );
} catch (e) {
  await page
    .screenshot({ path: path.join(out, "failure.png") })
    .catch(() => {});
  await writeFile(
    path.join(out, "failure.txt"),
    String(e) + "\n" + (await page.locator("body").innerText()),
  );
  throw e;
} finally {
  await browser.close();
}
