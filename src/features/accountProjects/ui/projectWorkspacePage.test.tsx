// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectWorkspacePage } from "./projectWorkspacePage";

const replace = vi.fn();
const refresh = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, refresh }) }));

const project = {
  address: "Москва", autoApproveExpenses: true, createdAt: "2026-09-24T12:00:00Z", createdByUserId: "user-1",
  currencyCode: "RUB", customerUserId: "user-1", description: "Интерьер квартиры", id: "project-1", name: "Полянка",
  plannedFinishOn: "2026-12-01", plannedStartOn: "2026-10-01", status: "active", type: "interior_design", version: 3,
};
const emptyUpcoming = {
  days: 7, items: [], rangeEnd: "2026-10-02T12:00:00Z", rangeStart: "2026-09-25T12:00:00Z",
};
const memberList = { canManage: true, items: [{ email: "sveta@example.com", firstName: "Светлана", joinedAt: "2026-09-24T12:00:00Z", lastName: null, login: "sveta", middleName: null, professionalRole: { code: "customer", name: "Заказчик" }, projectRoles: ["customer", "project_admin"], removable: false, userId: "user-1" }] };
const financeSummary = { availableBalanceMinor: 0, canCreateExpense: true, confirmedExpenseMinor: 0, confirmedIncomeMinor: 0, currencyCode: "RUB", pendingExpenseMinor: 0 };

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.clearAllMocks(); });

function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status }); }
function isUpcoming(input: string | URL | Request) { return String(input).includes("/upcoming?"); }
function isProjectSettings(input: string | URL | Request) { return String(input).endsWith("/settings"); }
function isProjectMembers(input: string | URL | Request) { return String(input).endsWith("/members"); }
function isFinanceSummary(input: string | URL | Request) { return String(input).endsWith("/finance-summary"); }
function mockInitialLoad(globalRole: "super_admin" | null = null) {
  vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/api/v1/me") ? Promise.resolve(response({ globalRole, id: "user-1", login: "sveta" })) : isProjectSettings(input) ? Promise.resolve(response({ upcomingDays: 7 })) : isUpcoming(input) ? Promise.resolve(response(emptyUpcoming)) : isProjectMembers(input) ? Promise.resolve(response(memberList)) : isFinanceSummary(input) ? Promise.resolve(response(financeSummary)) : Promise.resolve(response(project))));
}

