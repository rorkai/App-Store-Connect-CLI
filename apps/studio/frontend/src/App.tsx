import { FormEvent, useEffect, useState } from "react";

import "./styles.css";
import { ChatMessage, NavSection } from "./types";
import { Bootstrap, CheckAuthStatus, GetAppDetail, GetScreenshots, GetSettings, GetVersionMetadata, ListApps, SaveSettings } from "../wailsjs/go/main/App";
import { environment, settings as settingsNS } from "../wailsjs/go/models";

type SidebarGroup = { label: string; items: NavSection[] };

const sidebarGroups: SidebarGroup[] = [
  {
    label: "General",
    items: [
      { id: "overview", label: "App Information", description: "App details and metadata" },
      { id: "app-review", label: "App Review", description: "Review details and attachments" },
      { id: "history", label: "History", description: "Version history" },
    ],
  },
  {
    label: "Trust & Safety",
    items: [
      { id: "app-privacy", label: "App Privacy", description: "Privacy declarations" },
      { id: "app-accessibility", label: "App Accessibility", description: "Accessibility declarations" },
      { id: "ratings-reviews", label: "Ratings and Reviews", description: "Customer reviews" },
    ],
  },
  {
    label: "Growth & Marketing",
    items: [
      { id: "in-app-events", label: "In-App Events", description: "App events" },
      { id: "custom-product-pages", label: "Custom Product Pages", description: "Product pages" },
      { id: "ppo", label: "Product Page Optimization", description: "A/B tests" },
      { id: "promo-codes", label: "Promo Codes", description: "Promotional codes" },
      { id: "game-center", label: "Game Center", description: "Game Center resources" },
    ],
  },
  {
    label: "Monetization",
    items: [
      { id: "pricing", label: "Pricing and Availability", description: "Pricing" },
      { id: "iap", label: "In-App Purchases", description: "In-app purchases" },
      { id: "subscriptions", label: "Subscriptions", description: "Subscription groups" },
    ],
  },
  {
    label: "Featuring",
    items: [
      { id: "nominations", label: "Nominations", description: "Featuring nominations" },
    ],
  },
];

// Flatten for lookup
const allSections: NavSection[] = sidebarGroups.flatMap((g) => g.items);
allSections.push({ id: "settings", label: "Settings", description: "Studio preferences" });

type EnvSnapshot = {
  configPath: string;
  configPresent: boolean;
  defaultAppId: string;
  keychainAvailable: boolean;
  keychainBypassed: boolean;
  workflowPath: string;
};

type StudioSettings = {
  preferredPreset: string;
  agentCommand: string;
  agentArgs: string[];
  preferBundledASC: boolean;
  systemASCPath: string;
  workspaceRoot: string;
  showCommandPreviews: boolean;
};

const emptyEnv: EnvSnapshot = {
  configPath: "",
  configPresent: false,
  defaultAppId: "",
  keychainAvailable: false,
  keychainBypassed: false,
  workflowPath: "",
};

const defaultSettings: StudioSettings = {
  preferredPreset: "codex",
  agentCommand: "",
  agentArgs: [],
  preferBundledASC: true,
  systemASCPath: "",
  workspaceRoot: "",
  showCommandPreviews: true,
};

