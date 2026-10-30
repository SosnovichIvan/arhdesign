// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AccountNotificationCenter } from "./accountNotificationCenter";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

beforeEach(() => Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" }));
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

describe("AccountNotificationCenter", () => {
  it("shows events from different modules in one inbox and marks an opened notification as read", async () => {
	const items = [
	  { body: "Загружен план.pdf", createdAt: "2026-09-30T10:01:00Z", eventType: "project.document.uploaded", href: "/account/projects/project-1/documents", id: "notification-2", projectId: "project-1", readAt: null, title: "Изменение документации" },
	  { body: "Анна написала в обсуждении", createdAt: "2026-09-30T10:00:00Z", eventType: "project.context_chat.message_created", href: "/account/projects/project-1/chat/chat-1", id: "notification-1", projectId: "project-1", readAt: null, title: "Новое сообщение" },
	];
	const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => init?.method === "POST" ? Promise.resolve(new Response(null, { status: 204 })) : Promise.resolve(new Response(JSON.stringify({ items, unreadCount: 2 }), { headers: { "Content-Type": "application/json" }, status: 200 })));
	vi.stubGlobal("fetch", fetchMock);
	render(<AccountNotificationCenter />);
	expect(await screen.findByRole("button", { name: "Уведомления, непрочитанных: 2" })).toBeTruthy();
	fireEvent.click(screen.getByRole("button", { name: "Уведомления, непрочитанных: 2" }));
	const documentNotification = screen.getByRole("link", { name: /Изменение документации/ });
	expect(documentNotification.getAttribute("href")).toBe("/account/projects/project-1/documents");
	fireEvent.click(documentNotification);
	await waitFor(() => expect(fetchMock.mock.calls.some(([url, options]) => String(url).endsWith("/notifications/notification-2/read") && options?.method === "POST")).toBe(true));
  });

  it("marks all events as read and caps the visible badge", async () => {
    const items = [{ body: "Событие", createdAt: "2026-09-30T10:00:00Z", eventType: "project.updated", href: "/account", id: "notification-1", projectId: "project-1", readAt: null, title: "Проект обновлён" }];
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => init?.method === "POST" ? Promise.resolve(new Response(null, { status: 204 })) : Promise.resolve(new Response(JSON.stringify({ items, unreadCount: 120 }), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountNotificationCenter />);
    const trigger = await screen.findByRole("button", { name: "Уведомления, непрочитанных: 120" });
    expect(trigger.textContent).toContain("99+");
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("button", { name: "Прочитать все" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => String(url).endsWith("/notifications/read-all"))).toBe(true));
    expect(screen.getByRole("button", { name: "Уведомления" })).toBeTruthy();
  });

  it("renders an empty inbox and reports refresh failures", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [], unreadCount: 0 }), { status: 200 }))
      .mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountNotificationCenter />);
    const trigger = await screen.findByRole("button", { name: "Уведомления" });
    fireEvent.click(trigger);
    expect(screen.getByText("Новых событий пока нет.")).toBeTruthy();
    fireEvent(window, new Event("focus"));
    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Не удалось обновить уведомления"));
  });

  it("announces a newly increased unread count after the initial refresh", async () => {
    const item = { body: "Событие", createdAt: "2026-09-30T10:00:00Z", eventType: "project.updated", href: "/account", id: "notification-1", projectId: "project-1", readAt: null, title: "Проект обновлён" };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [item], unreadCount: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [item, { ...item, id: "notification-2" }], unreadCount: 2 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountNotificationCenter />);
    await screen.findByRole("button", { name: "Уведомления, непрочитанных: 1" });
    fireEvent(window, new Event("focus"));
    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Появилось новое уведомление"));
  });

  it("ignores an aborted refresh", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new DOMException("cancelled", "AbortError"))));
    render(<AccountNotificationCenter />);
    await waitFor(() => expect(screen.getByRole("status").textContent).toBe(""));
    expect(screen.getByRole("button", { name: "Уведомления" })).toBeTruthy();
  });

  it("keeps an already-read item at zero and refreshes after a failed read request", async () => {
    const item = { body: "Событие", createdAt: "2026-09-30T10:00:00Z", eventType: "project.updated", href: "/account", id: "notification-1", projectId: "project-1", readAt: "2026-09-30T10:05:00Z", title: "Проект обновлён" };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [item], unreadCount: 0 }), { status: 200 }))
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [item], unreadCount: 0 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountNotificationCenter />);
    const trigger = await screen.findByRole("button", { name: "Уведомления" });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("link", { name: /Проект обновлён/ }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(screen.getByRole("button", { name: "Уведомления" })).toBeTruthy();
  });
});