describe("ProjectWorkspacePage", () => {
  it("loads the project and saves edits using optimistic version", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "PATCH") return Promise.resolve(response({ ...project, name: "Полянка 2", version: 4 }));
      if (String(input).endsWith("/api/v1/me")) return Promise.resolve(response({ globalRole: null, id: "user-1", login: "sveta" }));
      if (isProjectSettings(input)) return Promise.resolve(response({ upcomingDays: 7 }));
      if (isUpcoming(input)) return Promise.resolve(response(emptyUpcoming));
      if (isProjectMembers(input)) return Promise.resolve(response(memberList));
      if (isFinanceSummary(input)) return Promise.resolve(response(financeSummary));
      return Promise.resolve(response(project));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectWorkspacePage projectId="project-1" />);
    expect(await screen.findByRole("heading", { name: "Полянка" })).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Хлебные крошки" }).textContent).toContain("Проекты/Полянка");
    expect(screen.queryByText("Интерьер квартиры")).toBeNull();
    expect(screen.queryByRole("button", { name: "Удалить проект" })).toBeNull();
    const editButton = screen.getByRole("button", { name: "Редактировать проект" });
    expect(editButton.textContent).toBe("");
    expect(document.getElementById(editButton.getAttribute("aria-describedby") ?? "")?.textContent).toBe("Редактировать проект");
    fireEvent.click(editButton);
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: "Полянка 2" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Редактирование проекта" })).toBeNull());
    const patchCall = fetchMock.mock.calls.find(([, options]) => options?.method === "PATCH");
    expect((patchCall?.[1]?.headers as Record<string, string>)["If-Match"]).toBe('"3"');
    expect((patchCall?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-token");
    expect(JSON.parse(String(patchCall?.[1]?.body)).name).toBe("Полянка 2");
  });

  it("lets only a super-admin confirm project deletion", async () => {
    mockInitialLoad("super_admin");
    render(<ProjectWorkspacePage projectId="project-1" />);
    const deleteButton = await screen.findByRole("button", { name: "Удалить проект" });
    expect(deleteButton.textContent).toBe("");
    expect(document.getElementById(deleteButton.getAttribute("aria-describedby") ?? "")?.textContent).toBe("Удалить проект");
    fireEvent.click(deleteButton);
    expect(screen.getByRole("dialog", { name: "Удаление проекта" })).toBeTruthy();
    vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/account"));
    expect(refresh).toHaveBeenCalled();
  });

  it("shows access and version-conflict errors", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ error: { message: "Не найден" } }, 404)));
    const { unmount } = render(<ProjectWorkspacePage projectId="missing" />);
    expect((await screen.findByRole("alert")).textContent).toContain("нет к нему доступа");
    unmount();

    mockInitialLoad();
    render(<ProjectWorkspacePage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Редактировать проект" }));
    vi.mocked(fetch).mockResolvedValueOnce(response({ error: { message: "Версия проекта устарела" } }, 409));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Версия проекта устарела");
  });

  it("covers empty project data and all editable controls", async () => {
    const sparse = { ...project, address: null, autoApproveExpenses: false, customerUserId: null, description: null, plannedFinishOn: null, plannedStartOn: null, status: "draft", type: "unknown" };
    const fetchMock = vi.fn((input: string | URL | Request) => String(input).endsWith("/api/v1/me") ? Promise.resolve(response({ globalRole: null, id: "user-1", login: "sveta" })) : isProjectSettings(input) ? Promise.resolve(response({ upcomingDays: 7 })) : isUpcoming(input) ? Promise.resolve(response(emptyUpcoming)) : isProjectMembers(input) ? Promise.resolve(response({ canManage: true, items: [] })) : isFinanceSummary(input) ? Promise.resolve(response(financeSummary)) : Promise.resolve(response(sparse)));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectWorkspacePage projectId="project-1" />);
    expect(await screen.findByRole("heading", { name: "Полянка" })).toBeTruthy();
    expect(screen.getAllByText("Не указан")).toHaveLength(2);
    expect(screen.getByRole("navigation", { name: "Хлебные крошки" }).textContent).toContain("Полянка");
    expect(await screen.findByText("Заказчик пока не назначен.")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Финансовая сводка" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать проект" }));
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: "x" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(screen.getByRole("alert").textContent).toContain("минимум 2 символа");
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: "Новый проект" } });
    fireEvent.change(screen.getByLabelText("Адрес"), { target: { value: "Новый адрес" } });
    fireEvent.change(screen.getByLabelText("Описание"), { target: { value: "Новое описание" } });
    fireEvent.click(screen.getByLabelText("Автосогласование расходов"));
    fireEvent.click(screen.getByRole("combobox", { name: "Тип" }));
    fireEvent.click(screen.getByRole("option", { name: "Архитектурный проект" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Статус" }));
    fireEvent.click(screen.getByRole("option", { name: "В работе" }));
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog", { name: "Редактирование проекта" })).toBeNull();
  });

  it("validates edited dates and reports update and deletion failures", async () => {
    const noDates = { ...project, plannedFinishOn: null, plannedStartOn: null };
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "PATCH") return Promise.resolve(response({ error: { message: "Конфликт проекта" } }, 409));
      if (init?.method === "DELETE") return Promise.resolve(response({ error: { message: "Удаление временно недоступно" } }, 503));
      if (String(input).endsWith("/api/v1/me")) return Promise.resolve(response({ globalRole: "super_admin", id: "admin-1", login: "sveta" }));
      if (isProjectSettings(input)) return Promise.resolve(response({ upcomingDays: 7 }));
      if (isUpcoming(input)) return Promise.resolve(response(emptyUpcoming));
      if (isProjectMembers(input)) return Promise.resolve(response(memberList));
      if (isFinanceSummary(input)) return Promise.resolve(response(financeSummary));
      return Promise.resolve(response(noDates));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectWorkspacePage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Редактировать проект" }));
    const today = new Date();
    const first = new Date(today.getFullYear(), today.getMonth(), 1);
    const second = new Date(today.getFullYear(), today.getMonth(), 2);
    fireEvent.click(screen.getByRole("combobox", { name: "Завершение" }));
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(first) }));
    fireEvent.click(screen.getByRole("combobox", { name: "Начало" }));
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(second) }));
    expect(screen.getByText("Дата начала не может быть позже даты завершения")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    fireEvent.click(screen.getByRole("button", { name: "Редактировать проект" }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Конфликт проекта");
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить проект" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Удаление временно недоступно");
  });

  it("retries a transport failure and redirects an expired session", async () => {
    let projectAttempts = 0;
    const fetchMock = vi.fn((input: string | URL | Request) => {
      if (String(input).endsWith("/api/v1/me")) return Promise.resolve(response({ globalRole: null, id: "user-1", login: "sveta" }));
      if (isProjectSettings(input)) return Promise.resolve(response({ upcomingDays: 7 }));
      if (isUpcoming(input)) return Promise.resolve(response(emptyUpcoming));
      if (isProjectMembers(input)) return Promise.resolve(response(memberList));
      if (isFinanceSummary(input)) return Promise.resolve(response(financeSummary));
      projectAttempts += 1;
      return projectAttempts <= 2 ? Promise.reject("offline") : Promise.resolve(response(project));
    });
    vi.stubGlobal("fetch", fetchMock);
    const { unmount } = render(<ProjectWorkspacePage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить проект");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("heading", { name: "Полянка" })).toBeTruthy();
    unmount();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ error: { message: "Требуется вход" } }, 401)));
    render(<ProjectWorkspacePage projectId="project-2" />);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
  });

  it("handles every retry failure without exposing transport details", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ error: { message: "Не найден" } }, 404));
    vi.stubGlobal("fetch", fetchMock);
    const { unmount } = render(<ProjectWorkspacePage projectId="missing" />);
    await screen.findByRole("button", { name: "Повторить" });
    fetchMock.mockRejectedValue(new Error("Сеть недоступна"));
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Сеть недоступна");
    fetchMock.mockRejectedValue("offline");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить проект");
    fetchMock.mockResolvedValue(response({ error: { message: "Войдите снова" } }, 401));
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
    unmount();

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ error: { message: "Не найден" } }, 404)));
    render(<ProjectWorkspacePage projectId="aborted" />);
    await screen.findByRole("button", { name: "Повторить" });
    vi.mocked(fetch).mockRejectedValue(new DOMException("aborted", "AbortError"));
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("status", { name: "Загрузка проекта" })).toBeTruthy();
  });

  it("formats single-ended schedules and unknown statuses", async () => {
    const accountResponse = response({ globalRole: null, id: "user-1", login: "sveta" });
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/api/v1/me") ? Promise.resolve(accountResponse.clone()) : isProjectSettings(input) ? Promise.resolve(response({ upcomingDays: 7 })) : isUpcoming(input) ? Promise.resolve(response(emptyUpcoming)) : isProjectMembers(input) ? Promise.resolve(response(memberList)) : isFinanceSummary(input) ? Promise.resolve(response(financeSummary)) : Promise.resolve(response({ ...project, plannedFinishOn: null, status: "archived" }))));
    const { unmount } = render(<ProjectWorkspacePage projectId="start-only" />);
    expect(await screen.findByText("01 окт. 2026 г.")).toBeTruthy();
    expect(screen.getByText("archived")).toBeTruthy();
    unmount();
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/api/v1/me") ? Promise.resolve(response({ globalRole: null, id: "user-1", login: "sveta" })) : isProjectSettings(input) ? Promise.resolve(response({ upcomingDays: 7 })) : isUpcoming(input) ? Promise.resolve(response(emptyUpcoming)) : isProjectMembers(input) ? Promise.resolve(response(memberList)) : isFinanceSummary(input) ? Promise.resolve(response(financeSummary)) : Promise.resolve(response({ ...project, plannedStartOn: null }))));
    render(<ProjectWorkspacePage projectId="finish-only" />);
    expect(await screen.findByText("01 дек. 2026 г.")).toBeTruthy();
  });
});

function formatLongDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}
