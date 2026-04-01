import { AppDetail, LocalizationEntry, ScreenshotSet } from "../../types";
import { shellQuote } from "../../utils";

type AppInfoViewProps = {
  appDetail: AppDetail;
  selectedAppId: string | null;
  metadataLoading: boolean;
  metadataError: string;
  allLocalizations: LocalizationEntry[];
  selectedLocale: string;
  screenshotsLoading: boolean;
  screenshotsError: string;
  screenshotSets: ScreenshotSet[];
  onLocaleChange: (locale: string) => void;
  onRunCommand: (cmd: string) => Promise<{ error?: string; data: string }>;
};

export function AppInfoView({
  appDetail,
  selectedAppId,
  metadataLoading,
  metadataError,
  allLocalizations,
  selectedLocale,
  screenshotsLoading,
  screenshotsError,
  screenshotSets,
  onLocaleChange,
  onRunCommand,
}: AppInfoViewProps) {
  return (
    <div className="app-detail-view">
      {/* Header */}
      <div className="app-detail-header">
        <div className="app-detail-header-row">
          <div>
            <p className="app-detail-name">{appDetail.name}</p>
            {appDetail.subtitle && <p className="app-detail-subtitle">{appDetail.subtitle}</p>}
          </div>
          <button
            className="submit-review-btn"
            type="button"
              onClick={() => {
                if (selectedAppId) {
                onRunCommand(`review submissions-list --app ${shellQuote(selectedAppId)} --output json`)
                  .then((res) => {
                    if (res.error) { alert(res.error); return; }
                    try {
                      const d = JSON.parse(res.data);
                      const items = d.data ?? [];
                      const pending = items.find((i: Record<string, unknown>) => {
                        const attrs = i.attributes as Record<string, string> | undefined;
                        return attrs?.state === "READY_FOR_REVIEW" || attrs?.state === "WAITING_FOR_REVIEW";
                      });
                      if (pending) {
                        alert(`Already submitted: ${(pending.attributes as Record<string, string>)?.state}`);
                      } else {
                        alert("No pending submission. Use ACP chat to run: asc submit --app " + selectedAppId);
                      }
                    } catch { alert("Could not parse submission status"); }
                  });
              }
            }}
          >
            Submit for Review
          </button>
        </div>
      </div>

      {/* General info */}
      <div className="app-detail-section">
        <h3 className="section-label">General</h3>
        <div className="env-grid">
          <div className="env-row">
            <span className="env-key">Bundle ID</span>
            <span className="env-value mono">{appDetail.bundleId}</span>
          </div>
          <div className="env-row">
            <span className="env-key">SKU</span>
            <span className="env-value mono">{appDetail.sku}</span>
          </div>
          <div className="env-row">
            <span className="env-key">Primary locale</span>
            <span className="env-value">{appDetail.primaryLocale}</span>
          </div>
        </div>
      </div>

      {/* App Store metadata */}
      {metadataLoading ? (
        <div className="app-detail-section">
          <p className="empty-hint">Loading metadata…</p>
        </div>
      ) : metadataError ? (
        <div className="app-detail-section">
          <h3 className="section-label">App Store Metadata</h3>
          <p className="empty-hint">{metadataError}</p>
        </div>
      ) : allLocalizations.length > 0 ? (() => {
        const loc = allLocalizations.find((l) => l.locale === selectedLocale) ?? allLocalizations[0];
        return (
          <div className="app-detail-section">
            <div className="metadata-header">
              <h3 className="section-label" style={{ margin: 0 }}>App Store Metadata</h3>
              <select
                className="locale-picker"
                aria-label="Select locale"
                value={selectedLocale}
                onChange={(e) => onLocaleChange(e.target.value)}
              >
                {allLocalizations.map((l) => (
                  <option key={l.locale} value={l.locale}>{l.locale}</option>
                ))}
              </select>
            </div>

            {/* Screenshots */}
            {screenshotsLoading ? (
              <div className="metadata-field">
                <p className="metadata-label">Screenshots</p>
                <p className="empty-hint" style={{ margin: 0 }}>Loading…</p>
              </div>
            ) : screenshotsError ? (
              <div className="metadata-field">
                <p className="metadata-label">Screenshots</p>
                <p className="empty-hint" style={{ margin: 0 }}>{screenshotsError}</p>
              </div>
            ) : screenshotSets.length > 0 ? (
              <div className="metadata-field">
                <p className="metadata-label">Screenshots</p>
                {screenshotSets.map((set) => {
                  const label = set.displayType
                    .replace(/^APP_/, "")
                    .replace(/_/g, " ")
                    .replace(/\b\w/g, (c) => c.toUpperCase());
                  return (
                    <div key={set.displayType} className="screenshot-set">
                      <p className="screenshot-set-label">{label}</p>
                      <div className="screenshot-row">
                        {set.screenshots.map((s, i) => (
                          <img
                            key={i}
                            src={s.thumbnailUrl}
                            alt={`Screenshot ${i + 1}`}
                            className={`screenshot-thumb ${s.width > s.height ? "landscape" : ""}`}
                          />
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : null}

            {loc.promotionalText && (
              <div className="metadata-field">
                <p className="metadata-label">Promotional Text</p>
                <p className="metadata-value">{loc.promotionalText}</p>
              </div>
            )}
            {loc.description && (
              <div className="metadata-field">
                <p className="metadata-label">Description</p>
                <p className="metadata-value metadata-multiline">{loc.description}</p>
              </div>
            )}
            {loc.whatsNew && (
              <div className="metadata-field">
                <p className="metadata-label">What's New</p>
                <p className="metadata-value metadata-multiline">{loc.whatsNew}</p>
              </div>
            )}
            {loc.keywords && (
              <div className="metadata-field">
                <p className="metadata-label">Keywords</p>
                <p className="metadata-value mono">{loc.keywords}</p>
              </div>
            )}
            {(loc.supportUrl || loc.marketingUrl) && (
              <div className="metadata-field">
                <p className="metadata-label">URLs</p>
                {loc.supportUrl && <p className="metadata-value mono">{loc.supportUrl}</p>}
                {loc.marketingUrl && <p className="metadata-value mono">{loc.marketingUrl}</p>}
              </div>
            )}
          </div>
        );
      })() : null}
    </div>
  );
}
