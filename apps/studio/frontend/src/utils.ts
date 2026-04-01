import { displayValue, screenshotDisplayLabels, sectionCommands, bundleIDPlatformOrder } from "./constants";
import { AuthState, EnvSnapshot, StudioSettings } from "./types";

export function fmt(val: string): string {
  if (displayValue[val]) return displayValue[val];
  // Format ISO dates like "2026-03-28T08:32:01-07:00" -> "2026-03-28"
  if (/^\d{4}-\d{2}-\d{2}T/.test(val)) return val.split("T")[0];
  return val;
}

export function screenshotLabel(displayType: string): string {
  return screenshotDisplayLabels[displayType] ?? displayType.replace(/^APP_/, "").replace(/_/g, " ");
}

export function insightsWeekStart(today: Date): string {
  const monday = new Date(today);
  const day = today.getDay();
  const daysSinceMonday = day === 0 ? 6 : day - 1;
  monday.setDate(today.getDate() - daysSinceMonday);
  // Use local date to avoid UTC timezone shift
  const y = monday.getFullYear();
  const m = String(monday.getMonth() + 1).padStart(2, "0");
  const d = String(monday.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export function normalizeEnvSnapshot(snapshot?: Partial<EnvSnapshot>): EnvSnapshot {
  return {
    configPath: snapshot?.configPath || "",
    configPresent: snapshot?.configPresent || false,
    defaultAppId: snapshot?.defaultAppId || "",
    keychainAvailable: snapshot?.keychainAvailable || false,
    keychainBypassed: snapshot?.keychainBypassed || false,
    keychainWarning: snapshot?.keychainWarning || "",
    workflowPath: snapshot?.workflowPath || "",
  };
}

export function normalizeStudioSettings(input?: Partial<StudioSettings>): StudioSettings {
  return {
    preferredPreset: input?.preferredPreset || "codex",
    agentCommand: input?.agentCommand || "",
    agentArgs: input?.agentArgs || [],
    agentEnv: input?.agentEnv || {},
    preferBundledASC: input?.preferBundledASC ?? true,
    systemASCPath: input?.systemASCPath || "",
    workspaceRoot: input?.workspaceRoot || "",
    showCommandPreviews: input?.showCommandPreviews ?? true,
    theme: input?.theme || "system",
    windowMaterial: input?.windowMaterial || "translucent",
  };
}

export function normalizeAuthStatus(input?: Partial<AuthState>): AuthState {
  return {
    authenticated: input?.authenticated || false,
    storage: input?.storage || "",
    profile: input?.profile || "",
    rawOutput: input?.rawOutput || "",
  };
}

export function mapAppList(apps?: { id: string; name: string; subtitle: string }[]) {
  return (apps ?? []).map((app) => ({
    id: app.id,
    name: app.name,
    subtitle: app.subtitle,
  }));
}

export function itemMatchesSearch(item: Record<string, unknown>, query: string): boolean {
  const trimmed = query.trim().toLowerCase();
  if (!trimmed) return true;
  return Object.values(item).some((value) => {
    if (value == null || typeof value === "object") return false;
    return String(value).toLowerCase().includes(trimmed);
  });
}

export function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

export function commandForApp(cmdTemplate: string, appId: string): string {
  return cmdTemplate.replace(/APP_ID/g, shellQuote(appId));
}

export function sectionRequiresApp(sectionId: string): boolean {
  return sectionCommands[sectionId]?.includes("APP_ID") ?? false;
}

export function compareBundleIDPlatforms(a: unknown, b: unknown, direction: "asc" | "desc"): number {
  const aPlatform = String(a ?? "");
  const bPlatform = String(b ?? "");
  const aRank = bundleIDPlatformOrder[aPlatform] ?? Number.MAX_SAFE_INTEGER;
  const bRank = bundleIDPlatformOrder[bPlatform] ?? Number.MAX_SAFE_INTEGER;
  const rankDiff = aRank - bRank;
  if (rankDiff !== 0) return direction === "asc" ? rankDiff : -rankDiff;
  return direction === "asc"
    ? aPlatform.localeCompare(bPlatform)
    : bPlatform.localeCompare(aPlatform);
}

export function getSystemTheme(): "light" | "dark" {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function resolveTheme(theme: string | undefined, systemTheme: "light" | "dark"): "light" | "dark" {
  switch (theme) {
    case "dark":
    case "glass-dark":
      return "dark";
    case "light":
    case "glass-light":
      return "light";
    default:
      return systemTheme;
  }
}
