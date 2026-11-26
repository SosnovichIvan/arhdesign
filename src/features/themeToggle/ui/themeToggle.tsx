"use client";
import { IconButton } from "@/shared/ui";
import { useThemePreference } from "../model/themePreferenceProvider";

export function ThemeToggle({ showTooltip = false }: Readonly<{ showTooltip?: boolean }>) {
  const { isPending, setTheme, theme } = useThemePreference();
  const dark = theme === "dark";
  const label = dark ? "Включить светлую тему" : "Включить тёмную тему";
  return <IconButton aria-label={label} disabled={isPending} onClick={() => void setTheme(dark ? "light" : "dark").catch(() => undefined)} tooltip={showTooltip ? label : undefined} tooltipAlign="end" tooltipPlacement="bottom">{dark ? "☀" : "◐"}</IconButton>;
}
