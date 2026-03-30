import { FormEvent, useEffect, useState } from "react";

import "./styles.css";
import { ChatMessage, NavSection } from "./types";
import { Bootstrap, CheckAuthStatus, GetSettings, SaveSettings } from "../wailsjs/go/main/App";
import { environment, settings as settingsNS } from "../wailsjs/go/models";

const sections: NavSection[] = [
  { id: "overview", label: "Overview", description: "Release cockpit" },
  { id: "builds", label: "Builds", description: "TestFlight and processing" },
  { id: "submission", label: "Submission", description: "Validation and publish" },
  { id: "settings", label: "Settings", description: "Studio preferences" },
];

const sectionIcons: Record<string, string> = {
  overview: "◎",
  builds: "⏣",
  submission: "↗",
  settings: "⚙",
};

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
  const [activeSection, setActiveSection] = useState<NavSection>(sections[0]);
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
          <button className="sidebar-action" type="button" aria-label="New thread">+</button>
        </div>

        <div className="sidebar-section">
          <p className="sidebar-section-label">Workspace</p>
          {sections.map((section) => (
            <button
              key={section.id}
              type="button"
              className={`sidebar-row ${section.id === activeSection.id ? "is-active" : ""}`}
              onClick={() => setActiveSection(section)}
            >
              <span className="sidebar-row-icon">{sectionIcons[section.id]}</span>
              <span>{section.label}</span>
            </button>
          ))}
        </div>

        <div className="sidebar-spacer" />

        <div className="thread-section">
          <p className="sidebar-section-label">Threads</p>
          {messages.length > 0 ? (
            <div className="thread-row is-selected">
              <strong>Current session</strong>
              <small>now</small>
            </div>
          ) : (
            <div className="thread-row">
              <span className="empty-hint">No threads yet</span>
            </div>
          )}
        </div>
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
                onClick={() => setActiveSection(sections.find((s) => s.id === "settings")!)}
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
              {authStatus.rawOutput && (
                <pre className="command-preview">{authStatus.rawOutput}</pre>
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
              onClick={() => setActiveSection(sections.find((s) => s.id === "settings")!)}
            >
              Open Settings
            </button>
          </div>
        ) : (
          <div className="empty-state">
            <p className="empty-title">
              {activeSection.label}
            </p>
            <p className="empty-hint">
              This workspace section is not wired to live data yet. Use the ACP chat below to run commands.
            </p>
          </div>
        )}

        {/* Chat dock */}
        <section className={`dock ${dockExpanded ? "dock-expanded" : ""}`}>
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
        </section>
      </div>
    </div>
  );
}
