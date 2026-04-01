import { LocalizationEntry, ScreenshotSet } from "../../types";
import { screenshotLabel } from "../../utils";

type ScreenshotsViewProps = {
  screenshotsLoading: boolean;
  screenshotsError: string;
  screenshotSets: ScreenshotSet[];
  allLocalizations: LocalizationEntry[];
  selectedLocale: string;
  onLocaleChange: (locale: string) => void;
};

export function ScreenshotsView({
  screenshotsLoading,
  screenshotsError,
  screenshotSets,
  allLocalizations,
  selectedLocale,
  onLocaleChange,
}: ScreenshotsViewProps) {
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <h3 className="section-label">Screenshots</h3>
        {allLocalizations.length > 1 && (
          <div className="metadata-header" style={{ marginBottom: 12 }}>
            <span />
            <select className="locale-picker" value={selectedLocale} onChange={(e) => onLocaleChange(e.target.value)}>
              {allLocalizations.map((l) => <option key={l.locale} value={l.locale}>{l.locale}</option>)}
            </select>
          </div>
        )}
        {screenshotsLoading ? (
          <p className="empty-hint">Loading…</p>
        ) : screenshotsError ? (
          <p className="empty-hint">{screenshotsError}</p>
        ) : screenshotSets.length > 0 ? (
          <>
            {screenshotSets.map((set) => {
              const label = screenshotLabel(set.displayType);
              return (
                <div key={set.displayType} className="screenshot-set">
                  <p className="screenshot-set-label">{label}</p>
                  <div className="screenshot-row">
                    {set.screenshots.map((s, i) => (
                      <img key={i} src={s.thumbnailUrl} alt={`Screenshot ${i + 1}`} className={`screenshot-thumb ${s.width > s.height ? "landscape" : ""}`} />
                    ))}
                  </div>
                </div>
              );
            })}
          </>
        ) : (
          <p className="empty-hint">No screenshots found. Select an app with screenshots or change locale.</p>
        )}
      </div>
    </div>
  );
}
