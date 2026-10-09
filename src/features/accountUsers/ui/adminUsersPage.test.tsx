// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AdminUsersPage } from "./adminUsersPage";

const router = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const user = {
  id: "00000000-0000-0000-0000-000000000002", login: "client.one", email: "client@example.com", firstName: "Анна", lastName: "Иванова", middleName: null,
  globalRole: null, professionalRole: { code: "customer", name: "Заказчик" }, status: "active" as const,
  registeredAt: "2026-09-20T10:00:00Z", lastInteractiveLoginAt: null, version: 1,
};

afterEach(() => { cleanup(); vi.unstubAllGlobals(); router.replace.mockReset(); document.cookie = "arhdesign_csrf=; Max-Age=0"; });

describe("AdminUsersPage", () => {
  it("renders users and confirms an accessible disable action with optimistic version", async () => {
    document.cookie = "arhdesign_csrf=csrf-token";
    const fetchMock = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>((url) => Promise.resolve(new Response(JSON.stringify(url.includes("/disable") ? { ...user, status: "disabled", version: 2 } : { items: [user], hasMore: false, nextCursor: null }), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    expect(await screen.findByText("Иванова Анна")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Отключить пользователя client.one" }));
    expect(screen.getByRole("dialog", { name: "Отключение доступа" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Отключить" })).toBe(document.activeElement);
    fireEvent.click(screen.getByRole("button", { name: "Отключить" }));
    await waitFor(() => expect(screen.getByText("Отключён")).toBeTruthy());
    expect(fetchMock.mock.calls[1][0]).toContain(`/api/v1/admin/users/${user.id}/disable`);
    expect(fetchMock.mock.calls[1][1]?.headers).toMatchObject({ "If-Match": '"1"', "X-CSRF-Token": "csrf-token" });
  });

  it("redirects a non-global account and shows retryable transport errors", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Недостаточно прав" } }), { status: 403 }))
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [], hasMore: false, nextCursor: null }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const view = render(<AdminUsersPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/account"));
    view.unmount();
    render(<AdminUsersPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("offline");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("Пользователи не найдены")).toBeTruthy();
  });

  it("filters, appends another page and renders all global roles and account states", async () => {
    const disabled = { ...user, id: "user-2", login: "tech", firstName: "Иван", lastName: null, globalRole: "technical_admin" as const, status: "disabled" as const, lastInteractiveLoginAt: "2026-09-25T12:00:00Z" };
    const pending = { ...user, id: "user-3", login: "owner", globalRole: "super_admin" as const, status: "pending_verification" as const };
    const fetchMock = vi.fn((url: string) => Promise.resolve(new Response(JSON.stringify(url.includes("cursor=next") ? { items: [pending], hasMore: false, nextCursor: null } : { items: [disabled], hasMore: true, nextCursor: "next" }), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    expect(await screen.findByText("Технический администратор")).toBeTruthy();
    expect(screen.getByText("Отключён")).toBeTruthy();
    expect(screen.queryByText("Ещё не входил")).toBeNull();
    fireEvent.change(screen.getByPlaceholderText("sveta.design или name@example.com"), { target: { value: " owner " } });
    fireEvent.click(screen.getByRole("button", { name: "Найти" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => String(url).includes("identifier=owner"))).toBe(true));
    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
    expect(await screen.findByText("Супер-администратор")).toBeTruthy();
    expect(screen.getByText("Не подтверждён")).toBeTruthy();
  });

  it("restores a disabled account and reports a failed status change", async () => {
    document.cookie = "arhdesign_csrf=csrf-token";
    const disabled = { ...user, status: "disabled" as const };
    let mutationFails = false;
    const fetchMock = vi.fn((url: string) => {
      if (url.includes("/restore")) return mutationFails ? Promise.reject(new Error("Изменение не сохранено")) : Promise.resolve(new Response(JSON.stringify({ ...user, version: 2 }), { status: 200 }));
      if (url.includes("/disable")) return Promise.reject(new Error("Изменение не сохранено"));
      return Promise.resolve(new Response(JSON.stringify({ items: [disabled], hasMore: false, nextCursor: null }), { status: 200 }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Восстановить пользователя client.one" }));
    expect(screen.getByRole("dialog", { name: "Восстановление доступа" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Восстановить" }));
    expect(await screen.findByRole("button", { name: "Отключить пользователя client.one" })).toBeTruthy();
    mutationFails = true;
    fireEvent.click(screen.getByRole("button", { name: "Отключить пользователя client.one" }));
    fireEvent.click(screen.getByRole("button", { name: "Отключить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Изменение не сохранено");
  });

  it("redirects an unauthenticated account to login", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response(JSON.stringify({ error: { message: "Войдите" } }), { status: 401 }))));
    render(<AdminUsersPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/?auth=login"));
  });

  it("applies a status filter and redirects when a retry discovers an expired session", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Войдите" } }), { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/?auth=login"));
  });

  it("redirects when a retry discovers insufficient global permissions", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce("offline")
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Недостаточно прав" } }), { status: 403 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить пользователей");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/account"));
  });

  it("loads a selected account status from the filter", async () => {
    const fetchMock = vi.fn<(url: string) => Promise<Response>>(() => Promise.resolve(new Response(JSON.stringify({ items: [], hasMore: false, nextCursor: null }), { status: 200 })));
    vi.stubGlobal("fetch", fetchMock);
    render(<AdminUsersPage />);
    await screen.findByText("Пользователи не найдены");
    fireEvent.click(screen.getByRole("combobox", { name: "Статус" }));
    fireEvent.click(screen.getByRole("option", { name: "Активные" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => String(url).includes("status=active"))).toBe(true));
  });
});