export default function App() {
  const [activeSection, setActiveSection] = useState<NavSection>(allSections[0]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [dockExpanded, setDockExpanded] = useState(false);

  const [env, setEnv] = useState<EnvSnapshot>(emptyEnv);
  const [studioSettings, setStudioSettings] = useState<StudioSettings>(defaultSettings);
  const [settingsSaved, setSettingsSaved] = useState(false);
  const [bootstrapError, setBootstrapError] = useState("");
  const [loading, setLoading] = useState(true);
  const [authStatus, setAuthStatus] = useState<{ authenticated: boolean; storage: string; profile: string; rawOutput: string }>({
    authenticated: false, storage: "", profile: "", rawOutput: "",
  });
  const [appList, setAppList] = useState<{ id: string; name: string; subtitle: string }[]>([]);
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [appDetail, setAppDetail] = useState<{
    id: string; name: string; subtitle: string; bundleId: string; sku: string; primaryLocale: string;
    versions: { id: string; platform: string; version: string; state: string }[];
    error?: string;
  } | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [allLocalizations, setAllLocalizations] = useState<{
    localizationId: string; locale: string; description: string; keywords: string;
    whatsNew: string; promotionalText: string; supportUrl: string; marketingUrl: string;
  }[]>([]);
  const [selectedLocale, setSelectedLocale] = useState<string>("");
  const [metadataLoading, setMetadataLoading] = useState(false);
  const [screenshotSets, setScreenshotSets] = useState<{
    displayType: string;
    screenshots: { thumbnailUrl: string; width: number; height: number }[];
  }[]>([]);
  const [screenshotsLoading, setScreenshotsLoading] = useState(false);
  const [appsLoading, setAppsLoading] = useState(false);

  useEffect(() => {
    Promise.all([Bootstrap(), CheckAuthStatus()])
      .then(([data, auth]) => {
        if (data.environment) {
          setEnv({
            configPath: data.environment.configPath || "",
            configPresent: data.environment.configPresent || false,
            defaultAppId: data.environment.defaultAppId || "",
            keychainAvailable: data.environment.keychainAvailable || false,
            keychainBypassed: data.environment.keychainBypassed || false,
            workflowPath: data.environment.workflowPath || "",
          });
        }
        if (data.settings) {
          setStudioSettings({
            preferredPreset: data.settings.preferredPreset || "codex",
            agentCommand: data.settings.agentCommand || "",
            agentArgs: data.settings.agentArgs || [],
            preferBundledASC: data.settings.preferBundledASC ?? true,
            systemASCPath: data.settings.systemASCPath || "",
            workspaceRoot: data.settings.workspaceRoot || "",
            showCommandPreviews: data.settings.showCommandPreviews ?? true,
          });
        }
        if (auth) {
          setAuthStatus({
            authenticated: auth.authenticated || false,
            storage: auth.storage || "",
            profile: auth.profile || "",
            rawOutput: auth.rawOutput || "",
          });
          if (auth.authenticated) {
            setAppsLoading(true);
            ListApps()
              .then((res) => {
                if (res.apps) setAppList(res.apps.map((a: { id: string; name: string; subtitle: string }) => ({
                  id: a.id, name: a.name, subtitle: a.subtitle,
                })));
              })
              .catch(() => {})
              .finally(() => setAppsLoading(false));
          }
        }
        setLoading(false);
      })
      .catch((err) => {
        setBootstrapError(String(err));
        setLoading(false);
      });
  }, []);

  function updateSetting<K extends keyof StudioSettings>(key: K, value: StudioSettings[K]) {
    setStudioSettings((prev) => ({ ...prev, [key]: value }));
    setSettingsSaved(false);
  }

  function handleSaveSettings() {
    const payload = new settingsNS.StudioSettings({
      preferredPreset: studioSettings.preferredPreset,
      agentCommand: studioSettings.agentCommand,
      agentArgs: studioSettings.agentArgs,
      agentEnv: {},
      preferBundledASC: studioSettings.preferBundledASC,
      systemASCPath: studioSettings.systemASCPath,
      workspaceRoot: studioSettings.workspaceRoot,
      theme: "glass-light",
      windowMaterial: "translucent",
      showCommandPreviews: studioSettings.showCommandPreviews,
    });
    SaveSettings(payload)
      .then(() => setSettingsSaved(true))
      .catch((err) => console.error("save settings:", err));
  }

  function handleSelectApp(id: string) {
    setSelectedAppId(id);
    setAppDetail(null);
    setAllLocalizations([]);
    setSelectedLocale("");
    setScreenshotSets([]);
    setDetailLoading(true);
    GetAppDetail(id)
      .then((d) => {
        const detail = {
          id: d.id, name: d.name, subtitle: d.subtitle, bundleId: d.bundleId,
          sku: d.sku, primaryLocale: d.primaryLocale, versions: d.versions ?? [], error: d.error,
        };
        setAppDetail(detail);
        // Fetch metadata for the primary iOS version (fallback to first version)
        const primaryVersion = (d.versions ?? []).find((v: { platform: string }) => v.platform === "IOS")
          ?? (d.versions ?? [])[0];
        if (primaryVersion?.id) {
          setMetadataLoading(true);
          GetVersionMetadata(primaryVersion.id)
            .then((meta) => {
              if (meta.localizations?.length) {
                setAllLocalizations(meta.localizations);
                const defaultLoc = meta.localizations.find(
                  (l: { locale: string }) => l.locale === d.primaryLocale
                ) ?? meta.localizations[0];
                setSelectedLocale(defaultLoc.locale);
                // Fetch screenshots for the default locale in parallel
                if (defaultLoc.localizationId) {
                  setScreenshotsLoading(true);
                  GetScreenshots(defaultLoc.localizationId)
                    .then((res) => setScreenshotSets(res.sets ?? []))
                    .catch(() => {})
                    .finally(() => setScreenshotsLoading(false));
                }
              }
            })
            .catch(() => {})
            .finally(() => setMetadataLoading(false));
        }
      })
      .catch((e) => setAppDetail({ id, name: "", subtitle: "", bundleId: "", sku: "", primaryLocale: "", versions: [], error: String(e) }))
      .finally(() => setDetailLoading(false));
  }

  function handleLocaleChange(locale: string) {
    setSelectedLocale(locale);
    const loc = allLocalizations.find((l) => l.locale === locale);
    if (loc?.localizationId) {
      setScreenshotsLoading(true);
      setScreenshotSets([]);
      GetScreenshots(loc.localizationId)
        .then((res) => setScreenshotSets(res.sets ?? []))
        .catch(() => {})
        .finally(() => setScreenshotsLoading(false));
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = draft.trim();
    if (!trimmed) return;

    setMessages((current) => [
      ...current,
      { id: `user-${current.length}`, role: "user", content: trimmed, timestamp: "Now" },
      {
        id: `assistant-${current.length}`,
        role: "assistant",
        content: "Bootstrap mode recorded the prompt. Live ACP transport is not wired yet.",
        timestamp: "Now",
      },
    ]);
    setDraft("");
    setDockExpanded(true);
  }

  const authConfigured = authStatus.authenticated;

  return (
    <div className="studio-shell">
      {/* Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <span className="sidebar-title">ASC Studio</span>
        </div>

        {/* App picker dropdown */}
        <div className="sidebar-app-picker">
          {appsLoading ? (
            <div className="app-picker-placeholder">Loading apps…</div>
          ) : appList.length > 0 ? (
            <select
              className="app-picker-select"
              value={selectedAppId ?? ""}
              onChange={(e) => {
                const id = e.target.value;
                if (id) {
                  handleSelectApp(id);
                  setActiveSection(allSections[0]);
                }
              }}
            >
              <option value="" disabled>Select an app…</option>
              {appList.map((app) => (
                <option key={app.id} value={app.id}>
                  {app.name}{app.subtitle ? ` — ${app.subtitle}` : ""}
                </option>
              ))}
            </select>
          ) : (
            <div className="app-picker-placeholder">
              {authStatus.authenticated ? "No apps found" : "Not authenticated"}
            </div>
          )}
        </div>

        {/* Version badges when an app is selected */}
        {appDetail && appDetail.versions.length > 0 && (
          <div className="sidebar-section">
            {(["IOS", "MAC_OS", "VISION_OS"] as const).map((platform) => {
              const v = appDetail.versions.find((ver) => ver.platform === platform);
              if (!v) return null;
              const label = platform === "IOS" ? "iOS App" : platform === "MAC_OS" ? "macOS App" : "visionOS App";
              return (
                <div key={platform} className="sidebar-version-group">
                  <p className="sidebar-version-platform">{label}</p>
                  <div className="sidebar-version-row">
                    <span className={`sidebar-version-dot state-${v.state.toLowerCase().replace(/_/g, "-")}`} />
                    <span className="sidebar-version-text">
                      {v.version} {v.state.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {selectedAppId && sidebarGroups.map((group) => (
          <div key={group.label} className="sidebar-section">
            <p className="sidebar-section-label">{group.label}</p>
            {group.items.map((section) => (
              <button
                key={section.id}
                type="button"
                className={`sidebar-row ${section.id === activeSection.id ? "is-active" : ""}`}
                onClick={() => setActiveSection(section)}
              >
                <span>{section.label}</span>
              </button>
            ))}
          </div>
        ))}

        <div className="sidebar-section">
          <button
            type="button"
            className={`sidebar-row ${activeSection.id === "settings" ? "is-active" : ""}`}
            onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
          >
            <span className="sidebar-row-icon">⚙</span>
            <span>Settings</span>
          </button>
        </div>

        <div className="sidebar-spacer" />
      </aside>

      <div className="shell-separator" />

      {/* Main area */}
      <div className="main-area">
        {/* Context bar */}
        <header className="context-bar">
          <div className="context-app">
            <strong className="context-app-name">ASC Studio</strong>
            {authConfigured ? (
              <>
                <span className="context-badge">{authStatus.storage || "Authenticated"}</span>
                {authStatus.profile && (
                  <span className="context-version">{authStatus.profile}</span>
                )}
                <span className="context-status state-ready">Connected</span>
              </>
            ) : (
              <span className="context-status state-processing">Not authenticated</span>
            )}
          </div>
          <div className="toolbar-right">
            {!authConfigured && (
              <button
                className="toolbar-btn"
                type="button"
                onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
              >
                Configure
              </button>
            )}
          </div>
        </header>

        {loading ? (
          <div className="empty-state">
            <p className="empty-hint">Loading…</p>
          </div>
        ) : bootstrapError ? (
          <div className="empty-state">
            <p className="empty-title">Bootstrap failed</p>
            <p className="empty-hint">{bootstrapError}</p>
          </div>
        ) : activeSection.id === "settings" ? (
          <div className="settings-view">
            {/* Auth status */}
            <div className="workspace-section">
              <h3 className="section-label">Authentication</h3>
              <div className="env-grid">
                <div className="env-row">
                  <span className="env-key">Status</span>
                  <span className="env-value">
                    {authStatus.authenticated ? (
                      <span style={{ color: "var(--green)" }}>Authenticated</span>
                    ) : (
                      <span style={{ color: "var(--orange)" }}>Not authenticated</span>
                    )}
                  </span>
                </div>
                {authStatus.storage && (
                  <div className="env-row">
                    <span className="env-key">Storage</span>
                    <span className="env-value">{authStatus.storage}</span>
                  </div>
                )}
                {authStatus.profile && (
                  <div className="env-row">
                    <span className="env-key">Profile</span>
                    <span className="env-value">{authStatus.profile}</span>
                  </div>
                )}
                <div className="env-row">
                  <span className="env-key">Config file</span>
                  <span className="env-value">{env.configPresent ? env.configPath : "Not found"}</span>
                </div>
                <div className="env-row">
                  <span className="env-key">Default app ID</span>
                  <span className="env-value">{env.defaultAppId || "Not set"}</span>
                </div>
              </div>
              {!authConfigured && (
                <p className="settings-hint">
                  Run <code>asc auth login</code> in your terminal to set up credentials, then relaunch Studio.
                </p>
              )}
            </div>

            {/* ACP Provider */}
            <div className="workspace-section">
              <h3 className="section-label">ACP Provider</h3>
              <div className="settings-field">
                <label className="settings-label">Preferred preset</label>
                <div className="segmented">
                  {["codex", "claude", "custom"].map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      className={studioSettings.preferredPreset === preset ? "is-active" : ""}
                      onClick={() => updateSetting("preferredPreset", preset)}
                    >
                      {preset.charAt(0).toUpperCase() + preset.slice(1)}
                    </button>
                  ))}
                </div>
              </div>
              <div className="settings-field">
                <label className="settings-label" htmlFor="agent-command">Agent command</label>
                <input
                  id="agent-command"
                  className="settings-input"
                  type="text"
                  value={studioSettings.agentCommand}
                  onChange={(e) => updateSetting("agentCommand", e.target.value)}
                  placeholder="e.g. codex, claude-acp"
                />
              </div>
            </div>

            {/* ASC Binary */}
            <div className="workspace-section">
              <h3 className="section-label">ASC Binary</h3>
              <div className="settings-field">
                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={studioSettings.preferBundledASC}
                    onChange={(e) => updateSetting("preferBundledASC", e.target.checked)}
                  />
                  <span>Prefer bundled asc binary</span>
                </label>
              </div>
              <div className="settings-field">
                <label className="settings-label" htmlFor="asc-path">System asc path override</label>
                <input
                  id="asc-path"
                  className="settings-input"
                  type="text"
                  value={studioSettings.systemASCPath}
                  onChange={(e) => updateSetting("systemASCPath", e.target.value)}
                  placeholder="/usr/local/bin/asc"
                />
              </div>
            </div>

            {/* Workspace */}
            <div className="workspace-section">
              <h3 className="section-label">Workspace</h3>
              <div className="settings-field">
                <label className="settings-label" htmlFor="workspace-root">Workspace root</label>
                <input
                  id="workspace-root"
                  className="settings-input"
                  type="text"
                  value={studioSettings.workspaceRoot}
                  onChange={(e) => updateSetting("workspaceRoot", e.target.value)}
                  placeholder="~/Developer/my-app"
                />
              </div>
              <div className="settings-field">
                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={studioSettings.showCommandPreviews}
                    onChange={(e) => updateSetting("showCommandPreviews", e.target.checked)}
                  />
                  <span>Show command previews before execution</span>
                </label>
              </div>
            </div>

            <div className="workspace-section">
              <div className="settings-actions">
                <button className="settings-save" type="button" onClick={handleSaveSettings}>
                  Save settings
                </button>
                {settingsSaved && <span className="settings-saved-label">Saved</span>}
              </div>
            </div>
          </div>
        ) : !authConfigured ? (
          <div className="empty-state">
            <p className="empty-title">No credentials configured</p>
            <p className="empty-hint">
              Run <code>asc init</code> to create an API key profile, or go to Settings to check your configuration.
            </p>
            <button
              className="toolbar-btn"
              type="button"
              onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
            >
              Open Settings
            </button>
          </div>
        ) : activeSection.id === "overview" && appDetail ? (
          <div className="app-detail-view">
            {/* Header */}
            <div className="app-detail-header">
              <p className="app-detail-name">{appDetail.name}</p>
              {appDetail.subtitle && <p className="app-detail-subtitle">{appDetail.subtitle}</p>}
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
            ) : allLocalizations.length > 0 ? (() => {
              const loc = allLocalizations.find((l) => l.locale === selectedLocale) ?? allLocalizations[0];
              return (
                <div className="app-detail-section">
                  <div className="metadata-header">
                    <h3 className="section-label" style={{ margin: 0 }}>App Store Metadata</h3>
                    <select
                      className="locale-picker"
                      value={selectedLocale}
                      onChange={(e) => handleLocaleChange(e.target.value)}
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
        ) : (
          <div className="empty-state">
            <p className="empty-title">
              {!selectedAppId && activeSection.id !== "settings" ? "Select an App" : activeSection.label}
            </p>
            <p className="empty-hint">
              {!selectedAppId && activeSection.id !== "settings"
                ? "Use the dropdown in the sidebar to pick an app."
                : "This workspace section is not wired to live data yet."}
            </p>
          </div>
        )}

        {/* Chat dock — hidden on settings */}
        {activeSection.id !== "settings" && <section className={`dock ${dockExpanded ? "dock-expanded" : ""}`}>
          {dockExpanded && (
            <div className="dock-header">
              <span className="dock-title">ACP Chat</span>
              <button
                className="dock-collapse"
                type="button"
                onClick={() => setDockExpanded(false)}
                aria-label="Collapse chat"
              >
                ▾
              </button>
            </div>
          )}

          <div className="dock-body">
            {messages.length > 0 && (
              <div className="message-list" aria-label="Chat messages">
                {messages.map((message) => (
                  <article key={message.id} className={`message-row role-${message.role}`}>
                    <p>{message.content}</p>
                  </article>
                ))}
              </div>
            )}
          </div>

          <form className="composer" onSubmit={handleSubmit}>
            <div className="composer-card" onClick={() => !dockExpanded && setDockExpanded(true)}>
              <textarea
                aria-label="Chat prompt"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder="Ask Studio to inspect builds, explain blockers, or draft a command…"
                rows={2}
              />
              <div className="composer-bar">
                <div className="composer-meta">
                  <span>Codex</span>
                  <span>Claude</span>
                  <span>Custom ACP</span>
                </div>
                <button className="send-btn" type="submit" aria-label="Send">⬆</button>
              </div>
            </div>
          </form>
        </section>}
      </div>
    </div>
  );
}
