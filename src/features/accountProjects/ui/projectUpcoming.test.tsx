// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectUpcoming } from "./projectUpcoming";

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status });
}

function feed(items: unknown[] = [], days = 7) {
  return { days, items, rangeEnd: "2026-10-02T12:00:00Z", rangeStart: "2026-09-25T12:00:00Z" };
}

describe("ProjectUpcoming", () => {
  it("loads items, filters them and hides creation actions without permission", async () => {
    const items = [
      { createdAt: "2026-09-25T10:00:00Z", description: "Подготовить варианты", effectiveAt: "2026-09-26T10:00:00Z", endsAt: null, id: "task-1", kind: "task", location: null, projectId: "project-1", status: "new", title: "Планировка", version: 1 },
      { createdAt: "2026-09-25T10:00:00Z", description: null, effectiveAt: "2026-09-27T10:00:00Z", endsAt: "2026-09-27T11:00:00Z", id: "meeting-1", kind: "meeting", location: "Объект", projectId: "project-1", status: null, title: "Встреча на объекте", version: 1 },
    ];
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => Promise.resolve(response(String(input).endsWith("/settings") ? { upcomingDays: 7 } : feed(items)))));

    render(<ProjectUpcoming canCreate={false} projectId="project-1" />);
    expect(await screen.findByText("Планировка")).toBeTruthy();
    expect(screen.getByText("Встреча на объекте")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Создать задачу" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Создать встречу" })).toBeNull();
    expect(screen.getByRole("button", { name: "Настроить период" })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Задачи" }));
    expect(screen.getByText("Планировка")).toBeTruthy();
    expect(screen.queryByText("Встреча на объекте")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Встречи" }));
    expect(screen.queryByText("Планировка")).toBeNull();
    expect(screen.getByText("Встреча на объекте")).toBeTruthy();
  });

  it("stores and applies a configurable period", async () => {
    const fetchMock = vi.fn().mockImplementation((input: string | URL | Request, init?: RequestInit) => {
      if (String(input).endsWith("/settings")) {
        if (init?.method === "PUT") return Promise.resolve(response({ upcomingDays: JSON.parse(String(init.body)).upcomingDays }));
        return Promise.resolve(response({ upcomingDays: 7 }));
      }
      const days = String(input).includes("days=14") ? 14 : 7;
      return Promise.resolve(response(feed([], days)));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate projectId="project-1" />);
    await screen.findByText("Ближайших событий пока нет");

    fireEvent.click(screen.getByRole("button", { name: "Настроить период" }));
    fireEvent.change(screen.getByLabelText("Количество дней *"), { target: { value: "0" } });
    expect(screen.getByRole("alert").textContent).toContain("от 1 до 90");
    expect(screen.getByRole("button", { name: "Сохранить" })).toHaveProperty("disabled", true);
    fireEvent.change(screen.getByLabelText("Количество дней *"), { target: { value: "14" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    expect(await screen.findByText("Период обновлён: 14 дней")).toBeTruthy();
    expect(screen.getByText("Задачи и встречи на ближайшие 14 дней")).toBeTruthy();
    await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => String(input).includes("days=14"))).toBe(true));
    const savedSettings = fetchMock.mock.calls.find(([input, init]) => String(input).endsWith("/settings") && init?.method === "PUT");
    expect(JSON.parse(String(savedSettings?.[1]?.body))).toEqual({ upcomingDays: 14 });
    expect((savedSettings?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("");
  });

  it("restores a server-side period and renders the read-only empty state", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation((input: string | URL | Request) => Promise.resolve(response(String(input).endsWith("/settings") ? { upcomingDays: 2 } : feed([], 2)))));
    render(<ProjectUpcoming canCreate={false} projectId="project-2" />);
    expect(await screen.findByText("Задачи и встречи на ближайшие 2 дня")).toBeTruthy();
    expect(await screen.findByText("Новые события появятся здесь после назначения организатором проекта.")).toBeTruthy();
  });

  it("retries a failed settings request before loading the project feed", async () => {
    let settingsAttempts = 0;
    const fetchMock = vi.fn((input: string | URL | Request) => {
      if (String(input).endsWith("/settings")) {
        settingsAttempts += 1;
        return Promise.resolve(settingsAttempts === 1 ? response({ error: { message: "Настройки временно недоступны" } }, 503) : response({ upcomingDays: 1 }));
      }
      return Promise.resolve(response(feed([], 1)));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate={false} projectId="project-retry" />);

    expect((await screen.findByRole("alert")).textContent).toContain("Настройки временно недоступны");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));

    expect(await screen.findByText("Задачи и встречи на ближайшие 1 день")).toBeTruthy();
    expect(settingsAttempts).toBe(2);
  });

  it("shows a settings persistence error and an empty selected filter", async () => {
    const item = { createdAt: "2026-09-25T10:00:00Z", description: null, effectiveAt: "2026-09-26T10:00:00Z", endsAt: null, id: "task-only", kind: "task", location: null, projectId: "project-1", status: "new", title: "Единственная задача", version: 1 };
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (String(input).endsWith("/settings")) {
        if (init?.method === "PUT") return Promise.resolve(response({ error: { message: "Период не сохранён" } }, 503));
        return Promise.resolve(response({ upcomingDays: 7 }));
      }
      return Promise.resolve(response(feed([item])));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate projectId="project-save-error" />);
    await screen.findByText("Единственная задача");

    fireEvent.click(screen.getByRole("button", { name: "Встречи" }));
    expect(screen.getByText("В выбранном фильтре событий нет")).toBeTruthy();
    expect(screen.getByText("Выберите другой тип события.")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Настроить период" }));
    fireEvent.change(screen.getByLabelText("Количество дней *"), { target: { value: "4" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Период не сохранён");
  });

  it("creates a task with CSRF and shows it after reloading the feed", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(12, 0, 0, 0);
    const created = { createdAt: new Date().toISOString(), description: "Проверить план", effectiveAt: tomorrow.toISOString(), endsAt: null, id: "task-1", kind: "task", location: null, projectId: "project-1", status: "new", title: "Проверить чертежи", version: 1 };
    let items: unknown[] = [];
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "POST") { items = [created]; return Promise.resolve(response(created, 201)); }
      if (String(input).endsWith("/settings")) return Promise.resolve(response({ upcomingDays: 7 }));
      return Promise.resolve(response(feed(items)));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate projectId="project-1" />);
    await screen.findByText("Ближайших событий пока нет");

    fireEvent.click(screen.getByRole("button", { name: "Создать задачу" }));
    const taskDialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: " Проверить чертежи " } });
    fireEvent.change(screen.getByLabelText("Описание"), { target: { value: "Проверить план" } });
    fireEvent.click(screen.getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.change(screen.getByLabelText("Дедлайн, время *"), { target: { value: "12:00" } });
    fireEvent.click(within(taskDialog).getByRole("button", { name: "Создать задачу" }));

    expect(await screen.findByText("Задача создана")).toBeTruthy();
    expect(await screen.findByText("Проверить чертежи")).toBeTruthy();
    const post = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
    expect(String(post?.[0])).toContain("/projects/project-1/tasks");
    expect((post?.[1]?.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-token");
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({ description: "Проверить план", title: "Проверить чертежи" });
  });

  it("explains an already elapsed task deadline next to the date and time fields", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => Promise.resolve(response(String(input).endsWith("/settings") ? { upcomingDays: 7 } : feed()))));
    render(<ProjectUpcoming canCreate projectId="project-deadline" />);
    await screen.findByText("Ближайших событий пока нет");

    fireEvent.click(screen.getByRole("button", { name: "Создать задачу" }));
    const dialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Проверить планы" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect((await within(dialog).findByRole("alert")).textContent).toBe("Выберите дату дедлайна");

    const today = new Date();
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(today) }));
    fireEvent.change(within(dialog).getByLabelText("Дедлайн, время *"), { target: { value: "00:00" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));

    expect((await within(dialog).findByRole("alert")).textContent).toContain("Выбранный дедлайн уже прошёл");
    expect(within(dialog).getByLabelText("Дедлайн, время *").getAttribute("aria-invalid")).toBe("true");
  });

  it("validates meetings and allows retrying a failed feed", async () => {
    let upcomingAttempts = 0;
    const fetchMock = vi.fn((input: string | URL | Request) => {
      if (String(input).endsWith("/settings")) return Promise.resolve(response({ upcomingDays: 7 }));
      upcomingAttempts += 1;
      return Promise.resolve(upcomingAttempts === 1 ? response({ error: { message: "Сервис временно недоступен" } }, 503) : response(feed()));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис временно недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("Ближайших событий пока нет")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Создать встречу" }));
    const dialog = screen.getByRole("dialog", { name: "Создание встречи" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(screen.getByRole("alert").textContent).toContain("минимум 2 символа");
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: "Встреча" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(screen.getByRole("alert").textContent).toContain("Выберите дату начала встречи");

    const today = new Date();
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Начало, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(today) }));
    fireEvent.change(within(dialog).getByLabelText("Начало, время *"), { target: { value: "00:00" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(screen.getByRole("alert").textContent).toContain("Выбранное время начала уже прошло");
    expect(within(dialog).getByLabelText("Начало, время *").getAttribute("aria-invalid")).toBe("true");
  });

  it("reports a task API error and creates a valid meeting", async () => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(12, 0, 0, 0);
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "POST" && String(input).endsWith("/tasks")) return Promise.resolve(response({ error: { message: "Задачу не удалось сохранить" } }, 503));
      if (init?.method === "POST" && String(input).endsWith("/meetings")) return Promise.resolve(response({ id: "meeting-1" }, 201));
      if (String(input).endsWith("/settings")) return Promise.resolve(response({ upcomingDays: 7 }));
      return Promise.resolve(response(feed()));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectUpcoming canCreate projectId="project-3" />);
    await screen.findByText("Ближайших событий пока нет");

    fireEvent.click(screen.getByRole("button", { name: "Создать задачу" }));
    let dialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Задача" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect((await within(dialog).findByRole("alert")).textContent).toContain("Задачу не удалось сохранить");
    fireEvent.click(within(dialog).getByRole("button", { name: "Отмена" }));

    fireEvent.click(screen.getByRole("button", { name: "Создать встречу" }));
    dialog = screen.getByRole("dialog", { name: "Создание встречи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Встреча" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Начало, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Окончание, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(await screen.findByText("Встреча создана")).toBeTruthy();
  });

  it("validates missing times and requires meeting end after its start", async () => {
    const tomorrow = new Date(); tomorrow.setDate(tomorrow.getDate() + 1); tomorrow.setHours(12, 0, 0, 0);
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => Promise.resolve(response(String(input).endsWith("/settings") ? { upcomingDays: 7 } : feed()))));
    render(<ProjectUpcoming canCreate projectId="project-validation" />);
    await screen.findByText("Ближайших событий пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Создать задачу" }));
    let dialog = screen.getByRole("dialog", { name: "Создание задачи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Задача" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Дедлайн, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.change(within(dialog).getByLabelText("Дедлайн, время *"), { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать задачу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("Укажите время дедлайна");
    fireEvent.click(within(dialog).getByRole("button", { name: "Отмена" }));
    fireEvent.click(screen.getByRole("button", { name: "Создать встречу" }));
    dialog = screen.getByRole("dialog", { name: "Создание встречи" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Встреча" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Начало, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.change(within(dialog).getByLabelText("Начало, время *"), { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("Укажите время начала");
    fireEvent.change(within(dialog).getByLabelText("Начало, время *"), { target: { value: "12:00" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("дату окончания");
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Окончание, дата" }));
    fireEvent.click(within(dialog).getByRole("button", { name: formatLongDate(tomorrow) }));
    fireEvent.change(within(dialog).getByLabelText("Окончание, время *"), { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("Укажите время окончания");
    fireEvent.change(within(dialog).getByLabelText("Окончание, время *"), { target: { value: "11:00" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Создать встречу" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("позже начала");
  });
});

function formatLongDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}
