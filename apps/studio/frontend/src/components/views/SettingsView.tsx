import { AuthState, EnvSnapshot, StudioSettings } from "../../types";

type SettingsViewProps = {
  authStatus: AuthState;
  env: EnvSnapshot;
  studioSettings: StudioSettings;
  settingsSaved: boolean;
  updateSetting: <K extends keyof StudioSettings>(key: K, value: StudioSettings[K]) => void;
  handleSaveSettings: () => void;
};

export function SettingsView({
  authStatus,
  env,
  studioSettings,
  settingsSaved,
  updateSetting,
  handleSaveSettings,
}: SettingsViewProps) {
  const authConfigured = authStatus.authenticated;

  return (
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
        {env.keychainWarning && (
          <p className="settings-hint">
            Keychain access warning: {env.keychainWarning}
          </p>
        )}
      </div>

      {/* ACP Provider */}
      <div className="workspace-section">
        <h3 className="section-label">ACP Provider</h3>
        <div className="settings-field">
          <label className="settings-label">Preferred preset</label>
          <div className="segmented" role="radiogroup" aria-label="Preferred preset">
            {["codex", "claude", "custom"].map((preset) => (
              <button
                key={preset}
                type="button"
                role="radio"
                aria-checked={studioSettings.preferredPreset === preset}
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
  );
}
