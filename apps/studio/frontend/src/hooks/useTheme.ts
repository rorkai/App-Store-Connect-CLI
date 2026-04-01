import { useSyncExternalStore } from "react";
import { getSystemTheme, resolveTheme } from "../utils";

const SYSTEM_THEME_MEDIA_QUERY = "(prefers-color-scheme: dark)";

function getServerTheme(): "light" | "dark" {
  return "light";
}

function subscribeToSystemTheme(onStoreChange: () => void) {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => {};
  }

  const mediaQuery = window.matchMedia(SYSTEM_THEME_MEDIA_QUERY);
  const handleChange = () => onStoreChange();

  if (typeof mediaQuery.addEventListener === "function") {
    mediaQuery.addEventListener("change", handleChange);
    return () => mediaQuery.removeEventListener("change", handleChange);
  }

  mediaQuery.addListener(handleChange);
  return () => mediaQuery.removeListener(handleChange);
}

export function useTheme(themePreference: string) {
  const systemTheme = useSyncExternalStore(
    subscribeToSystemTheme,
    getSystemTheme,
    getServerTheme,
  );

  const resolvedTheme = resolveTheme(themePreference, systemTheme);

  return { resolvedTheme, systemTheme };
}
