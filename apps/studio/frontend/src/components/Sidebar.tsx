import { allSections, scopes } from "../constants";
import { AppDetail, AppListItem, NavSection } from "../types";

type SidebarProps = {
  activeScope: string;
  selectedAppId: string | null;
  appDetail: AppDetail | null;
  appList: AppListItem[];
  appSearchTerm: string;
  activeSection: NavSection;
  appsLoading: boolean;
  appsError: string;
  authAuthenticated: boolean;
  filteredApps: AppListItem[];
  onAppSearchChange: (term: string) => void;
  onSelectApp: (id: string) => void;
  onSetActiveSection: (section: NavSection) => void;
};

export function Sidebar({
  activeScope,
  selectedAppId,
  appDetail,
  appList,
  appSearchTerm,
  activeSection,
  appsLoading,
  appsError,
  authAuthenticated,
  filteredApps,
  onAppSearchChange,
  onSelectApp,
  onSetActiveSection,
}: SidebarProps) {
  return (
    <aside className="sidebar" aria-label="App navigation">
      <div className="sidebar-header" />

      {/* App picker dropdown — only in App scope */}
      {activeScope === "app" && <div className="sidebar-app-picker">
        {appsLoading ? (
          <div className="app-picker-placeholder" role="status">Loading apps…</div>
        ) : appList.length > 0 ? (
          <>
            <input
              className="app-picker-search"
              type="search"
              aria-label="Search apps"
              placeholder="Search apps…"
              value={appSearchTerm}
              onChange={(e) => onAppSearchChange(e.target.value)}
            />
            <div className="app-picker-list" role="listbox" aria-label="Apps">
              {filteredApps.length > 0 ? filteredApps.map((app) => (
                <button
                  key={app.id}
                  type="button"
                  role="option"
                  aria-selected={selectedAppId === app.id}
                  className={`app-picker-item ${selectedAppId === app.id ? "is-active" : ""}`}
                  onClick={() => {
                    onSelectApp(app.id);
                    onSetActiveSection(allSections[0]);
                  }}
                >
                  <span className="app-picker-item-name">{app.name}</span>
                  {app.subtitle && <span className="app-picker-item-subtitle">{app.subtitle}</span>}
                </button>
              )) : (
                <div className="app-picker-placeholder" role="status">No matching apps</div>
              )}
            </div>
          </>
        ) : (
          <div className="app-picker-placeholder" role="status">
            {authAuthenticated ? (appsError || "No apps found") : "Not authenticated"}
          </div>
        )}
      </div>}

      <nav className="sidebar-scroll" aria-label="Sections">
      {/* Version badges when an app is selected (app scope only) */}
      {activeScope === "app" && appDetail && appDetail.versions.length > 0 && (
        <div className="sidebar-section">
          {(["IOS", "MAC_OS", "VISION_OS"] as const).map((platform) => {
            const v = appDetail.versions.find((ver) => ver.platform === platform);
            if (!v) return null;
            const label = platform === "IOS" ? "iOS App" : platform === "MAC_OS" ? "macOS App" : "visionOS App";
            const stateLabel = v.state.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
            return (
              <div key={platform} className="sidebar-version-group">
                <p className="sidebar-version-platform">{label}</p>
                <div className="sidebar-version-row">
                  <span
                    className={`sidebar-version-dot state-${v.state.toLowerCase().replace(/_/g, "-")}`}
                    role="img"
                    aria-label={stateLabel}
                  />
                  <span className="sidebar-version-text">
                    {v.version} {stateLabel}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {(scopes.find((s) => s.id === activeScope)?.groups ?? []).map((group) => {
        // App scope needs an app selected; other scopes don't
        if (activeScope === "app" && !selectedAppId) return null;
        return (
          <div key={group.label} className="sidebar-section" role="group" aria-label={group.label}>
            <p className="sidebar-section-label" aria-hidden="true">{group.label}</p>
            {group.items.map((section) => (
              <button
                key={section.id}
                type="button"
                className={`sidebar-row ${section.id === activeSection.id ? "is-active" : ""}`}
                aria-current={section.id === activeSection.id ? "page" : undefined}
                onClick={() => onSetActiveSection(section)}
              >
                <span>{section.label}</span>
              </button>
            ))}
          </div>
        );
      })}

      <div className="sidebar-section">
        <button
          type="button"
          className={`sidebar-row ${activeSection.id === "settings" ? "is-active" : ""}`}
          aria-current={activeSection.id === "settings" ? "page" : undefined}
          onClick={() => onSetActiveSection(allSections.find((s) => s.id === "settings")!)}
        >
          <span className="sidebar-row-icon sidebar-row-icon-settings" aria-hidden="true">&#x2699;</span>
          <span>Settings</span>
        </button>
      </div>

      <div className="sidebar-spacer" />
      </nav>
    </aside>
  );
}
