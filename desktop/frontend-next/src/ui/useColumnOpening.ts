import { useEffect, useRef, type Dispatch, type MutableRefObject, type SetStateAction } from "react";
import { reason } from "../i18n/kernel";
import { host } from "../port/host";
import type { AgentPort } from "../port/port";
import { pageForEmptyColumn, pageInFlight } from "./browser_open";

interface Opening {
  manual: boolean;
  /** Counts the title-bar Browser presses; a change asks for a browser, not just the column. */
  browserAsk: number;
  port: AgentPort;
  agentNow: MutableRefObject<string>;
  dropBlankStart: () => void;
  pick: (key: string) => void;
  setFailed: Dispatch<SetStateAction<string>>;
  setBrowsers: Dispatch<SetStateAction<string[]>>;
  setSelected: Dispatch<SetStateAction<string>>;
}

/** What the workbench column does when it is opened. It follows the agent's page,
 *  and a page a followed link already has on its way; with neither it stays on
 *  its launcher until a browser is asked for. The title bar's Browser button
 *  asks for one itself: each press is one ask, and a page already on its way is
 *  shared rather than requested twice. */
export function useColumnOpening({ manual, browserAsk, port, agentNow, dropBlankStart, pick, setFailed, setBrowsers, setSelected }: Opening) {
  useEffect(() => {
    if (!manual) return;
    if (agentNow.current) {
      dropBlankStart();
      pick(`browser:${agentNow.current}`);
      return;
    }
    pageInFlight(port)
      ?.then((tab) => pick(`browser:${tab.target}`))
      .catch((e) => setFailed(reason(e)));
  }, [manual, agentNow, dropBlankStart, pick, port, setFailed]);

  const asked = useRef(browserAsk);
  useEffect(() => {
    if (asked.current === browserAsk) return;
    asked.current = browserAsk;
    if (!manual) return;
    if (agentNow.current) {
      dropBlankStart();
      pick(`browser:${agentNow.current}`);
      return;
    }
    // A shell that draws views opens a view here too: a framed page is refused
    // by any site sending X-Frame-Options or frame-ancestors, as many do.
    if (host().drawsBrowserViews()) {
      pageForEmptyColumn(port)
        .then((tab) => pick(`browser:${tab.target}`))
        .catch((e) => setFailed(reason(e)));
      return;
    }
    setBrowsers((open) => (open.length ? open : ["b0"]));
    setSelected((at) => (at.startsWith("manual:") ? at : "manual:b0"));
  }, [browserAsk, manual, agentNow, dropBlankStart, pick, port, setFailed, setBrowsers, setSelected]);
}
