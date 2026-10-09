// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectFinance, parseMoney } from "./projectFinance";

const initialSummary = { availableBalanceMinor: 75_000, canCreateExpense: true, confirmedExpenseMinor: 125_000, confirmedIncomeMinor: 200_000, currencyCode: "RUB", pendingExpenseMinor: 0 };
const expense = { amountMinor: 125_050, category: "materials", createdAt: "2026-09-25T10:00:00Z", createdByUserId: "user-1", currencyCode: "RUB", description: "Светильники", id: "expense-1", plannedPaymentOn: null, projectId: "project-1", status: "auto_approved", vendorName: "Свет", version: 1 };

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.clearAllMocks(); });
function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status }); }

describe("ProjectFinance", () => {
  it("shows totals and retries the same idempotent expense request without losing the form", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    let posts = 0;
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      if (init?.method === "POST") {
        posts += 1;
        return Promise.resolve(posts === 1 ? response({ error: { message: "Сервис временно недоступен" } }, 503) : response(expense, 201));
      }
      return Promise.resolve(response(posts > 1 ? { ...initialSummary, pendingExpenseMinor: 125_050 } : initialSummary));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinance autoApproveExpenses projectId="project-1" />);
    expect(await screen.findByText(/2[\s\u00a0]?000,00/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Перейти в расходы" }).getAttribute("href")).toBe("/account/projects/project-1/finances");
    fireEvent.click(screen.getByRole("button", { name: "Добавить расход" }));
    const dialog = screen.getByRole("dialog", { name: "Добавление расхода" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect(screen.getByRole("alert").textContent).toContain("Укажите сумму");
    fireEvent.change(screen.getByLabelText("Сумма, ₽ *"), { target: { value: "1250,50" } });
    fireEvent.change(screen.getByLabelText("Описание *"), { target: { value: "Светильники" } });
    fireEvent.change(screen.getByLabelText("Поставщик или подрядчик"), { target: { value: "Свет" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис временно недоступен");
    expect((screen.getByLabelText("Сумма, ₽ *") as HTMLInputElement).value).toBe("1250,50");
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect((await screen.findByRole("status")).textContent).toContain("автосогласован");
    const postCalls = fetchMock.mock.calls.filter(([, options]) => options?.method === "POST");
    expect(postCalls).toHaveLength(2);
    const firstHeaders = postCalls[0][1]?.headers as Record<string, string>;
    const secondHeaders = postCalls[1][1]?.headers as Record<string, string>;
    expect(firstHeaders["X-CSRF-Token"]).toBe("csrf-token");
    expect(firstHeaders["Idempotency-Key"]).toBe(secondHeaders["Idempotency-Key"]);
    expect(JSON.parse(String(postCalls[0][1]?.body))).toMatchObject({ amountMinor: 125050, category: "materials", description: "Светильники", vendorName: "Свет" });
    await waitFor(() => expect(screen.getByText(/1[\s\u00a0]?250,50/)).toBeTruthy());
  });

  it("shows recoverable summary errors and hides create for read-only users", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(response({ error: { message: "Финансы временно недоступны" } }, 503)).mockResolvedValue(response({ ...initialSummary, canCreateExpense: false }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinance autoApproveExpenses={false} projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Финансы временно недоступны");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(screen.queryByRole("button", { name: "Добавить расход" })).toBeNull();
    expect(screen.getByRole("link", { name: "Перейти в расходы" })).toBeTruthy();
  });

  it("parses decimal money without floating point rounding", () => {
    expect(parseMoney("1 250,5")).toBe(125_050);
    expect(parseMoney("0")).toBeNull();
    expect(parseMoney("12,345")).toBeNull();
    expect(parseMoney("not-money")).toBeNull();
  });

  it("validates an expense description and creates a pending request without optional fields", async () => {
    const pendingExpense = { ...expense, status: "pending_approval", vendorName: null };
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => init?.method === "POST" ? Promise.resolve(response(pendingExpense, 201)) : Promise.resolve(response(initialSummary)));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinance autoApproveExpenses={false} projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Добавить расход" }));
    const dialog = screen.getByRole("dialog", { name: "Добавление расхода" });
    expect(within(dialog).getByText(/ожидать согласования/)).toBeTruthy();
    fireEvent.change(within(dialog).getByLabelText("Сумма, ₽ *"), { target: { value: "500" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("описание");
    fireEvent.change(within(dialog).getByLabelText("Описание *"), { target: { value: "Доставка" } });
    fireEvent.click(within(dialog).getByRole("combobox", { name: "Статья расхода" }));
    fireEvent.click(screen.getByRole("option", { name: "Доставка" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect((await screen.findByRole("status")).textContent).toContain("ожидает согласования");
    const payload = JSON.parse(String(fetchMock.mock.calls.find(([, init]) => init?.method === "POST")?.[1]?.body));
    expect(payload).toMatchObject({ category: "delivery", plannedPaymentOn: null, vendorName: null });
  });

  it("uses the generic summary message when a retry rejects with an unknown value", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce("offline");
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinance autoApproveExpenses projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить финансовую сводку");
  });

  it("does not present an aborted summary retry as an error", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ error: { message: "Временная ошибка" } }, 503))
      .mockRejectedValueOnce(new DOMException("cancelled", "AbortError"));
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectFinance autoApproveExpenses projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("status", { name: "Загрузка финансовой сводки" })).toBeTruthy();
  });

  it("uses the generic expense error and lets the user close the form", async () => {
    vi.stubGlobal("fetch", vi.fn((_input: string | URL | Request, init?: RequestInit) => init?.method === "POST" ? Promise.reject("offline") : Promise.resolve(response(initialSummary))));
    render(<ProjectFinance autoApproveExpenses projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Добавить расход" }));
    const dialog = screen.getByRole("dialog", { name: "Добавление расхода" });
    fireEvent.change(within(dialog).getByLabelText("Сумма, ₽ *"), { target: { value: "500" } });
    fireEvent.change(within(dialog).getByLabelText("Описание *"), { target: { value: "Доставка" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить расход" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось добавить расход");
    fireEvent.click(within(dialog).getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog", { name: "Добавление расхода" })).toBeNull();
  });
});
