import { useState, type ReactNode } from "react";
import { Home, X } from "lucide-react";
export const stateName = (s: string) =>
  ({
    online: "在线",
    offline: "离线",
    ready: "已开启",
    closed: "已关闭",
    closing: "关闭中",
    success: "成功",
    received: "待确认",
    failed: "失败",
    running: "运行中",
    queued: "排队中",
    timeout: "超时",
    rejected: "已拒绝",
    compatible: "兼容",
    incompatible: "不兼容",
    unknown: "未知",
  })[s] ?? s;
export function Badge({ state }: { state: string }) {
  return (
    <span
      className={
        "badge " +
        (["online", "ready", "success", "compatible"].includes(state)
          ? "good"
          : ["closed", "offline", "unknown"].includes(state)
            ? "muted"
            : "warn")
      }
    >
      <i />
      {stateName(state)}
    </span>
  );
}
export function Card({
  title,
  icon: Icon,
  children,
  action,
  className = "",
}: {
  title: string;
  icon: typeof Home;
  children: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <section className={"card " + className}>
      <div className="card-heading">
        <h3>
          <Icon size={20} />
          {title}
        </h3>
        {action}
      </div>
      {children}
    </section>
  );
}

export function FormDialog({
  title,
  fields,
  busy,
  error,
  onClose,
  onSubmit,
}: {
  title: string;
  fields: { key: string; label: string; value: string; multiline?: boolean }[];
  busy: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (v: Record<string, string>) => void;
}) {
  const [values, setValues] = useState(
    Object.fromEntries(fields.map((f) => [f.key, f.value])),
  );
  return (
    <div className="modal-backdrop">
      <form
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onKeyDown={(e) => {
          if (e.key === "Escape" && !busy) {
            e.preventDefault();
            onClose();
          }
          if (e.key === "Tab") {
            const fields = Array.from(
              e.currentTarget.querySelectorAll<HTMLElement>(
                "button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled)",
              ),
            );
            const first = fields[0],
              last = fields.at(-1);
            if (e.shiftKey && document.activeElement === first) {
              e.preventDefault();
              last?.focus();
            } else if (!e.shiftKey && document.activeElement === last) {
              e.preventDefault();
              first?.focus();
            }
          }
        }}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit(values);
        }}
      >
        <div className="card-heading">
          <h2>{title}</h2>
          <button
            type="button"
            aria-label="取消"
            disabled={busy}
            onClick={onClose}
          >
            <X />
          </button>
        </div>
        {fields.map((f, i) => (
          <label key={f.key}>
            {f.label}
            {f.multiline ? (
              <textarea
                aria-label={f.label}
                autoFocus={i === 0}
                value={values[f.key]}
                onChange={(e) =>
                  setValues({ ...values, [f.key]: e.target.value })
                }
              />
            ) : (
              <input
                aria-label={f.label}
                autoFocus={i === 0}
                value={values[f.key]}
                onChange={(e) =>
                  setValues({ ...values, [f.key]: e.target.value })
                }
              />
            )}
          </label>
        ))}
        {error && (
          <p className="alert error" role="alert">
            {error}
          </p>
        )}
        <div className="modal-actions">
          <button type="button" disabled={busy} onClick={onClose}>
            取消
          </button>
          <button className="primary" disabled={busy} type="submit">
            {busy ? "处理中…" : "确定"}
          </button>
        </div>
      </form>
    </div>
  );
}
