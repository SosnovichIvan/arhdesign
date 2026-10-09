// @vitest-environment jsdom

import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GlobalChatsPage } from "./globalChatsPage";

const chatId = "00000000-0000-0000-0000-000000000101";
const currentUserId = "00000000-0000-0000-0000-000000000001";
const memberId = "00000000-0000-0000-0000-000000000002";
const messageID = "00000000-0000-0000-0000-000000000201";
const member = { email: "maria@example.test", firstName: "Мария", joinedAt: "2026-09-28T09:00:00Z", lastName: "Иванова", login: "maria", userId: memberId };
const owner = { email: "anna@example.test", firstName: "Анна", joinedAt: "2026-09-28T09:00:00Z", lastName: null, login: "anna", userId: currentUserId };
const summary = { displayName: "Мария Иванова", id: chatId, kind: "direct", lastActivityAt: "2026-09-28T10:00:00Z", lastMessage: null, members: [owner, member], name: null, unreadCount: 2, version: 1 };

function json(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status })); }

  beforeEach(() => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "visible",
    });
    Element.prototype.scrollIntoView = vi.fn();
  Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-value" });
});

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("GlobalChatsPage", () => {
  it("loads a conversation, clears unread count and sends an idempotent message", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes(`/chats/${chatId}/messages`)) return json({ author: { firstName: "Анна", lastName: null, login: "anna", userId: currentUserId }, body: "Добрый день", chatId, createdAt: "2026-09-28T10:01:00Z", deletedAt: null, editedAt: null, id: "00000000-0000-0000-0000-000000000201", version: 1 }, 201);
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: [], nextCursor: null });
      if (url.includes("/api/v1/chats?pageSize=50")) return json({ hasMore: false, items: [summary], nextCursor: null });
      throw new Error(`unexpected request ${url} ${init?.method ?? "GET"}`);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalChatsPage />);
    expect(await screen.findByRole("heading", { name: "Мария Иванова" })).toBeTruthy();
    expect(await screen.findByText("Сообщений пока нет")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Сообщение"), { target: { value: "Добрый день" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить сообщение" }));
    expect(await screen.findByText("Добрый день")).toBeTruthy();
    const send = fetchMock.mock.calls.find(([input]) => String(input).includes("/messages"));
    expect((send?.[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBeTruthy();
    expect((send?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-value");
  });

  it("creates a personal chat through the shared autocomplete", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("chat-user-candidates")) return json({ items: [member, { ...member, email: "petr@example.test", firstName: "Пётр", login: "petr", userId: "00000000-0000-0000-0000-000000000003" }] });
      if (url === "/api/v1/chats" && init?.method === "POST") return json(summary, 201);
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: [], nextCursor: null });
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      return json({ hasMore: false, items: [], nextCursor: null });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalChatsPage />);
    await screen.findByText("Начните общение");
    fireEvent.click(screen.getAllByRole("button", { name: "Создать чат" })[0]);
    const search = screen.getByRole("combobox", { name: /Собеседник/ });
    fireEvent.change(search, { target: { value: "ma" } });
    expect(await screen.findByRole("option", { name: /Мария Иванова/ })).toBeTruthy();
    fireEvent.click(screen.getByRole("option", { name: /Мария Иванова/ }));
    await screen.findByRole("button", { name: /Удалить Мария/ });
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(await screen.findByRole("heading", { name: "Мария Иванова" })).toBeTruthy();
  });

  it("shows a recoverable loading error", async () => {
    vi.stubGlobal("fetch", vi.fn(() => json({ error: { message: "Сервис временно недоступен" } }, 503)));
    render(<GlobalChatsPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис временно недоступен");
    expect(screen.getByRole("button", { name: "Повторить" })).toBeTruthy();
  });

  it("creates a named group, removes a participant and reports create errors", async () => {
    const second = { ...member, email: "petr@example.test", firstName: "Пётр", login: "petr", userId: "00000000-0000-0000-0000-000000000003" };
    let createFails = true;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("chat-user-candidates")) return json({ items: [member, second] });
      if (url === "/api/v1/chats" && init?.method === "POST") return createFails ? json({ error: { message: "Чат уже существует" } }, 409) : json({ ...summary, displayName: "Команда", kind: "group", name: "Команда" }, 201);
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: { ...summary, displayName: "Команда", kind: "group", name: "Команда" }, currentUserId, hasMore: false, messages: [], nextCursor: null });
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      return json({ hasMore: false, items: [], nextCursor: null });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalChatsPage />);
    await screen.findByText("Начните общение");
    fireEvent.click(screen.getAllByRole("button", { name: "Создать чат" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Групповой" }));
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(screen.getByRole("alert").textContent).toContain("не менее двух");
    fireEvent.change(screen.getByLabelText(/Название чата/), { target: { value: "Команда" } });
    const search = screen.getByRole("combobox", { name: /Участники/ });
    fireEvent.change(search, { target: { value: "ma" } });
    fireEvent.click(await screen.findByRole("option", { name: /Мария/ }));
    await screen.findByRole("button", { name: /Удалить Мария/ });
    await waitFor(() => expect((search as HTMLInputElement).value).toBe(""));
    fireEvent.change(search, { target: { value: "pe" } });
    fireEvent.click(await screen.findByRole("option", { name: /Пётр/ }));
    await screen.findByRole("button", { name: /Удалить Пётр/ });
    fireEvent.click(screen.getByRole("button", { name: /Удалить Пётр/ }));
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(screen.getByRole("alert").textContent).toContain("не менее двух");
    await waitFor(() => expect((search as HTMLInputElement).value).toBe(""));
    fireEvent.change(search, { target: { value: "pe" } });
    fireEvent.click(await screen.findByRole("option", { name: /Пётр/ }));
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Чат уже существует");
    createFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(await screen.findByRole("heading", { name: "Команда" })).toBeTruthy();
  });

  it("shows a conversation error and retries a failed message with the same id", async () => {
    let conversationFails = true;
    let sendFails = true;
    const sentIds: string[] = [];
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes(`/chats/${chatId}/messages`)) {
        const payload = JSON.parse(String(init?.body)); sentIds.push(payload.clientMessageId);
        if (sendFails) return json({ error: { message: "Нет связи" } }, 503);
        return json({ author: { firstName: "Анна", lastName: null, login: "anna", userId: currentUserId }, body: payload.body, chatId, createdAt: "2026-09-28T10:01:00Z", deletedAt: null, editedAt: null, id: messageID, version: 1 }, 201);
      }
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      if (url.includes(`/chats/${chatId}?`)) {
        if (conversationFails) return json({ error: { message: "Чат недоступен" } }, 503);
        return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: [], nextCursor: null });
      }
      return json({ hasMore: false, items: [summary], nextCursor: null });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalChatsPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("Чат не открылся");
    conversationFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await screen.findByText("Сообщений пока нет");
    fireEvent.change(screen.getByLabelText("Сообщение"), { target: { value: "Повтор" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить сообщение" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Нет связи");
    sendFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Отправить сообщение" }));
    expect(await screen.findByText("Повтор")).toBeTruthy();
    expect(sentIds[0]).toBe(sentIds[1]);
  });

  it("renders group and personal message variants and blocks sending without permission", async () => {
    const otherMessage = { author: { firstName: null, lastName: null, login: "maria", userId: memberId }, body: "Чужое сообщение", chatId, createdAt: "2026-09-28T10:00:00Z", deletedAt: null, editedAt: null, id: messageID, version: 1 };
    const own = { ...otherMessage, author: { firstName: "Анна", lastName: null, login: "anna", userId: currentUserId }, body: "Моё сообщение", id: "own-message" };
    const group = { ...summary, displayName: "Команда", kind: "group", lastMessage: otherMessage, members: [owner, member, { ...member, userId: "third" }, { ...member, userId: "fourth" }, { ...member, userId: "fifth" }], name: "Команда", unreadCount: 0 };
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => { const url = String(input); if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 })); if (url.includes(`/chats/${chatId}?`)) return json({ canSend: false, chat: group, currentUserId, hasMore: false, messages: [otherMessage, own], nextCursor: null }); return json({ hasMore: false, items: [group], nextCursor: null }); }));
    render(<GlobalChatsPage />);
    expect(await screen.findByText("Групповой чат")).toBeTruthy();
    expect(screen.getByText("5 участников")).toBeTruthy();
    expect(screen.getByText("@maria")).toBeTruthy();
    expect(screen.getByText("Вы")).toBeTruthy();
    expect(screen.getByLabelText("Сообщение").hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("button", { name: "Отправить сообщение" }).hasAttribute("disabled")).toBe(true);
  });

  it("validates long messages and submits on Enter when sending is allowed", async () => {
    const sent: string[] = [];
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/messages")) { const payload = JSON.parse(String(init?.body)); sent.push(payload.body); return json({ author: { firstName: "Анна", lastName: null, login: "anna", userId: currentUserId }, body: payload.body, chatId, createdAt: "2026-09-28T10:01:00Z", deletedAt: null, editedAt: null, id: "sent", version: 1 }, 201); }
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: [], nextCursor: null });
      return json({ hasMore: false, items: [summary], nextCursor: null });
    }));
    render(<GlobalChatsPage />);
    await screen.findByText("Сообщений пока нет");
    const input = screen.getByLabelText("Сообщение");
    fireEvent.change(input, { target: { value: "x".repeat(5001) } });
    fireEvent.submit(input.closest("form")!);
    expect(screen.getByRole("alert").textContent).toContain("5000");
    fireEvent.change(input, { target: { value: "Через Enter" } });
    fireEvent.keyDown(input, { key: "Enter", shiftKey: false });
    await waitFor(() => expect(sent).toEqual(["Через Enter"]));
  });

  it("validates direct and group creation and closes the dialog", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => String(input).includes("chat-user-candidates") ? Promise.reject(new Error("offline")) : json({ hasMore: false, items: [], nextCursor: null })));
    render(<GlobalChatsPage />);
    await screen.findByText("Начните общение");
    fireEvent.click(screen.getAllByRole("button", { name: "Создать чат" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(screen.getByRole("alert").textContent).toContain("одного пользователя");
    fireEvent.click(screen.getByRole("button", { name: "Групповой" }));
    fireEvent.click(screen.getByRole("button", { name: "Создать" }));
    expect(screen.getByRole("alert").textContent).toContain("не менее двух");
    fireEvent.change(screen.getByRole("combobox", { name: /Участники/ }), { target: { value: "xx" } });
    expect(await screen.findByText("Пользователи не найдены")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog", { name: "Создание чата" })).toBeNull();
  });

  it("polls the selected conversation quietly and merges new messages", async () => {
    let poll: (() => void) | undefined;
    vi.spyOn(window, "setInterval").mockImplementation(((callback: TimerHandler, delay?: number) => {
      if (delay === 10_000) poll = callback as () => void;
      return 1;
    }) as typeof window.setInterval);
    vi.spyOn(window, "clearInterval").mockImplementation(() => undefined);
    let includeIncoming = false;
    const incoming = { author: { firstName: "Мария", lastName: "Иванова", login: "maria", userId: memberId }, body: "Новое", chatId, createdAt: new Date().toISOString(), deletedAt: null, editedAt: null, id: "incoming", version: 1 };
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: includeIncoming ? [incoming] : [], nextCursor: null });
      return json({ hasMore: false, items: [summary], nextCursor: null });
    }));
    render(<GlobalChatsPage />);
    await screen.findByText("Сообщений пока нет");
    expect(poll).toBeTypeOf("function");
    includeIncoming = true;
    await act(async () => {
      poll?.();
      await Promise.resolve();
    });
    expect(await screen.findByText("Новое")).toBeTruthy();
  });

  it("falls back to generic errors for non-Error chat failures", async () => {
    const second = { ...summary, displayName: "Пётр", id: "chat-2" };
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/chats/chat-2?")) return Promise.reject("offline");
      if (url.includes(`/chats/${chatId}?`)) return json({ canSend: true, chat: summary, currentUserId, hasMore: false, messages: [], nextCursor: null });
      if (url.endsWith(`/chats/${chatId}/read`)) return Promise.resolve(new Response(null, { status: 204 }));
      return json({ hasMore: false, items: [summary, second], nextCursor: null });
    }));
    render(<GlobalChatsPage />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: /Пётр/ }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось открыть чат");
  });

  it("ignores an aborted initial chat-list request", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new DOMException("cancelled", "AbortError"))));
    render(<GlobalChatsPage />);
    expect(await screen.findByRole("status", { name: "Загрузка чатов" })).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("ignores an aborted conversation request", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => String(input).includes(`/chats/${chatId}?`)
      ? Promise.reject(new DOMException("cancelled", "AbortError"))
      : json({ hasMore: false, items: [summary], nextCursor: null })));
    render(<GlobalChatsPage />);
    expect(await screen.findByRole("status", { name: "Загрузка переписки" })).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
