"use client";
import { IconButton } from "@/shared/ui";
import { useThemePreference } from "../model/themePreferenceProvider";

export function ThemeToggle() {
  const { isPending, setTheme, theme } = useThemePreference();
  const dark = theme === "dark";
  return <IconButton aria-label={dark ? "Включить светлую тему" : "Включить тёмную тему"} disabled={isPending} onClick={() => void setTheme(dark ? "light" : "dark").catch(() => undefined)}>{dark ? "☀" : "◐"}</IconButton>;
}
