// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AccountProjectsPage } from "./accountProjectsPage";

const replace = vi.fn();
const setProjectCount = vi.fn();
const router = { replace };

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/widgets/accountShell", () => ({ useAccountNavigation: () => ({ setProjectCount }) }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status });
}

describe("AccountProjectsPage", () => {
  it("shows an accessible empty state", async () => {
    vi.mocked(fetch).mockResolvedValue(response({ hasMore: false, items: [], nextCursor: null }));
    render(<AccountProjectsPage />);
    expect(await screen.findByText("У вас пока нет проектов")).toBeTruthy();
	expect(screen.getByRole("link", { name: "Создать первый проект" }).getAttribute("href")).toBe("/account/projects/new");
    expect(setProjectCount).toHaveBeenCalledWith(0);
  });

  it("renders cards and filters by query and status", async () => {
    vi.mocked(fetch).mockResolvedValue(response({
      hasMore: false,
      nextCursor: null,
      items: [
        { id: "1", name: "Полянка", type: "Дизайн интерьера", address: "Москва", status: "active", customerUserId: "2", plannedStartOn: "2026-09-01", plannedFinishOn: "2026-12-01", createdAt: "2026-09-01T00:00:00Z", createdByUserId: "2", currencyCode: "RUB", description: null, autoApproveExpenses: true, version: 1 },
        { id: "3", name: "Дом в Наро-Фоминске", type: "Архитектурный проект", address: null, status: "draft", customerUserId: null, plannedStartOn: null, plannedFinishOn: null, createdAt: "2026-09-02T00:00:00Z", createdByUserId: "2", currencyCode: "RUB", description: null, autoApproveExpenses: true, version: 1 },
		{ id: "4", name: "Только старт", type: "Надзор", address: null, status: "paused", customerUserId: null, plannedStartOn: "2026-10-01", plannedFinishOn: null, createdAt: "2026-09-03T00:00:00Z", createdByUserId: "2", currencyCode: "RUB", description: null, autoApproveExpenses: false, version: 1 },
		{ id: "5", name: "Только финиш", type: "Комплектация", address: null, status: "completed", customerUserId: null, plannedStartOn: null, plannedFinishOn: "2026-11-01", createdAt: "2026-09-04T00:00:00Z", createdByUserId: "2", currencyCode: "RUB", description: null, autoApproveExpenses: false, version: 1 },
      ],
    }));
    render(<AccountProjectsPage />);
    expect(await screen.findByRole("heading", { name: "Полянка" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Дом в Наро-Фоминске" })).toBeTruthy();
	expect(screen.getByRole("heading", { name: "Только старт" })).toBeTruthy();
	expect(screen.getByRole("heading", { name: "Только финиш" })).toBeTruthy();
    expect(setProjectCount).toHaveBeenCalledWith(1);

    fireEvent.change(screen.getByLabelText("Поиск проекта"), { target: { value: "наро" } });
    expect(screen.queryByRole("heading", { name: "Полянка" })).toBeNull();
    expect(screen.getByRole("heading", { name: "Дом в Наро-Фоминске" })).toBeTruthy();

    fireEvent.change(screen.getByLabelText("Поиск проекта"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("combobox", { name: "Статус" }));
    fireEvent.click(screen.getByRole("option", { name: /Черновики/ }));
    expect(screen.queryByRole("heading", { name: "Полянка" })).toBeNull();
	fireEvent.change(screen.getByLabelText("Поиск проекта"), { target: { value: "не существует" } });
	expect(screen.getByRole("heading", { name: "Ничего не найдено" })).toBeTruthy();
  });

  it("shows API errors, retries and redirects expired sessions", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(response({ error: { message: "База временно недоступна" } }, 500))
      .mockResolvedValueOnce(response({ hasMore: false, items: [], nextCursor: null }))
      .mockResolvedValueOnce(response({ error: { message: "Требуется вход" } }, 401));
    const { unmount } = render(<AccountProjectsPage />);
	expect((await screen.findByRole("alert")).textContent).toContain("База временно недоступна");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("У вас пока нет проектов")).toBeTruthy();
    unmount();
    render(<AccountProjectsPage />);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
  });

	it("uses a generic message for an unknown transport failure", async () => {
		vi.mocked(fetch).mockRejectedValue("offline");
		render(<AccountProjectsPage />);
		expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить проекты");
	});

  it("redirects when the retry discovers an expired session", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockResolvedValueOnce(response({ error: { message: "Требуется вход" } }, 401));
    render(<AccountProjectsPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
  });

  it("ignores an aborted retry without presenting it as an application error", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce(new DOMException("cancelled", "AbortError"));
    render(<AccountProjectsPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("status", { name: "Загрузка проектов" })).toBeTruthy();
  });

  it("shows the generic retry error for an unknown rejected value", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce("offline");
    render(<AccountProjectsPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить проекты");
  });
});
