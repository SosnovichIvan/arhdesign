// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";

import { AdminUsersApiError, changeAdminUserStatus, loadAdminUsers } from "./users";

const user = { id: "user/1", login: "ivan", email: "ivan@example.com", firstName: "Иван", lastName: null, middleName: null, globalRole: null, professionalRole: { code: "customer", name: "Заказчик" }, status: "active" as const, registeredAt: "2026-09-01T00:00:00Z", lastInteractiveLoginAt: null, version: 3 };

afterEach(() => { vi.unstubAllGlobals(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("admin users API", () => {
  it("builds a bounded filtered query", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], hasMore: false, nextCursor: null }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await loadAdminUsers({ cursor: "next page", identifier: "  sveta  ", status: "disabled" });
    const url = String(fetchMock.mock.calls[0][0]);
    expect(url).toContain("pageSize=25");
    expect(url).toContain("cursor=next+page");
    expect(url).toContain("identifier=sveta");
    expect(url).toContain("status=disabled");
  });

  it("sends optimistic version and either supported status action", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "theme=dark; __Host-arhdesign_csrf=token%20value" });
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify(user), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    await changeAdminUserStatus(user, "disable");
    await changeAdminUserStatus({ ...user, status: "disabled" }, "restore");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/admin/users/user%2F1/disable");
    expect(fetchMock.mock.calls[1][0]).toBe("/api/v1/admin/users/user%2F1/restore");
    expect(fetchMock.mock.calls[0][1]?.headers).toEqual({ "If-Match": '"3"', "X-CSRF-Token": "token value" });
  });

  it("preserves API status and safe error messages", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ error: { message: "Последнего администратора нельзя отключить" } }), { status: 409 }))));
    await expect(changeAdminUserStatus(user, "disable")).rejects.toMatchObject<Partial<AdminUsersApiError>>({ message: "Последнего администратора нельзя отключить", status: 409 });
    await expect(loadAdminUsers({})).rejects.toMatchObject<Partial<AdminUsersApiError>>({ message: "Последнего администратора нельзя отключить", status: 409 });
  });
});
