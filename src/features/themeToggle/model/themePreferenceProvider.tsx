"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { loadThemePreference, saveThemePreference, type ThemePreference } from "../api/themePreferences";

type ThemePreferenceContextValue = {
  isPending: boolean;
  setTheme: (theme: ThemePreference) => Promise<void>;
  theme: ThemePreference;
};

const ThemePreferenceContext = createContext<ThemePreferenceContextValue | null>(null);

export function ThemePreferenceProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [theme, setThemeState] = useState<ThemePreference>("light");
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isPending, setIsPending] = useState(false);

  const applyTheme = useCallback((nextTheme: ThemePreference) => {
    setThemeState(nextTheme);
    document.documentElement.dataset.theme = nextTheme;
  }, []);

  const reload = useCallback(async (signal?: AbortSignal) => {
    try {
      const savedTheme = await loadThemePreference(signal);
      if (signal?.aborted) return;
      setIsAuthenticated(savedTheme !== null);
      if (savedTheme) applyTheme(savedTheme);
    } catch (reason) {
      if (!(reason instanceof DOMException && reason.name === "AbortError")) setIsAuthenticated(false);
    }
  }, [applyTheme]);

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => { if (!controller.signal.aborted) void reload(controller.signal); });
    const sessionChanged = (event: Event) => {
      const authenticated = event instanceof CustomEvent ? event.detail?.authenticated : undefined;
      if (authenticated === false) {
        setIsAuthenticated(false);
        applyTheme("light");
        return;
      }
      void reload();
    };
    window.addEventListener("arhdesign:session-changed", sessionChanged);
    return () => {
      controller.abort();
      window.removeEventListener("arhdesign:session-changed", sessionChanged);
    };
  }, [applyTheme, reload]);

  const setTheme = useCallback(async (nextTheme: ThemePreference) => {
    const previousTheme = theme;
    applyTheme(nextTheme);
    if (!isAuthenticated) return;
    setIsPending(true);
    try {
      applyTheme(await saveThemePreference(nextTheme));
    } catch (reason) {
      applyTheme(previousTheme);
      throw reason;
    } finally {
      setIsPending(false);
    }
  }, [applyTheme, isAuthenticated, theme]);

  const value = useMemo(() => ({ isPending, setTheme, theme }), [isPending, setTheme, theme]);
  return <ThemePreferenceContext.Provider value={value}>{children}</ThemePreferenceContext.Provider>;
}

export function useThemePreference() {
  const context = useContext(ThemePreferenceContext);
  if (!context) throw new Error("useThemePreference must be used inside ThemePreferenceProvider");
  return context;
}
