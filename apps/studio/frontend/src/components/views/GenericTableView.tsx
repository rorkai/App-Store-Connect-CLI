import { fieldLabels } from "../../constants";
import { NavSection, SectionCacheEntry } from "../../types";
import { compareBundleIDPlatforms, fmt, itemMatchesSearch, sectionRequiresApp } from "../../utils";

type GenericTableViewProps = {
  activeSection: NavSection;
  cache: SectionCacheEntry;
  bundleIDsPlatformSort: "asc" | "desc";
  activeSectionSearch: string;
  onSetSectionSearch: (sectionId: string, term: string) => void;
  onToggleBundleIDSort: () => void;
  onOpenBundleIDSheet: () => void;
  onOpenDeviceSheet: () => void;
};

export function GenericTableView({
  activeSection,
  cache,
  bundleIDsPlatformSort,
  activeSectionSearch,
  onSetSectionSearch,
  onToggleBundleIDSort,
  onOpenBundleIDSheet,
  onOpenDeviceSheet,
}: GenericTableViewProps) {
  if (!cache || cache.loading) {
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <h3 className="section-label">{activeSection.label}</h3>
          <p className="empty-hint">Loading…</p>
        </div>
      </div>
    );
  }
  if (cache.error) {
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <h3 className="section-label">{activeSection.label}</h3>
          <p className="empty-hint">{cache.error}</p>
        </div>
      </div>
    );
  }
  if (cache.items.length === 0) {
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <h3 className="section-label">{activeSection.label}</h3>
          <p className="empty-hint">No data found.</p>
        </div>
      </div>
    );
  }

  const displayItems = activeSection.id === "bundle-ids"
    ? [...cache.items].sort((a, b) => compareBundleIDPlatforms(a.platform, b.platform, bundleIDsPlatformSort))
    : cache.items;
  const showSectionSearch = !sectionRequiresApp(activeSection.id) && displayItems.length > 1;
  const filteredItems = displayItems.filter((item) => itemMatchesSearch(item, activeSectionSearch));

  // Build column list from all items' keys
  const allKeys = new Set<string>();
  for (const item of filteredItems.length > 0 ? filteredItems : displayItems) {
    for (const [k, v] of Object.entries(item)) {
      if (k !== "id" && k !== "type" && v !== null && v !== undefined && v !== "" && typeof v !== "object") {
        allKeys.add(k);
      }
    }
  }
  const columns = [...allKeys];

  const headerActions = (
    <div className="section-header-actions">
      {showSectionSearch && (
        <input
          type="search"
          className="section-search-input"
          aria-label={`${activeSection.label} search`}
          placeholder={`Search ${activeSection.label.toLowerCase()}…`}
          value={activeSectionSearch}
          onChange={(event) => onSetSectionSearch(activeSection.id, event.target.value)}
        />
      )}
      {activeSection.id === "bundle-ids" && (
        <button
          type="button"
          className="toolbar-btn section-create-btn"
          onClick={onOpenBundleIDSheet}
        >
          <span aria-hidden="true">+</span>
          <span>New Bundle ID</span>
        </button>
      )}
      {activeSection.id === "devices" && (
        <button
          type="button"
          className="toolbar-btn section-create-btn"
          onClick={onOpenDeviceSheet}
        >
          <span aria-hidden="true">+</span>
          <span>New Device</span>
        </button>
      )}
    </div>
  );

  if (filteredItems.length === 0) {
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <div className="section-header-row">
            <div className="section-header-meta">
              <h3 className="section-label">{activeSection.label}</h3>
              {showSectionSearch && <span className="section-count">{displayItems.length} items</span>}
            </div>
            {headerActions}
          </div>
          <p className="empty-hint">No matching results.</p>
        </div>
      </div>
    );
  }

  // Single-item views (like age-rating) render as key-value pairs
  if (filteredItems.length === 1) {
    const item = filteredItems[0];
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <div className="section-header-row">
            <div className="section-header-meta">
              <h3 className="section-label">{activeSection.label}</h3>
            </div>
            {headerActions}
          </div>
          <table className="data-table">
            <thead>
              <tr>
                <th>Setting</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              {columns.map((key) => (
                <tr key={key}>
                  <td>{fieldLabels[key] ?? key}</td>
                  <td>{fmt(String(item[key] ?? ""))}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  // Wide tables (>5 columns): render each item as a vertical card
  if (columns.length > 5) {
    return (
      <div className="app-detail-view">
        <div className="app-detail-section">
          <div className="section-header-row">
            <div className="section-header-meta">
              <h3 className="section-label">{activeSection.label}</h3>
              <span className="section-count">{filteredItems.length} of {displayItems.length}</span>
            </div>
            {headerActions}
          </div>
          {filteredItems.map((item, idx) => (
            <div key={item.id as string ?? idx} className="vertical-card">
              <table className="data-table">
                <tbody>
                  {columns.map((key) => {
                    const raw = item[key] != null ? String(item[key]) : "";
                    const display = fmt(raw);
                    const isState = key === "state" || key === "appVersionState" || key === "appStoreState";
                    return (
                      <tr key={key}>
                        <td className="vcard-label">{fieldLabels[key] ?? key}</td>
                        <td>{isState ? <span className={`status-pill status-${raw.toLowerCase().replace(/_/g, "-")}`}>{display}</span> : display}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      </div>
    );
  }

  // Standard table
  return (
    <div className="app-detail-view">
      <div className="app-detail-section">
        <div className="section-header-row">
          <div className="section-header-meta">
            <h3 className="section-label">{activeSection.label}</h3>
            <span className="section-count">{filteredItems.length} of {displayItems.length}</span>
          </div>
          {headerActions}
        </div>
        <table className="data-table">
          <thead>
            <tr>
              {columns.map((col) => (
                <th key={col}>
                  {activeSection.id === "bundle-ids" && col === "platform" ? (
                    <button
                      type="button"
                      className="table-sort-button"
                      aria-label={`Sort by platform, currently ${bundleIDsPlatformSort === "asc" ? "ascending" : "descending"}`}
                      onClick={onToggleBundleIDSort}
                    >
                      <span>{fieldLabels[col] ?? col}</span>
                      <span aria-hidden="true" className="table-sort-arrow">
                        {bundleIDsPlatformSort === "asc" ? "\u2191" : "\u2193"}
                      </span>
                    </button>
                  ) : (
                    fieldLabels[col] ?? col
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filteredItems.map((item, idx) => (
              <tr key={item.id as string ?? idx}>
                {columns.map((col) => {
                  const val = item[col];
                  const isState = col === "state" || col === "appVersionState" || col === "appStoreState" || col === "processingState";
                  const raw = val != null ? String(val) : "";
                  const display = fmt(raw);
                  return (
                    <td key={col}>
                      {isState ? (
                        <span className={`status-pill status-${raw.toLowerCase().replace(/_/g, "-")}`}>
                          {display}
                        </span>
                      ) : display}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
