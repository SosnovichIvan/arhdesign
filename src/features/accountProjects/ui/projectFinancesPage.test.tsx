// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectFinancesPage } from "./projectFinancesPage";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status }); }

describe("ProjectFinancesPage", () => {
  it("shows project expenses and their textual statuses", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/expenses") ? Promise.resolve(response({ canCreateExpense: true, items: [{ amountMinor: 125050, category: "materials", createdAt: "2026-09-25T10:00:00Z", createdByUserId: "user-1", currencyCode: "RUB", description: "Светильники", id: "expense-1", plannedPaymentOn: null, projectId: "project-1", status: "auto_approved", vendorName: "Свет", version: 1 }] })) : Promise.resolve(response({ id: "project-1", name: "Полянка" }))));
    render(<ProjectFinancesPage projectId="project-1" />);
    expect(await screen.findByRole("heading", { name: "Расходы" })).toBeTruthy();
    expect(screen.getByText("Светильники")).toBeTruthy();
    expect(screen.getByText("Автосогласован")).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Хлебные крошки" }).textContent).toContain("Полянка");
  });

  it("shows an empty state", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/expenses") ? Promise.resolve(response({ canCreateExpense: true, items: [] })) : Promise.resolve(response({ id: "project-1", name: "Полянка" }))));
    render(<ProjectFinancesPage projectId="project-1" />);
    expect(await screen.findByText("Расходов пока нет")).toBeTruthy();
  });

  it("finishes loading when React Strict Mode remounts the page effect", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/expenses") ? Promise.resolve(response({ canCreateExpense: true, items: [] })) : Promise.resolve(response({ id: "project-1", name: "Полянка" }))));
    render(<StrictMode><ProjectFinancesPage projectId="project-1" /></StrictMode>);
    expect(await screen.findByText("Расходов пока нет")).toBeTruthy();
  });

  it("renders every supported category and status, including an absent vendor", async () => {
    const categories = ["materials", "furniture", "contractor", "delivery", "installation", "design", "other"] as const;
    const statuses = ["draft", "pending_approval", "auto_approved", "approved", "rejected", "awaiting_payment", "paid", "cancelled"] as const;
    const items = statuses.map((status, index) => ({
      amountMinor: 10000 + index,
      category: categories[index % categories.length],
      createdAt: "2026-09-25T10:00:00Z",
      createdByUserId: "user-1",
      currencyCode: "RUB",
      description: `Расход ${index + 1}`,
      id: `expense-${index + 1}`,
      plannedPaymentOn: null,
      projectId: "project-1",
      status,
      vendorName: index === 0 ? null : `Поставщик ${index}`,
      version: 1,
    }));
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => String(input).endsWith("/expenses") ? Promise.resolve(response({ canCreateExpense: false, items })) : Promise.resolve(response({ id: "project-1", name: "Полянка" }))));
    render(<ProjectFinancesPage projectId="project-1" />);
    expect(await screen.findByText("Черновик")).toBeTruthy();
    for (const label of ["Ожидает согласования", "Автосогласован", "Согласован", "Отклонён", "Ожидает оплаты", "Оплачен", "Отменён", "Материалы", "Мебель", "Работы подрядчика", "Доставка", "Монтаж", "Проектирование", "Другое", "Не указан"]) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
  });

  it("shows not-found and unknown errors and retries successfully", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "missing" } }, 404))
      .mockResolvedValueOnce(response({ id: "project-1", name: "Полянка" }))
      .mockResolvedValueOnce(response({ id: "project-1", name: "Полянка" }))
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinancesPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Проект не найден");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(screen.getByText("Расходов пока нет")).toBeTruthy());
  });

  it("shows a transport error returned while loading", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("Сеть недоступна"))));
    render(<ProjectFinancesPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Сеть недоступна");
  });

  it("reports a not-found response returned by the retry", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("Сеть недоступна"))
      .mockResolvedValueOnce(response({ id: "project-1", name: "Полянка" }))
      .mockResolvedValueOnce(response({ error: { message: "missing" } }, 404))
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinancesPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Проект не найден");
  });

  it("reports an unknown rejected value returned by the retry", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("Сеть недоступна"))
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }))
      .mockRejectedValueOnce("offline")
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinancesPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить расходы");
  });

  it("keeps an aborted retry out of the error state", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("Сеть недоступна"))
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }))
      .mockRejectedValueOnce(new DOMException("cancelled", "AbortError"))
      .mockResolvedValueOnce(response({ canCreateExpense: true, items: [] }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinancesPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("status", { name: "Загрузка расходов" })).toBeTruthy();
  });
});
