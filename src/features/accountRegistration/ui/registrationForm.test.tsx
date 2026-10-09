// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RegistrationForm } from "./registrationForm";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function fillRegistration() {
  fireEvent.change(screen.getByLabelText("Логин *"), { target: { value: "sveta.design" } });
  fireEvent.change(screen.getByLabelText("Электронная почта *"), { target: { value: "sveta@example.com" } });
  fireEvent.change(screen.getByLabelText("Имя *"), { target: { value: "Светлана" } });
  fireEvent.change(screen.getByLabelText("Пароль *"), { target: { value: "Надёжный пароль 2026!" } });
  fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "Надёжный пароль 2026!" } });
}

describe("RegistrationForm", () => {
  it("switches back to the login form when embedded in the auth dialog", () => {
    const onSwitchToLogin = vi.fn();
    render(<RegistrationForm onSwitchToLogin={onSwitchToLogin} />);
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(onSwitchToLogin).toHaveBeenCalledOnce();
  });

  it("registers and sends email only after the user selects that channel", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "verification_pending", channelSelectionRequired: true }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "accepted", channel: "email" }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    expect(await screen.findByRole("heading", { name: "Как подтвердить профиль?" })).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain("Профиль ещё не подтверждён");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Получить письмо" }));
    expect(await screen.findByRole("heading", { name: "Проверьте почту" })).toBeTruthy();
    expect(screen.getByText(/s••••@example.com/)).toBeTruthy();
    expect(screen.getByRole("button", { name: /Отправить ещё раз \(60 сек\.\)/ })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: "Получить в Telegram" })).toBeTruthy();
    expect(JSON.parse(String(fetchMock.mock.calls[1][1]?.body))).toEqual({ identifier: "sveta.design", channel: "email" });
  });

  it("never renders Telegram credential fields and handles a missing handoff URL", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "verification_pending", channelSelectionRequired: true }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "accepted", channel: "telegram", telegramBotUrl: null }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.submit(screen.getByRole("button", { name: "Зарегистрироваться" }).closest("form")!);
    await screen.findByRole("button", { name: "Открыть Telegram" });
    expect(screen.queryByLabelText("Логин Telegram")).toBeNull();
    expect(screen.queryByLabelText("Пароль Telegram")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Открыть Telegram" }));
    await waitFor(() => expect(screen.getByRole("heading", { name: "Не удалось открыть Telegram" })).toBeTruthy());
  });

  it("keeps the form and shows validation/server errors", async () => {
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "другой пароль" } });
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    expect(screen.getByRole("alert").textContent).toContain("Исправьте ошибки в выделенных полях");
    expect(screen.getByText("Пароли не совпадают")).toBeTruthy();
    expect(screen.getByLabelText("Подтверждение пароля *").getAttribute("aria-invalid")).toBe("true");

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Исправьте электронную почту", fields: { email: "Этот адрес нельзя использовать" } } }), { status: 422 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "identifier_unavailable", message: "Логин или почта недоступны" } }), { status: 409 }));
    vi.stubGlobal("fetch", fetchMock);
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "Надёжный пароль 2026!" } });
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Исправьте электронную почту"));
    expect(screen.getByText("Этот адрес нельзя использовать")).toBeTruthy();

    fireEvent.change(screen.getByLabelText("Электронная почта *"), { target: { value: "sveta.new@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Логин или почта недоступны"));
    expect(screen.getByText("Логин или почта уже используются")).toBeTruthy();
    expect(screen.getByText("Почта или логин уже используются")).toBeTruthy();
  });

  it("shows actionable field errors and does not send an invalid form", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fireEvent.change(screen.getByLabelText("Логин *"), { target: { value: "иван" } });
    fireEvent.change(screen.getByLabelText("Электронная почта *"), { target: { value: "not-an-email" } });
    fireEvent.change(screen.getByLabelText("Имя *"), { target: { value: " " } });
    fireEvent.change(screen.getByLabelText("Пароль *"), { target: { value: "короткий" } });
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "короткий" } });
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));

    expect(screen.getByText(/3–64 латинских символа/)).toBeTruthy();
    expect(screen.getByText("Введите корректный адрес электронной почты")).toBeTruthy();
    expect(screen.getByText("Укажите имя длиной до 100 символов")).toBeTruthy();
    expect(screen.getByText("Пароль должен содержать от 15 до 128 символов")).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("handles transport and malformed server errors without losing the draft", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(new Response("not-json", { status: 503 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.change(screen.getByLabelText("Фамилия"), { target: { value: "Полисмакова" } });
    fireEvent.change(screen.getByLabelText("Отчество"), { target: { value: "Александровна" } });
    fireEvent.click(screen.getByRole("combobox", { name: "Роль" }));
    fireEvent.click(screen.getByRole("option", { name: "Архитектор" }));

    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Сервис регистрации временно недоступен"));
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Не удалось выполнить запрос"));
    expect(screen.getByLabelText("Фамилия")).toHaveProperty("value", "Полисмакова");
  });

  it("supports keyboard role selection", () => {
    render(<RegistrationForm />);
    const roleSelect = screen.getByRole("combobox", { name: "Роль" });
    fireEvent.keyDown(roleSelect, { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Дизайнер" }).getAttribute("class")).toContain("bg-page");
    fireEvent.keyDown(roleSelect, { key: "Enter" });
    expect(roleSelect.textContent).toContain("Дизайнер");
    expect(roleSelect.getAttribute("aria-expanded")).toBe("false");
  });

  it("shows a channel error, retries and executes the Telegram handoff", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "verification_pending" }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "Попробуйте позже" }), { status: 503 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ telegramBotUrl: null }), { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ telegramBotUrl: "https://t.me/polismakovaSvetlanaBot?start=register" }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await screen.findByRole("button", { name: "Открыть Telegram" });

    fireEvent.click(screen.getByRole("button", { name: "Получить письмо" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Попробуйте позже"));
    fireEvent.click(screen.getByRole("button", { name: "Открыть Telegram" }));
    await screen.findByRole("heading", { name: "Не удалось открыть Telegram" });
    fireEvent.click(screen.getByRole("button", { name: "Открыть приложение Telegram" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(4));
    expect(screen.getByRole("heading", { name: "Открываем Telegram" })).toBeTruthy();
    expect(screen.queryByLabelText("Логин Telegram")).toBeNull();
  });

  it("returns to channel selection when email delivery has a transport error", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "verification_pending" }), { status: 202 }))
      .mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    render(<RegistrationForm />);
    fillRegistration();
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    await screen.findByRole("button", { name: "Получить письмо" });
    fireEvent.click(screen.getByRole("button", { name: "Получить письмо" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Сервис подтверждения временно недоступен"));
    expect(screen.getByRole("heading", { name: "Как подтвердить профиль?" })).toBeTruthy();
  });
});
