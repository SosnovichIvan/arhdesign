// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CreateProjectForm } from "./createProjectForm";

const push = vi.fn();
const refresh = vi.fn();
const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push, refresh, replace }) }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("CreateProjectForm", () => {
	it("requires a meaningful project name", () => {
		const fetchMock = vi.fn();
		vi.stubGlobal("fetch", fetchMock);
		render(<CreateProjectForm />);
		fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: "x" } });
		fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
		expect(screen.getByRole("alert").textContent).toContain("минимум 2 символа");
		expect(fetchMock).not.toHaveBeenCalled();
	});
  it("validates dates without sending a request", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    render(<CreateProjectForm />);
    fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: "Новый объект" } });
    const today = new Date();
    const firstDay = new Date(today.getFullYear(), today.getMonth(), 1);
    const secondDay = new Date(today.getFullYear(), today.getMonth(), 2);
    fireEvent.click(screen.getByRole("combobox", { name: "Планируемое завершение" }));
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(firstDay) }));
    fireEvent.click(screen.getByRole("combobox", { name: "Планируемое начало" }));
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(secondDay) }));
    expect(screen.getByText("Дата начала не может быть позже даты завершения")).toBeTruthy();
    expect(screen.getByRole("combobox", { name: "Планируемое завершение" }).getAttribute("aria-invalid")).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
    expect(screen.getByText("Дата начала не может быть позже даты завершения")).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("creates a project with CSRF and returns to the list", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf-token" });
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "project-1" }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<CreateProjectForm />);
    fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: " Квартира на Полянке " } });
    fireEvent.change(screen.getByLabelText("Адрес"), { target: { value: "Москва" } });
    fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/account"));
    const options = fetchMock.mock.calls[0][1] as RequestInit;
    expect((options.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-token");
    expect(JSON.parse(String(options.body))).toMatchObject({ name: "Квартира на Полянке", address: "Москва", customerUserId: null, currencyCode: "RUB" });
    expect(refresh).toHaveBeenCalled();
  });

  it("renders a server error and redirects an expired session", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Проверьте поля проекта" } }), { status: 400 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Требуется вход" } }), { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<CreateProjectForm />);
    fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: "Проект" } });
    fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
	expect((await screen.findByRole("alert")).textContent).toContain("Проверьте поля проекта");
    fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
  });

	it("reads the secure CSRF cookie and submits disabled auto-approval", async () => {
		Object.defineProperty(document, "cookie", { configurable: true, value: "__Host-arhdesign_csrf=secure-csrf" });
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "project-2" }), { status: 201 }));
		vi.stubGlobal("fetch", fetchMock);
		render(<CreateProjectForm />);
		fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: "Дом 25" } });
		fireEvent.click(screen.getByLabelText(/Автосогласование расходов/));
		fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
		await waitFor(() => expect(push).toHaveBeenCalledWith("/account"));
		const options = fetchMock.mock.calls[0][1] as RequestInit;
		expect((options.headers as Record<string, string>)["X-CSRF-Token"]).toBe("secure-csrf");
		expect(JSON.parse(String(options.body)).autoApproveExpenses).toBe(false);
	});

	it("lets the API reject a request when the CSRF cookie is absent", async () => {
		Object.defineProperty(document, "cookie", { configurable: true, value: "" });
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "Запрос отклонён" } }), { status: 403 }));
		vi.stubGlobal("fetch", fetchMock);
		render(<CreateProjectForm />);
		fireEvent.change(screen.getByLabelText("Название проекта *"), { target: { value: "Дом 26" } });
		fireEvent.click(screen.getByRole("button", { name: "Создать проект" }));
		expect((await screen.findByRole("alert")).textContent).toContain("Запрос отклонён");
		expect(((fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>)["X-CSRF-Token"]).toBe("");
	});
});

function formatLongDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}
