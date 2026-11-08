import { csrfToken, readApiError } from "@/shared/lib";

export type ThemePreference = "light" | "dark";

export async function loadThemePreference(signal?: AbortSignal): Promise<ThemePreference | null> {
  const response = await fetch("/api/v1/settings", { credentials: "same-origin", signal });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось загрузить тему"));
  const body = await response.json() as { theme?: unknown };
  if (body.theme !== "light" && body.theme !== "dark") throw new Error("Сервис вернул некорректную тему");
  return body.theme;
}
export async function saveThemePreference(theme: ThemePreference): Promise<ThemePreference> {
  const response = await fetch("/api/v1/settings", {
    body: JSON.stringify({ theme }), credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "PUT",
  });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось сохранить тему"));
  const body = await response.json() as { theme?: unknown };
  if (body.theme !== "light" && body.theme !== "dark") throw new Error("Сервис вернул некорректную тему");
  return body.theme;
}
