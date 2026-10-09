// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectMembers } from "./projectMembers";

const customer = { email: "customer@example.com", firstName: "Светлана", joinedAt: "2026-09-24T12:00:00Z", lastName: "Полисмакова", login: "customer", middleName: null, professionalRole: { code: "customer", name: "Заказчик" }, projectRoles: ["customer", "project_admin"], removable: false, userId: "user-1" };
const executor = { email: "anna@example.com", firstName: "Анна", joinedAt: "2026-09-24T13:00:00Z", lastName: "Иванова", login: "anna", middleName: null, professionalRole: { code: "designer", name: "Дизайнер" }, projectRoles: ["executor"], removable: true, userId: "user-2" };
const candidate = { email: "ivan@example.com", firstName: "Иван", lastName: "Петров", login: "ivan", middleName: null, professionalRole: { code: "architect", name: "Архитектор" }, userId: "user-3" };

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.clearAllMocks(); });

function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status }); }

describe("ProjectMembers", () => {
  it("searches, adds and removes project participants", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("member-candidates")) return Promise.resolve(response({ items: [candidate] }));
      if (init?.method === "POST") return Promise.resolve(response({ ...candidate, joinedAt: "2026-09-25T10:00:00Z", projectRoles: ["executor"], removable: true }, 201));
      if (init?.method === "DELETE") return Promise.resolve(new Response(null, { status: 204 }));
      return Promise.resolve(response({ canManage: true, items: [customer, executor] }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMembers projectId="project-1" />);

    expect(await screen.findByText("Светлана Полисмакова")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    const search = screen.getByRole("combobox", { name: "Пользователь" });
    fireEvent.change(search, { target: { value: "ivan" } });
    const option = await screen.findByRole("option", { name: /Иван Петров/ }, { timeout: 1500 });
    fireEvent.click(option);
    fireEvent.click(screen.getByRole("button", { name: "Добавить участника" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, options]) => options?.method === "POST")).toBe(true));
    expect(screen.getAllByText("Иван Петров").length).toBeGreaterThan(0);
    const postCall = fetchMock.mock.calls.find(([, options]) => options?.method === "POST");
    expect((postCall?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-token");

    fireEvent.click(screen.getByRole("button", { name: "Удалить участника Анна Иванова" }));
    expect(screen.getByRole("dialog", { name: "Удаление участника" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(screen.queryByText("Анна Иванова")).toBeNull());
    expect(fetchMock.mock.calls.some(([, options]) => options?.method === "DELETE")).toBe(true);
  });

  it("hides settings without permission and recovers a loading error", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Сервис недоступен" } }, 503))
      .mockResolvedValueOnce(response({ canManage: false, items: [customer] }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMembers projectId="project-1" />);

    expect((await screen.findByRole("alert")).textContent).toContain("Сервис недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("Светлана Полисмакова")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Настроить участников" })).toBeNull();
  });

  it("shows an empty managed project and validates selection before adding", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(response({ canManage: true, items: [] }))));
    render(<ProjectMembers projectId="project-1" />);
    expect(await screen.findByText("В проекте пока нет участников.")).toBeTruthy();
    expect(screen.getByText("Заказчик пока не назначен.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    expect(screen.getByText("Список пока пуст.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Добавить участника" }).hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Закрыть" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("reports candidate search and add failures, then clears errors when the query changes", async () => {
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("member-candidates")) return Promise.reject(new Error("Поиск недоступен"));
      if (init?.method === "POST") return Promise.reject(new Error("Участник не добавлен"));
      return Promise.resolve(response({ canManage: true, items: [customer] }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMembers projectId="project-1" />);
    await screen.findByText("Светлана Полисмакова");
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    const search = screen.getByRole("combobox", { name: "Пользователь" });
    fireEvent.change(search, { target: { value: "iv" } });
    expect(await screen.findByText("Поиск недоступен", {}, { timeout: 1500 })).toBeTruthy();
    fireEvent.change(search, { target: { value: "i" } });
    await waitFor(() => expect(screen.queryByText("Поиск недоступен")).toBeNull());
  });

  it("keeps the removal dialog open when the API rejects the operation and allows cancellation", async () => {
    vi.stubGlobal("fetch", vi.fn((_input: string | URL | Request, init?: RequestInit) => init?.method === "DELETE" ? Promise.reject(new Error("Удаление запрещено")) : Promise.resolve(response({ canManage: true, items: [executor] }))));
    render(<ProjectMembers projectId="project-1" />);
    await screen.findByText("Анна Иванова");
    expect(screen.getByText("Заказчик пока не назначен.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить участника Анна Иванова" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Удаление запрещено");
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog", { name: "Удаление участника" })).toBeNull();
  });

  it("reports an add failure after a candidate was selected", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("member-candidates")) return Promise.resolve(response({ items: [candidate] }));
      if (init?.method === "POST") return Promise.reject(new Error("Добавление запрещено"));
      return Promise.resolve(response({ canManage: true, items: [customer] }));
    }));
    render(<ProjectMembers projectId="project-1" />);
    await screen.findByText("Светлана Полисмакова");
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    fireEvent.change(screen.getByRole("combobox", { name: "Пользователь" }), { target: { value: "ivan" } });
    fireEvent.click(await screen.findByRole("option", { name: /Иван Петров/ }, { timeout: 1500 }));
    fireEvent.click(screen.getByRole("button", { name: "Добавить участника" }));
    expect(await screen.findByText("Добавление запрещено")).toBeTruthy();
  });

  it("uses the generic loading message when a retry rejects with an unknown value", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce("offline");
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMembers projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить участников");
  });

  it("does not present an aborted members retry as an error", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce(new DOMException("cancelled", "AbortError"));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMembers projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("status", { name: "Загрузка участников" })).toBeTruthy();
  });

  it("uses generic messages for non-Error search and removal failures", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (String(input).includes("member-candidates")) return Promise.reject("offline");
      if (init?.method === "DELETE") return Promise.reject("offline");
      return Promise.resolve(response({ canManage: true, items: [executor] }));
    }));
    render(<ProjectMembers projectId="project-1" />);
    await screen.findByText("Анна Иванова");
    fireEvent.click(screen.getByRole("button", { name: "Настроить участников" }));
    fireEvent.change(screen.getByRole("combobox", { name: "Пользователь" }), { target: { value: "iv" } });
    expect(await screen.findByText("Не удалось выполнить поиск пользователей", {}, { timeout: 1500 })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Удалить участника Анна Иванова" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось удалить участника");
  });
});
