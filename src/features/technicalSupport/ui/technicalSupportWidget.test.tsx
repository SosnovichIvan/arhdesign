// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const navigation = vi.hoisted(() => ({ pathname: "/account/chats" as string | null }));

vi.mock("next/navigation", () => ({ usePathname: () => navigation.pathname }));
vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

import { FrontendErrorReporter } from "./frontendErrorReporter";
import { TechnicalSupportWidget } from "./technicalSupportWidget";

afterEach(() => { cleanup(); navigation.pathname = "/account/chats"; vi.restoreAllMocks(); vi.unstubAllGlobals(); Object.defineProperty(document, "cookie", { configurable: true, value: "" }); });

describe("TechnicalSupportWidget", () => {
  it("submits a complaint without secrets and explains Telegram delivery", async () => {
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=csrf" });
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<TechnicalSupportWidget />);
    fireEvent.click(screen.getByRole("button", { name: /Сообщить о проблеме/ }));
    expect(screen.getByText(/Не указывайте пароли/)).toBeTruthy();
    fireEvent.change(screen.getByLabelText(/Сообщение/), { target: { value: "Не открывается список чатов" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить" }));
    expect(await screen.findByText("Спасибо за обратную связь")).toBeTruthy();
    const [, init] = fetchMock.mock.calls[0];
    expect(JSON.parse(init.body)).toEqual({ category: "complaint", message: "Не открывается список чатов", path: "/account/chats" });
    expect(init.headers["X-CSRF-Token"]).toBe("csrf");
  });

  it("reports runtime errors once with sanitized same-origin metadata", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<FrontendErrorReporter />);
    const error = new Error("Ошибка для test@example.com bearer secret-token");
    window.dispatchEvent(new ErrorEvent("error", { error, message: error.message }));
    window.dispatchEvent(new ErrorEvent("error", { error, message: error.message }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const payload = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(payload.message).not.toContain("test@example.com");
    expect(payload.message).not.toContain("secret-token");
    expect(payload.path).toBe("/");
    expect(payload.fingerprint.length).toBeGreaterThanOrEqual(16);
  });

  it("validates suggestions, displays delivery errors and can be closed", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "Сервис недоступен" } }), { headers: { "Content-Type": "application/json" }, status: 503 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<TechnicalSupportWidget />);
    fireEvent.click(screen.getByRole("button", { name: /Сообщить о проблеме или предложить/ }));
    fireEvent.click(screen.getByRole("button", { name: "Предложить улучшение" }));
    fireEvent.change(screen.getByLabelText(/Сообщение/), { target: { value: "мало" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить" }));
    expect(screen.getByRole("alert").textContent).toContain("пятью символами");
    fireEvent.change(screen.getByLabelText(/Сообщение/), { target: { value: "Добавьте экспорт" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Сервис недоступен");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body).category).toBe("suggestion");
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("heading", { name: "Жалоба или предложение" })).toBeNull();
  });

  it("reports unhandled rejections and message-only browser errors", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<FrontendErrorReporter />);
    window.dispatchEvent(new ErrorEvent("error", { message: "unique message only" }));
    const rejection = new Event("unhandledrejection") as PromiseRejectionEvent;
    Object.defineProperty(rejection, "reason", { value: "plain rejection" });
    window.dispatchEvent(rejection);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(JSON.parse(fetchMock.mock.calls[1][1].body).message).toBe("Необработанная ошибка приложения");
  });

  it("uses the account fallback path and a generic error for an unknown failure", async () => {
    navigation.pathname = null;
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject("offline")));
    render(<TechnicalSupportWidget />);
    fireEvent.click(screen.getByRole("button", { name: /Сообщить о проблеме или предложить/ }));
    fireEvent.change(screen.getByLabelText(/Сообщение/), { target: { value: "Проверка отправки" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось отправить обращение");
  });
});
