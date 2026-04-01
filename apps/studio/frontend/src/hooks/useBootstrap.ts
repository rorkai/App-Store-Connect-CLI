import { startTransition, useEffect, useEffectEvent, useRef, useState } from "react";
import { AuthState, AppListItem, EnvSnapshot, StudioSettings } from "../types";
import { emptyEnv, defaultSettings, emptyAuthStatus } from "../constants";
import { normalizeEnvSnapshot, normalizeStudioSettings, normalizeAuthStatus, mapAppList } from "../utils";
import { Bootstrap, CheckAuthStatus, ListApps, SaveSettings } from "../../wailsjs/go/main/App";
import { settings as settingsNS } from "../../wailsjs/go/models";

export function useBootstrap() {
  const [env, setEnv] = useState<EnvSnapshot>(emptyEnv as EnvSnapshot);
  const [studioSettings, setStudioSettings] = useState<StudioSettings>(defaultSettings as StudioSettings);
  const [settingsSaved, setSettingsSaved] = useState(false);
  const [bootstrapError, setBootstrapError] = useState("");
  const [loading, setLoading] = useState(true);
  const [authStatus, setAuthStatus] = useState<AuthState>(emptyAuthStatus as AuthState);
  const [appList, setAppList] = useState<AppListItem[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [appsError, setAppsError] = useState("");
  const requestRef = useRef(0);

  const loadStudioShell = useEffectEvent(async (options?: {
    clearApps?: boolean;
    requestId?: number;
  }) => {
    const requestId = options?.requestId ?? ++requestRef.current;
    const isStale = () => requestId !== requestRef.current;

    try {
      const [data, auth] = await Promise.all([Bootstrap(), CheckAuthStatus()]);
      if (isStale()) return;

      startTransition(() => {
        setEnv(normalizeEnvSnapshot(data.environment));
        setStudioSettings(normalizeStudioSettings(data.settings));
        setAuthStatus(normalizeAuthStatus(auth));
        setBootstrapError("");
        setAppsError("");
        if (options?.clearApps) setAppList([]);
      });

      if (!auth?.authenticated) {
        if (isStale()) return;
        startTransition(() => {
          setAppList([]);
          setAppsError("");
          setAppsLoading(false);
        });
        return;
      }

      setAppsLoading(true);
      try {
        const res = await ListApps();
        if (isStale()) return;
        startTransition(() => {
          if (res.error) {
            setAppList([]);
            setAppsError(res.error);
            return;
          }
          setAppList(mapAppList(res.apps));
          setAppsError("");
        });
      } catch (err) {
        if (isStale()) return;
        startTransition(() => {
          setAppList([]);
          setAppsError(String(err));
        });
      } finally {
        if (!isStale()) setAppsLoading(false);
      }
    } catch (err) {
      if (isStale()) return;
      setBootstrapError(String(err));
    } finally {
      if (!isStale()) setLoading(false);
    }
  });

  // Sync with external system: Wails backend on app startup.
  // No user action triggers this — the app just opened.
  useEffect(() => {
    const requestId = ++requestRef.current;
    void loadStudioShell({ requestId });
    return () => { if (requestRef.current === requestId) requestRef.current += 1; };
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
      agentEnv: studioSettings.agentEnv,
      preferBundledASC: studioSettings.preferBundledASC,
      systemASCPath: studioSettings.systemASCPath,
      workspaceRoot: studioSettings.workspaceRoot,
      theme: studioSettings.theme,
      windowMaterial: studioSettings.windowMaterial,
      showCommandPreviews: studioSettings.showCommandPreviews,
    });
    SaveSettings(payload)
      .then(() => setSettingsSaved(true))
      .catch((err) => console.error("save settings:", err));
  }

  function handleRefresh(selectedAppId: string | null, reselectApp: (id: string) => void) {
    if (selectedAppId) {
      reselectApp(selectedAppId);
    } else {
      const requestId = ++requestRef.current;
      setLoading(true);
      setBootstrapError("");
      void loadStudioShell({ clearApps: true, requestId });
    }
  }

  return {
    env, authStatus, studioSettings, settingsSaved, bootstrapError, loading,
    appList, appsLoading, appsError,
    updateSetting, handleSaveSettings, handleRefresh,
  };
}
