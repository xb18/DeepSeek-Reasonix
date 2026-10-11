import { t } from "../i18n";
import { StudioIcon } from "./StudioIcon";

/** What an empty workbench offers. Opening the column is asking for a place to
 *  work, not for one particular tool, so the choice is the person's. */
export function WorkbenchLauncher({ changes, onFiles, onChanges, onBrowser }: { changes: number; onFiles: () => void; onChanges: () => void; onBrowser: () => void }) {
  return (
    <div className="workbench-launcher" role="group" aria-label={t("开始")}>
      <StudioIcon name="spark" />
      <button data-action="workbench.launch-files" onClick={onFiles}>
        <StudioIcon name="folder" />
        <b>{t("文件")}</b>
      </button>
      <button data-action="workbench.launch-changes" disabled={changes === 0} onClick={onChanges}>
        <StudioIcon name="branch" />
        <b>{t("改动")}</b>
        {changes > 0 && <small>{changes}</small>}
      </button>
      <button data-action="workbench.launch-browser" onClick={onBrowser}>
        <StudioIcon name="globe" />
        <b>{t("浏览器")}</b>
      </button>
    </div>
  );
}
