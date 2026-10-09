import { readApiError } from "@/shared/lib";

export type FeedbackCategory = "complaint" | "suggestion";

function csrfToken() {
  const cookie = document.cookie.split("; ").find((value) => value.startsWith("arhdesign_csrf=") || value.startsWith("__Host-arhdesign_csrf="));
  return cookie ? decodeURIComponent(cookie.slice(cookie.indexOf("=") + 1)) : "";
}

async function post(path: string, payload: object, fallback: string) {
  const response = await fetch(path, { body: JSON.stringify(payload), credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new Error(await readApiError(response, fallback));
}

export function sendTechnicalFeedback(category: FeedbackCategory, message: string, path: string) {
  return post("/api/v1/technical/feedback", { category, message, path: safePath(path) }, "Не удалось отправить обращение");
}

export function reportFrontendError(message: string, path: string, digest?: string) {
  const normalized = sanitizeMessage(message);
  return post("/api/v1/technical/frontend-errors", { message: normalized, path: safePath(path), fingerprint: fingerprint(`${normalized}|${safePath(path)}`), ...(digest ? { digest: digest.slice(0, 128).replace(/[^A-Za-z0-9._-]/g, "-") } : {}) }, "Не удалось передать сведения об ошибке");
}

export function safePath(value: string) {
  try { const url = new URL(value, window.location.origin); return url.origin === window.location.origin ? (url.pathname || "/").slice(0, 300) : "/account"; }
  catch { return "/account"; }
}

function sanitizeMessage(value: string) {
  return (value || "Неизвестная ошибка интерфейса")
    .replace(/https?:\/\/\S+/gi, "[url]")
    .replace(/[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}/g, "[email]")
    .replace(/bearer\s+\S+/gi, "[credential]")
    .slice(0, 300);
}

function fingerprint(value: string) {
  let first = 2166136261; let second = 2246822519;
  for (let index = 0; index < value.length; index += 1) { first = Math.imul(first ^ value.charCodeAt(index), 16777619); second = Math.imul(second ^ value.charCodeAt(index), 3266489917); }
  return `${(first >>> 0).toString(36).padStart(8, "0")}${(second >>> 0).toString(36).padStart(8, "0")}`;
}
