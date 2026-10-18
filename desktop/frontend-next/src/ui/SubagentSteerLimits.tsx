import { useEffect, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { t } from "../i18n";
import { reason } from "../i18n/kernel";
import type { AgentPort } from "../port/port";
import type { SubagentSteerSettings } from "../port/boundary";

type Field = "maxBytes" | "perMinute";

// What can be said to a sub-agent that is still running is bounded in size and
// in rate, and both bounds are the person's to move. Zero lifts a bound.
export function SubagentSteerLimits({ port }: { port: AgentPort }) {
  const [state, setState] = useState<SubagentSteerSettings | null>(null);
  const [draft, setDraft] = useState({ maxBytes: "", perMinute: "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const adopt = (s: SubagentSteerSettings) => {
    setState(s);
    setDraft({ maxBytes: String(s.maxBytes), perMinute: String(s.perMinute) });
  };

  useEffect(() => {
    port.subagentSteerSettings().then(adopt).catch(() => setState(null));
  }, [port]);

  if (!state) return <div className="empty">{t("无法读取子代理指示的上限设置。")}</div>;

  const commit = async (field: Field) => {
    const next = { maxBytes: Number(draft.maxBytes), perMinute: Number(draft.perMinute) };
    if (next[field] === state[field]) return;
    setBusy(true);
    setError("");
    try {
      adopt(await port.saveSubagentSteerSettings(next.maxBytes, next.perMinute));
    } catch (e) {
      setError(reason(e));
    } finally {
      setBusy(false);
    }
  };

  const typed = (field: Field) => (e: ChangeEvent<HTMLInputElement>) => setDraft({ ...draft, [field]: e.target.value.replace(/\D/g, "") });
  const enter = (e: KeyboardEvent<HTMLInputElement>) => { if (e.key === "Enter") e.currentTarget.blur(); };

  const sizeLabel = t("单条指示的字节上限");
  const rateLabel = t("每分钟条数上限");
  return (
    <div className="box">
      <div className="lrow threshold-row">
        <span className="tx">
          <span className="lb">{sizeLabel}</span>
          <span className="ds">{t("默认 {n}。超过的指示会被拒绝，不会发给子代理；填 0 表示不设上限。", { n: state.defaultMaxBytes })}</span>
        </span>
        <div className="threshold-control">
          <span className="unit-field">
            <input type="text" inputMode="numeric" aria-label={sizeLabel} data-action-keydown="subagent-steer.max-bytes" data-action-change="subagent-steer.max-bytes" value={draft.maxBytes} disabled={busy} onChange={typed("maxBytes")} onBlur={() => void commit("maxBytes")} onKeyDown={enter} />
            <i>{t("字节")}</i>
          </span>
        </div>
      </div>
      <div className="lrow threshold-row">
        <span className="tx">
          <span className="lb">{rateLabel}</span>
          <span className="ds">{t("默认 {n}。发给同一个子代理的指示，一分钟内超过这个条数就会被拒绝；填 0 表示不设上限。", { n: state.defaultPerMinute })}</span>
        </span>
        <div className="threshold-control">
          <span className="unit-field">
            <input type="text" inputMode="numeric" aria-label={rateLabel} data-action-keydown="subagent-steer.per-minute" data-action-change="subagent-steer.per-minute" value={draft.perMinute} disabled={busy} onChange={typed("perMinute")} onBlur={() => void commit("perMinute")} onKeyDown={enter} />
            <i>{t("条")}</i>
          </span>
        </div>
      </div>
      {error && <div className="why" role="alert">{error}</div>}
    </div>
  );
}
