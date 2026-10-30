// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GlobalProjectsCalendarPage } from "./globalProjectsCalendarPage";

const projects = [
  {
    id: "00000000-0000-0000-0000-000000000101",
    items: [
      { createdAt: "2026-09-01T08:00:00Z", description: null, effectiveAt: "2026-09-15T10:00:00Z", endsAt: null, id: "00000000-0000-0000-0000-000000000201", kind: "task", location: null, projectId: "00000000-0000-0000-0000-000000000101", status: "new", title: "Подготовить планы", version: 1 },
      { createdAt: "2026-09-01T08:00:00Z", description: null, effectiveAt: "2026-09-18T12:00:00Z", endsAt: "2026-09-18T13:00:00Z", id: "00000000-0000-0000-0000-000000000202", kind: "meeting", location: "Объект", projectId: "00000000-0000-0000-0000-000000000101", status: null, title: "Встреча на объекте", version: 1 },
    ],
    name: "Полянка",
    plannedFinishOn: "2026-11-30",
    plannedStartOn: "2026-09-01",
    status: "active",
  },
  { id: "00000000-0000-0000-0000-000000000102", items: [], name: "Дом 26", plannedFinishOn: null, plannedStartOn: null, status: "draft" },
] as const;

function response(body: unknown, ok = true) {
  return Promise.resolve({ json: () => Promise.resolve(body), ok, status: ok ? 200 : 500 } as Response);
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-09-10T09:00:00Z"));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("GlobalProjectsCalendarPage", () => {
  it("shows accessible project lanes, filters event kinds, and opens linked list items", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => { String(input); return response({ hasMoreProjects: false, projects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false }); });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalProjectsCalendarPage />);

    expect(await screen.findByRole("heading", { name: "Календарь проектов" })).toBeTruthy();
    expect(await screen.findByRole("link", { name: "Полянка" })).toBeTruthy();
    expect(screen.getByRole("link", { name: /Задача Подготовить планы/ }).getAttribute("href")).toContain("/tasks?taskId=");
    expect(screen.getByRole("link", { name: /Встреча Встреча на объекте/ }).getAttribute("href")).toContain("/calendar?meetingId=");

    fireEvent.click(screen.getByRole("button", { name: "Задачи" }));
    expect(screen.queryByRole("link", { name: /Встреча Встреча на объекте/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Встречи" }));
    expect(screen.queryByRole("link", { name: /Задача Подготовить планы/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Задачи" }));
    fireEvent.click(screen.getByRole("button", { name: "Показать календарь списком" }));
    expect(document.querySelector('[data-cy="global-calendar-list"]')).toBeTruthy();
    expect(screen.getByText("Подготовить планы")).toBeTruthy();
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/api/v1/calendar?");
  });

  it("applies a project filter and exposes the active-filter badge", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      const filtered = url.includes("projectId=");
      return response({ hasMoreProjects: false, projects: filtered ? [projects[0]] : projects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalProjectsCalendarPage />);
    await screen.findByRole("link", { name: "Полянка" });
    fireEvent.click(screen.getByRole("button", { name: "Фильтр проектов" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Полянка/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Полянка/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Полянка/ }));
    fireEvent.click(screen.getByRole("button", { name: "Применить" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => String(input).includes("projectId=00000000-0000-0000-0000-000000000101"))).toBe(true));
    expect(screen.getByLabelText("Применено фильтров проектов: 1")).toBeTruthy();
  });

  it("navigates periods and explains truncated data", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => { String(input); return response({ hasMoreProjects: true, projects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: true }); });
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalProjectsCalendarPage />);
    expect(await screen.findByText(/Показана часть календаря/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Предыдущие четыре месяца" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    fireEvent.click(screen.getByRole("button", { name: "Следующие четыре месяца" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    fireEvent.click(screen.getByRole("button", { name: "Сегодня" }));
    fireEvent.click(screen.getByRole("button", { name: "Показать календарь списком" }));
    fireEvent.click(screen.getByRole("button", { name: "Показать временную шкалу" }));
    expect(document.querySelector('[data-cy="global-calendar-timeline"]')).toBeTruthy();
  });

  it("can cancel and reset project selection, including an empty filtered result", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => response({ hasMoreProjects: false, projects: String(input).includes("projectId=") ? [] : projects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false }));
    vi.stubGlobal("fetch", fetchMock);
    render(<GlobalProjectsCalendarPage />);
    await screen.findByRole("link", { name: "Полянка" });
    fireEvent.click(screen.getByRole("button", { name: "Фильтр проектов" }));
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog", { name: "Фильтр проектов" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Фильтр проектов" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Полянка/ }));
    fireEvent.click(screen.getByRole("button", { name: "Применить" }));
    expect(await screen.findByText("Нет проектов по фильтру")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Сбросить фильтр" }));
    expect(await screen.findByRole("link", { name: "Полянка" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Фильтр проектов" }));
    fireEvent.click(screen.getByRole("button", { name: "Сбросить" }));
    expect(screen.queryByRole("dialog", { name: "Фильтр проектов" })).toBeNull();
  });

  it("shows the no-project state and recovery action", async () => {
    vi.stubGlobal("fetch", vi.fn(() => response({ hasMoreProjects: false, projects: [], rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false })));
    render(<GlobalProjectsCalendarPage />);
    expect(await screen.findByText("Проектов пока нет")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Создать проект" }).getAttribute("href")).toBe("/account/projects/new");
  });

  it("shows a retryable API error", async () => {
    vi.stubGlobal("fetch", vi.fn(() => response({ message: "Сервис недоступен" }, false)));
    render(<GlobalProjectsCalendarPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис недоступен");
    expect(screen.getByRole("button", { name: "Повторить" })).toBeTruthy();
  });

  it("uses the agenda first on a narrow screen and handles partial or out-of-range project dates", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    const edgeProjects = [
      { ...projects[1], id: "00000000-0000-0000-0000-000000000103", name: "Только старт", plannedStartOn: "2026-10-01" },
      { ...projects[1], id: "00000000-0000-0000-0000-000000000104", name: "Только сдача", plannedFinishOn: "2026-11-01" },
      { ...projects[1], id: "00000000-0000-0000-0000-000000000105", items: [{ ...projects[0].items[0], effectiveAt: "2025-01-01T10:00:00Z", id: "00000000-0000-0000-0000-000000000205", projectId: "00000000-0000-0000-0000-000000000105" }], name: "Вне периода", plannedFinishOn: "2025-02-01", plannedStartOn: "2025-01-01" },
    ];
    vi.stubGlobal("fetch", vi.fn(() => response({ hasMoreProjects: false, projects: edgeProjects, rangeEnd: "2027-01-01T00:00:00Z", rangeStart: "2026-09-01T00:00:00Z", truncatedEvents: false })));
    render(<GlobalProjectsCalendarPage />);
    expect(await screen.findByRole("button", { name: "Показать временную шкалу" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Показать временную шкалу" }));
    expect(screen.getByRole("link", { name: /Проект Только старт:/ })).toBeTruthy();
    expect(screen.getByRole("link", { name: /Проект Только сдача:/ })).toBeTruthy();
    expect(screen.queryByRole("link", { name: /Задача Подготовить планы, проект Вне периода/ })).toBeNull();
  });
});
