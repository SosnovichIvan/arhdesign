// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectTasksPage } from "./projectTasksPage";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

const project = { address: null, autoApproveExpenses: true, createdAt: "2026-09-24T10:00:00Z", createdByUserId: "user-1", currencyCode: "RUB", customerUserId: "user-1", description: null, id: "project-1", name: "Полянка", plannedFinishOn: null, plannedStartOn: null, status: "active", type: "interior_design", version: 1 };
const member = { email: "anna@example.com", firstName: "Анна", joinedAt: "2026-09-24T10:00:00Z", lastName: "Иванова", login: "anna", middleName: null, professionalRole: { code: "designer", name: "Дизайнер" }, projectRoles: ["executor"], removable: true, userId: "user-2" };
const task = { assignees: [{ firstName: "Анна", lastName: "Иванова", login: "anna", userId: "user-2" }], canChangeStatus: true, canEdit: true, completedAt: null, createdAt: "2026-09-24T10:00:00Z", description: "Подготовить комплект", dueAt: "2027-10-10T12:00:00Z", id: "task-1", projectId: "project-1", startedAt: null, status: "new", title: "Рабочие чертежи", version: 1 };

afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });
function response(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status })); }

describe("ProjectTasksPage", () => {
  it("shows tasks, filters them and updates status", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "PATCH") return response({ ...task, startedAt: "2026-09-26T10:00:00Z", status: "in_progress", version: 2 });
      if (url.endsWith("/tasks")) return response({ canCreate: true, items: [task] });
      if (url.endsWith("/members")) return response({ canManage: true, items: [member] });
      if (url.endsWith("/me")) return response({ globalRole: null, id: "user-2", login: "anna" });
      return response(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectTasksPage projectId="project-1" />);
    expect(await screen.findByText("Рабочие чертежи")).toBeTruthy();
    expect(screen.getByLabelText("Область задач").textContent).toContain("Все");
    fireEvent.click(screen.getByRole("button", { name: "Настроить фильтры" }));
    const filterDialog = screen.getByRole("dialog", { name: "Расширенные фильтры задач" });
    fireEvent.click(within(filterDialog).getByRole("combobox", { name: "Статус" }));
    fireEvent.click(screen.getByRole("option", { name: "Новая" }));
    fireEvent.click(within(filterDialog).getByRole("button", { name: "Сохранить" }));
    expect(screen.getByRole("button", { name: "Настроить фильтры, применено: 1" })).toBeTruthy();
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию или описанию"), { target: { value: "нет совпадений" } });
    expect(screen.getByText("По выбранным фильтрам задач нет")).toBeTruthy();
    fireEvent.change(screen.getByPlaceholderText("Поиск по названию или описанию"), { target: { value: "чертежи" } });
    fireEvent.click(screen.getByRole("combobox", { name: "Изменить статус" }));
    fireEvent.click(screen.getByRole("option", { name: "В работе" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url, options]) => String(url).endsWith("/tasks/task-1") && options?.method === "PATCH")).toBe(true));
    expect(await screen.findByText("Статус задачи обновлён")).toBeTruthy();
  });

  it("creates a task with an assignee", async () => {
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "POST") return response({ createdAt: "2026-09-26T10:00:00Z", description: null, effectiveAt: "2027-10-10T12:00:00Z", endsAt: null, id: "task-2", kind: "task", location: null, projectId: "project-1", status: "new", title: "Новая задача", version: 1 }, 201);
      if (url.endsWith("/tasks")) return response({ canCreate: true, items: [] });
      if (url.endsWith("/members")) return response({ canManage: true, items: [member] });
      if (url.endsWith("/me")) return response({ globalRole: null, id: "user-1", login: "owner" });
      return response(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectTasksPage projectId="project-1" />);
    fireEvent.click((await screen.findAllByRole("button", { name: "Создать задачу" }))[0]);
    const dialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Новая задача" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Ответственный" }));
    fireEvent.click(screen.getByRole("option", { name: /Анна Иванова/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect(screen.getByRole("alert").textContent).toContain("будущие дату и время");
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(screen.getByRole("button", { name: "Следующий месяц" }));
    const nextMonth = new Date();
    nextMonth.setDate(1);
    nextMonth.setMonth(nextMonth.getMonth() + 1);
    const nextMonthLabel = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(nextMonth);
    fireEvent.click(screen.getByRole("button", { name: nextMonthLabel }));
    fireEvent.change(within(dialog).getByLabelText("Дедлайн, время *"), { target: { value: "12:00" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, options]) => options?.method === "POST" && String(options.body).includes('"assigneeUserIds":["user-2"]'))).toBe(true));
  });

  it("covers mine, deadline, assignee and sort filters across task states", async () => {
    const now = Date.now();
    const tasks = [
      { ...task, dueAt: new Date(now - 86_400_000).toISOString(), id: "overdue", status: "changes_requested", title: "Просроченная", canEdit: false },
      { ...task, dueAt: new Date(now + 86_400_000).toISOString(), id: "mine", status: "in_progress", title: "Моя ближайшая", startedAt: new Date(now - 3_600_000).toISOString(), canEdit: false },
      { ...task, assignees: [], completedAt: new Date(now).toISOString(), createdAt: new Date(now + 1000).toISOString(), dueAt: new Date(now + 20 * 86_400_000).toISOString(), id: "accepted", status: "accepted", title: "Дальняя принята", canChangeStatus: false },
      { ...task, assignees: [{ firstName: null, lastName: null, login: "guest", userId: "user-3" }], dueAt: new Date(now + 2 * 86_400_000).toISOString(), id: "review", status: "review", title: "Проверка" },
    ];
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/tasks")) return response({ canCreate: false, items: tasks }); if (url.endsWith("/members")) return response({ canManage: false, items: [member] }); if (url.endsWith("/me")) return response({ id: "user-2", login: "anna" }); return response(project); }));
    render(<ProjectTasksPage projectId="project-1" />);
    expect(await screen.findByText("Просроченная")).toBeTruthy();
    expect(screen.getByText("Просрочено")).toBeTruthy();
    expect(screen.getByText(/Старт:/)).toBeTruthy();
    expect(screen.getByText(/Завершение:/)).toBeTruthy();
    expect(screen.getByText("Только просмотр")).toBeTruthy();
    expect(screen.getByText("Ответственные: не назначены")).toBeTruthy();
    expect(screen.getByText("Ответственные: @guest")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Мои" }));
    expect(screen.queryByText("Дальняя принята")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Настроить фильтры" }));
    const dialog = screen.getByRole("dialog", { name: "Расширенные фильтры задач" });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Срок" }));
    fireEvent.click(screen.getByRole("option", { name: "Ближайшие 7 дней" }));
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Ответственный" }));
    fireEvent.click(screen.getByRole("option", { name: /Анна Иванова/ }));
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Сортировка" }));
    fireEvent.click(screen.getByRole("option", { name: "Сначала поздние" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Сохранить" }));
    expect(screen.getByText("Моя ближайшая")).toBeTruthy();
    expect(screen.queryByText("Просроченная")).toBeNull();
  });

  it("shows a load error, retries to an empty read-only state and opens the empty action only when allowed", async () => {
    let failed = true;
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => {
      const url = String(input);
      if (failed && url.endsWith("/tasks")) { failed = false; return Promise.reject(new Error("Задачи недоступны")); }
      if (url.endsWith("/tasks")) return response({ canCreate: false, items: [] });
      if (url.endsWith("/members")) return response({ canManage: false, items: [] });
      if (url.endsWith("/me")) return response({ id: "user-1", login: "owner" });
      return response(project);
    }));
    render(<ProjectTasksPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Задачи недоступны");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("В проекте пока нет задач")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Создать задачу" })).toBeNull();
  });

  it("reports status update and task creation errors", async () => {
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "PATCH") return Promise.reject(new Error("Статус не сохранён"));
      if (init?.method === "POST") return Promise.reject(new Error("Задача не создана"));
      if (url.endsWith("/tasks")) return response({ canCreate: true, items: [task] });
      if (url.endsWith("/members")) return response({ canManage: true, items: [member] });
      if (url.endsWith("/me")) return response({ id: "user-2", login: "anna" });
      return response(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectTasksPage projectId="project-1" />);
    await screen.findByText("Рабочие чертежи");
    fireEvent.click(screen.getByRole("combobox", { name: "Изменить статус" }));
    fireEvent.click(screen.getByRole("option", { name: "В работе" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Статус не сохранён");
    fireEvent.click(screen.getByRole("button", { name: "Создать задачу" }));
    const dialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("название");
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Новая задача" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("ответственного");
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Ответственный" }));
    fireEvent.click(screen.getByRole("option", { name: /Анна Иванова/ }));
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(screen.getByRole("button", { name: "Следующий месяц" }));
    const nextMonth = new Date(); nextMonth.setDate(1); nextMonth.setMonth(nextMonth.getMonth() + 1);
    fireEvent.click(screen.getByRole("button", { name: new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(nextMonth) }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect((await within(dialog).findByRole("alert")).textContent).toContain("Задача не создана");
  });

  it("resets draft filters and exposes only permitted status transitions", async () => {
    const restricted = { ...task, canEdit: false, status: "review" };
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/tasks")) return response({ canCreate: false, items: [restricted] }); if (url.endsWith("/members")) return response({ canManage: false, items: [member] }); if (url.endsWith("/me")) return response({ id: "user-2", login: "anna" }); return response(project); }));
    render(<ProjectTasksPage projectId="project-1" />);
    await screen.findByText("Рабочие чертежи");
    fireEvent.click(screen.getByRole("combobox", { name: "Изменить статус" }));
    expect(screen.getByRole("option", { name: "На проверке" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Нужны доработки" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Принята" })).toBeTruthy();
    fireEvent.keyDown(screen.getByRole("combobox", { name: "Изменить статус" }), { key: "Escape" });
    fireEvent.click(screen.getByRole("button", { name: "Настроить фильтры" }));
    const dialog = screen.getByRole("dialog", { name: "Расширенные фильтры задач" });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Статус" }));
    fireEvent.click(screen.getByRole("option", { name: "Принята" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Сбросить" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Сохранить" }));
    expect(screen.getByRole("button", { name: "Настроить фильтры" })).toBeTruthy();
  });
});
