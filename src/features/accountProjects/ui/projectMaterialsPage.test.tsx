// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectMaterialsPage } from "./projectMaterialsPage";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });
function response(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, status })); }
const project = { autoApproveExpenses: true, currencyCode: "RUB", id: "project-1", name: "Полянка" };
const material = { actualDeliveryOn: null, canDelete: true, canEdit: true, category: "furniture", contactInfo: "+7 999 000-00-00", contractAmountMinor: 250000, contractReference: "Счёт 17", createdAt: "2026-09-26T10:00:00Z", createdByUserId: "user-1", currencyCode: "RUB", deliveryStatus: "expected", id: "material-1", installationOn: null, linkedExpenseDescription: "Диван", linkedExpenseId: "expense-1", name: "Диван", notes: null, paidAmountMinor: 100000, paymentStatus: "partial", plannedDeliveryOn: "2026-10-10", projectId: "project-1", remainingAmountMinor: 150000, supplierName: "Фабрика", updatedAt: "2026-09-26T10:00:00Z", version: 1 };

describe("ProjectMaterialsPage", () => {
  it("shows totals, filters and linked expense", async () => {
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/materials")) return response({ canCreate: true, canViewFinancialInformation: true, items: [material] }); if (url.endsWith("/expenses")) return response({ canCreateExpense: true, items: [] }); return response(project); }));
    render(<ProjectMaterialsPage projectId="project-1" />);
    expect(await screen.findByRole("heading", { name: "Материалы" })).toBeTruthy();
    expect(screen.queryByText("Реестр проекта")).toBeNull();
    expect(screen.getByRole("button", { name: "Добавить материал" })).toBeTruthy();
    expect(screen.getByText("Фабрика")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Диван" }).getAttribute("href")).toContain("/finances");
    expect(screen.getByRole("table")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Показать материалы списком" }));
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.getByRole("list", { name: "Материалы списком" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Показать материалы таблицей" }).getAttribute("aria-pressed")).toBe("true");
    fireEvent.change(screen.getByPlaceholderText("Название, поставщик, контакт или договор"), { target: { value: "нет" } });
    expect(screen.getByText("Ничего не найдено")).toBeTruthy();
  });

  it("validates and creates a material", async () => {
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => { const url = String(input); if (init?.method === "POST") return response(material, 201); if (url.endsWith("/materials")) return response({ canCreate: true, canViewFinancialInformation: true, items: [] }); if (url.endsWith("/expenses")) return response({ canCreateExpense: true, items: [] }); return response(project); });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMaterialsPage projectId="project-1" />);
    fireEvent.click((await screen.findAllByRole("button", { name: "Добавить материал" }))[0]);
    const dialog = screen.getByRole("dialog", { name: "Добавление материала" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить материал" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("название");
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Диван" } });
    fireEvent.change(within(dialog).getByLabelText("Поставщик *"), { target: { value: "Фабрика" } });
    fireEvent.change(within(dialog).getByLabelText("Сумма договора, ₽ *"), { target: { value: "2500" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить материал" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, options]) => options?.method === "POST" && String(options.body).includes('"supplierName":"Фабрика"'))).toBe(true));
  });

  it("honours restricted permissions and renders missing and actual delivery details", async () => {
    const restricted = { ...material, actualDeliveryOn: "2026-10-09", canDelete: false, canEdit: false, category: "other", contactInfo: null, contractReference: null, deliveryStatus: "delivered", linkedExpenseDescription: null, linkedExpenseId: null, plannedDeliveryOn: null };
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/materials")) return response({ canCreate: false, canViewFinancialInformation: false, items: [restricted] }); if (url.endsWith("/expenses")) return response({ canCreateExpense: false, items: [] }); return response(project); }));
    render(<ProjectMaterialsPage projectId="project-1" />);
    expect(await screen.findByText("Финансовые суммы скрыты вашими правами проекта.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Добавить материал" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Редактировать/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Удалить/ })).toBeNull();
    expect(screen.getByText("Контакт не указан")).toBeTruthy();
    expect(screen.getByText("Договор не указан")).toBeTruthy();
    expect(screen.getByText(/Факт:/)).toBeTruthy();
    expect(screen.getByText("Не связан")).toBeTruthy();
  });

  it("edits and deletes a material with validation and refresh notices", async () => {
    let items = [material];
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "PATCH") return response({ ...material, name: "Диван обновлён", version: 2 });
      if (init?.method === "DELETE") { items = []; return Promise.resolve(new Response(null, { status: 204 })); }
      if (url.endsWith("/materials")) return response({ canCreate: true, canViewFinancialInformation: true, items });
      if (url.endsWith("/expenses")) return response({ canCreateExpense: true, items: [{ amountMinor: 250000, category: "furniture", createdAt: "2026-09-25T10:00:00Z", createdByUserId: "user-1", currencyCode: "RUB", description: "Диван", id: "expense-1", plannedPaymentOn: null, projectId: "project-1", status: "paid", vendorName: "Фабрика", version: 1 }] });
      return response(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMaterialsPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Редактировать Диван" }));
    const editDialog = screen.getByRole("dialog", { name: "Редактирование материала" });
    fireEvent.change(within(editDialog).getByLabelText("Название *"), { target: { value: "Диван обновлён" } });
    fireEvent.click(within(editDialog).getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByText("Материал обновлён.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Удалить Диван" }));
    const deleteDialog = screen.getByRole("dialog", { name: "Удаление материала" });
    fireEvent.click(within(deleteDialog).getByRole("button", { name: "Удалить" }));
    expect(within(deleteDialog).getByRole("alert").textContent).toContain("причину");
    fireEvent.change(within(deleteDialog).getByLabelText("Причина удаления *"), { target: { value: "Дубль" } });
    fireEvent.click(within(deleteDialog).getByRole("button", { name: "Удалить" }));
    expect(await screen.findByText(/Материал удалён/)).toBeTruthy();
    expect(await screen.findByText("Материалов пока нет")).toBeTruthy();
  });

  it("reports save and delete failures and lets the user cancel dialogs", async () => {
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "PATCH") return Promise.reject(new Error("Сохранение недоступно"));
      if (init?.method === "DELETE") return Promise.reject(new Error("Удаление недоступно"));
      if (url.endsWith("/materials")) return response({ canCreate: true, canViewFinancialInformation: true, items: [material] });
      if (url.endsWith("/expenses")) return response({ canCreateExpense: true, items: [] });
      return response(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMaterialsPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Редактировать Диван" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Редактирование материала" })).getByRole("button", { name: "Сохранить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Сохранение недоступно");
    fireEvent.click(within(screen.getByRole("dialog", { name: "Редактирование материала" })).getByRole("button", { name: "Отмена" }));
    fireEvent.click(screen.getByRole("button", { name: "Удалить Диван" }));
    const dialog = screen.getByRole("dialog", { name: "Удаление материала" });
    fireEvent.change(within(dialog).getByLabelText("Причина удаления *"), { target: { value: "Ошибка" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Удалить" }));
    expect((await within(dialog).findByRole("alert")).textContent).toContain("Удаление недоступно");
    fireEvent.click(within(dialog).getByRole("button", { name: "Отмена" }));
  });

  it("shows an access error and successfully retries", async () => {
    let failed = true;
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => {
      const url = String(input);
      if (failed && url.endsWith("/materials")) { failed = false; return response({ error: { message: "missing" } }, 404); }
      if (url.endsWith("/materials")) return response({ canCreate: false, canViewFinancialInformation: false, items: [] });
      if (url.endsWith("/expenses")) return response({ canCreateExpense: false, items: [] });
      return response(project);
    }));
    render(<ProjectMaterialsPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("нет доступа");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("Материалов пока нет")).toBeTruthy();
  });

  it("renders every material and delivery category with planned, actual and missing dates", async () => {
    const categories = ["materials", "furniture", "delivery", "installation", "other"] as const;
    const deliveries = ["expected", "partially_delivered", "delivered", "installed", "cancelled"] as const;
    const items = deliveries.map((deliveryStatus, index) => ({
      ...material,
      actualDeliveryOn: index === 2 ? "2026-10-09" : null,
      canDelete: false,
      canEdit: false,
      category: categories[index],
      contactInfo: index ? `Контакт ${index}` : null,
      contractReference: index ? `Договор ${index}` : null,
      deliveryStatus,
      id: `material-${index}`,
      linkedExpenseDescription: index === 1 ? null : material.linkedExpenseDescription,
      linkedExpenseId: index === 4 ? null : material.linkedExpenseId,
      name: `Позиция ${index}`,
      plannedDeliveryOn: index === 3 ? null : "2026-10-10",
    }));
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/materials")) return response({ canCreate: false, canViewFinancialInformation: true, items }); if (url.endsWith("/expenses")) return response({ canCreateExpense: false, items: [] }); return response(project); }));
    render(<ProjectMaterialsPage projectId="project-1" />);
    await screen.findByText("Позиция 0");
    for (const label of ["Материалы", "Мебель", "Доставка", "Монтаж", "Другое", "Ожидается", "Частично доставлено", "Доставлено", "Установлено", "Отменено", "Дата не указана", "Открыть расход", "Не связан"]) expect(screen.getAllByText(label).length).toBeGreaterThan(0);
  });

  it("validates supplier, amount and actual-delivery consistency before creating", async () => {
    const fetchMock = vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/materials")) return response({ canCreate: true, canViewFinancialInformation: true, items: [] }); if (url.endsWith("/expenses")) return response({ canCreateExpense: true, items: [] }); return response(project); });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectMaterialsPage projectId="project-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Добавить первый материал" }));
    const dialog = screen.getByRole("dialog", { name: "Добавление материала" });
    fireEvent.change(within(dialog).getByLabelText("Название *"), { target: { value: "Диван" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить материал" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("поставщика");
    fireEvent.change(within(dialog).getByLabelText("Поставщик *"), { target: { value: "Фабрика" } });
    fireEvent.change(within(dialog).getByLabelText("Сумма договора, ₽ *"), { target: { value: "0" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Добавить материал" }));
    expect(within(dialog).getByRole("alert").textContent).toContain("больше нуля");
  });
});
