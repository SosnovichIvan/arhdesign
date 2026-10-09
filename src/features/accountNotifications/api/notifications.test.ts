// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { loadAccountNotifications, markAccountNotificationRead, markAllAccountNotificationsRead } from "./notifications";

afterEach(() => { vi.unstubAllGlobals(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("notifications API", () => {
  it("loads and marks one or all notifications with CSRF", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "theme=dark; __Host-arhdesign_csrf=csrf%20token" });
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ items: [], unreadCount: 0 }), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    await loadAccountNotifications();
    await markAccountNotificationRead("notification/1");
    await markAllAccountNotificationsRead();
    expect(fetchMock.mock.calls[1][0]).toBe("/api/v1/notifications/notification%2F1/read");
    expect(fetchMock.mock.calls[1][1]?.headers).toEqual({ "X-CSRF-Token": "csrf token" });
    expect(fetchMock.mock.calls[2][0]).toBe("/api/v1/notifications/read-all");
  });

  it("uses an empty token and safe endpoint-specific errors", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(new Response("invalid", { status: 500 }))));
    await expect(loadAccountNotifications()).rejects.toThrow("Не удалось загрузить уведомления");
    await expect(markAccountNotificationRead("id")).rejects.toThrow("Не удалось отметить уведомление прочитанным");
    await expect(markAllAccountNotificationsRead()).rejects.toThrow("Не удалось отметить уведомления прочитанными");
  });
});
