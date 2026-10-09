import { readApiError } from "@/shared/lib";

import type { CreateGlobalChatInput, GlobalChatCandidate, GlobalChatConversation, GlobalChatMessage, GlobalChatPage, GlobalChatSummary } from "../model/types";

export class GlobalChatApiError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

function csrfToken() {
  const cookie = document.cookie.split("; ").find((value) => value.startsWith("arhdesign_csrf=") || value.startsWith("__Host-arhdesign_csrf="));
  return cookie ? decodeURIComponent(cookie.slice(cookie.indexOf("=") + 1)) : "";
}

async function expectJson<T>(response: Response, fallback: string): Promise<T> {
  if (!response.ok) throw new GlobalChatApiError(await readApiError(response, fallback), response.status);
  return response.json() as Promise<T>;
}

export async function loadGlobalChats(signal?: AbortSignal): Promise<GlobalChatPage> {
  return expectJson(await fetch("/api/v1/chats?pageSize=50", { credentials: "same-origin", signal }), "Не удалось загрузить чаты");
}

export async function searchGlobalChatCandidates(query: string, signal?: AbortSignal): Promise<GlobalChatCandidate[]> {
  const response = await fetch(`/api/v1/chat-user-candidates?query=${encodeURIComponent(query)}`, { credentials: "same-origin", signal });
  return (await expectJson<{ items: GlobalChatCandidate[] }>(response, "Не удалось найти пользователей")).items;
}

export async function createGlobalChat(input: CreateGlobalChatInput): Promise<GlobalChatSummary> {
  const response = await fetch("/api/v1/chats", {
    body: JSON.stringify(input),
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID(), "X-CSRF-Token": csrfToken() },
    method: "POST",
  });
  return expectJson(response, "Не удалось создать чат");
}

export async function loadGlobalChat(chatId: string, before?: string, signal?: AbortSignal): Promise<GlobalChatConversation> {
  const query = new URLSearchParams({ pageSize: "50" });
  if (before) query.set("before", before);
  return expectJson(await fetch(`/api/v1/chats/${encodeURIComponent(chatId)}?${query}`, { credentials: "same-origin", signal }), "Не удалось загрузить переписку");
}

export async function sendGlobalChatMessage(chatId: string, body: string, clientMessageId: string): Promise<GlobalChatMessage> {
  const response = await fetch(`/api/v1/chats/${encodeURIComponent(chatId)}/messages`, {
    body: JSON.stringify({ body, clientMessageId }),
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "Idempotency-Key": clientMessageId, "X-CSRF-Token": csrfToken() },
    method: "POST",
  });
  return expectJson(response, "Не удалось отправить сообщение");
}

export async function markGlobalChatRead(chatId: string): Promise<void> {
  const response = await fetch(`/api/v1/chats/${encodeURIComponent(chatId)}/read`, { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new GlobalChatApiError(await readApiError(response, "Не удалось отметить чат прочитанным"), response.status);
}
