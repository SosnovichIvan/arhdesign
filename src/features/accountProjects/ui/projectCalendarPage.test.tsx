// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectCalendarPage } from "./projectCalendarPage";
import type { ProjectUpcomingItem } from "../model/types";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.clearAllMocks(); });
function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status }); }
function localEvent(dayOffset: number, hour: number) { const value = new Date(); value.setHours(hour, 0, 0, 0); value.setDate(value.getDate() + dayOffset); return value.toISOString(); }
const project = { address: null, autoApproveExpenses: true, createdAt: localEvent(-10, 12), createdByUserId: "user-1", currencyCode: "RUB", customerUserId: "user-1", description: null, id: "project-1", name: "Полянка", plannedFinishOn: null, plannedStartOn: null, status: "active", type: "interior_design", version: 1 };
const events: ProjectUpcomingItem[] = [
  { createdAt: localEvent(-2, 10), description: "Подготовить комплект", effectiveAt: localEvent(0, 12), endsAt: null, id: "task-1", kind: "task", location: null, projectId: "project-1", status: "new", title: "Рабочие чертежи", version: 1 },
  { createdAt: localEvent(-2, 10), description: null, effectiveAt: localEvent(1, 15), endsAt: localEvent(1, 16), id: "meeting-1", kind: "meeting", location: "Объект", projectId: "project-1", status: null, title: "Встреча с заказчиком", version: 1 },
];

function mockLoad(items = events) {
  const fetchMock = vi.fn((input: string | URL | Request) => String(input).includes("/calendar?") ? Promise.resolve(response({ items, rangeEnd: localEvent(40, 0), rangeStart: localEvent(-10, 0) })) : Promise.resolve(response(project)));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("ProjectCalendarPage", () => {
  it("renders tasks and meetings, filters them and opens event details", async () => {
    mockLoad();
    render(<ProjectCalendarPage projectId="project-1" />);
    expect(await screen.findByRole("region", { name: "Календарь проекта" })).toBeTruthy();
    expect(screen.queryByText("План проекта")).toBeNull();
    expect(screen.queryByRole("heading", { name: "Календарь проекта" })).toBeNull();
    expect(await screen.findByRole("button", { name: /Задача: Рабочие чертежи/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Встреча: Встреча с заказчиком/ })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Встречи" }));
    expect(screen.queryByRole("button", { name: /Задача: Рабочие чертежи/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Встреча: Встреча с заказчиком/ }));
    expect(screen.getByRole("dialog", { name: "Событие календаря" }).textContent).toContain("Объект");
  });

  it("switches views and reloads a new period", async () => {
    const fetchMock = mockLoad();
    render(<ProjectCalendarPage projectId="project-1" />);
    await screen.findByRole("button", { name: /Задача: Рабочие чертежи/ });
    fireEvent.click(screen.getByRole("button", { name: "Показать события списком" }));
    expect(document.querySelector("[data-cy='calendar-list']")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Неделя" }));
    fireEvent.click(screen.getByRole("button", { name: /Выбор месяца и года:/ }));
    expect(screen.getByRole("dialog", { name: "Выбор месяца и года" })).toBeTruthy();
    fireEvent.click(screen.getByRole("combobox", { name: "Год" }));
    fireEvent.click(screen.getByRole("option", { name: String(new Date().getFullYear() + 1) }));
    fireEvent.click(screen.getByRole("button", { name: "Показать" }));
    await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => String(input).includes("/calendar?")).length).toBeGreaterThan(1));
  });

  it("shows empty and recoverable error states", async () => {
    mockLoad([]);
    const { unmount } = render(<ProjectCalendarPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Показать события списком" }));
    expect(screen.getByRole("heading", { name: "В этом периоде событий нет" })).toBeTruthy();
    unmount();
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).includes("/calendar?") ? Promise.resolve(response({ error: { message: "Сервис календаря недоступен" } }, 503)) : Promise.resolve(response(project))));
    render(<ProjectCalendarPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис календаря недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  });
});
