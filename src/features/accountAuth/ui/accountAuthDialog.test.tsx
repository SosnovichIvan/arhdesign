// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AccountAuthProvider, useAccountAuth } from "./accountAuthDialog";

const routerReplace = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: routerReplace }) }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  routerReplace.mockReset();
  window.history.replaceState({}, "", "/");
});

function Trigger() {
  const { openAuth } = useAccountAuth();
  return <button onClick={() => openAuth()} type="button">Открыть вход</button>;
}

describe("AccountAuthProvider", () => {
  it("requires the auth provider", () => {
    expect(() => render(<Trigger />)).toThrow("useAccountAuth must be used inside AccountAuthProvider");
  });

  it("opens login, switches to registration and returns to login", () => {
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    expect(screen.getByRole("dialog", { name: "Авторизация" })).toBeTruthy();
    expect(screen.getByLabelText("Логин или почта *")).toBe(document.activeElement);
    expect(window.location.search).toBe("?auth=login");

    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    expect(screen.getByRole("dialog", { name: "Регистрация" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Создать профиль" })).toBeTruthy();
    expect(screen.getByLabelText("Логин *")).toBe(document.activeElement);
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(screen.getByRole("dialog", { name: "Авторизация" })).toBeTruthy();
    expect(screen.getByLabelText("Логин или почта *")).toBe(document.activeElement);
  });

  it("submits credentials and immediately redirects to the account", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ account: { firstName: "Светлана" }, expiresAt: "2026-09-25T00:00:00Z" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "sveta.design" } });
    fireEvent.change(screen.getByLabelText("Пароль *"), { target: { value: "Надёжный пароль 2026!" } });
    fireEvent.click(screen.getByLabelText("Запомнить меня"));
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));

    await waitFor(() => expect(routerReplace).toHaveBeenCalledWith("/account"));
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ identifier: "sveta.design", password: "Надёжный пароль 2026!", rememberMe: true });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByText("Перейти в кабинет")).toBeNull();
  });

  it("keeps login input and shows API or transport errors", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Неверный логин или пароль" } }), { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "Профиль не подтверждён" }), { status: 403 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({}), { status: 500 }))
      .mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "sveta.design" } });
    fireEvent.change(screen.getByLabelText("Пароль *"), { target: { value: "wrong-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Неверный логин или пароль");
    expect(screen.getByLabelText("Логин или почта *")).toHaveProperty("value", "sveta.design");

    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Профиль не подтверждён"));
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Не удалось выполнить вход"));
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("Сервис авторизации временно недоступен"));
  });

  it("offers verification channels for a valid but unverified account", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "account_unverified", message: "Учётная запись не подтверждена" } }), { status: 403 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "accepted", channel: "email" }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "sveta.design" } });
    fireEvent.change(screen.getByLabelText("Пароль *"), { target: { value: "Надёжный пароль 2026!" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));

    expect(await screen.findByText("Профиль ещё не подтверждён")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Получить письмо" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Открыть Telegram" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Получить письмо" }));
    expect(await screen.findByRole("heading", { name: "Проверьте почту" })).toBeTruthy();
    expect(JSON.parse(String(fetchMock.mock.calls[1][1]?.body))).toEqual({ identifier: "sveta.design", channel: "email" });
  });

  it("opens a requested registration modal from the URL and closes it with Escape", async () => {
    window.history.replaceState({}, "", "/?auth=register");
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("dialog", { name: "Регистрация" })).toBeTruthy();
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(window.location.search).toBe(""));
  });

  it("ignores an unknown auth mode in the URL", async () => {
    window.history.replaceState({}, "", "/?auth=unknown");
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    await new Promise((resolve) => window.setTimeout(resolve, 0));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("requests password recovery without disclosing whether the account exists", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "recovery_accepted" }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    expect(screen.getByRole("dialog", { name: "Восстановление доступа" })).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "sveta.design" } });
    fireEvent.click(screen.getByRole("button", { name: "Получить по email" }));
    expect(await screen.findByRole("heading", { name: "Проверьте выбранный канал" })).toBeTruthy();
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ identifier: "sveta.design", channel: "email" });
    expect(screen.getByText(/Если профиль существует/)).toBeTruthy();
  });

  it("requests Telegram recovery without opening credential input on the website", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "recovery_accepted" }), { status: 202 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "sveta.design" } });
    fireEvent.click(screen.getByRole("button", { name: "Получить в Telegram" }));
    expect(await screen.findByRole("heading", { name: "Проверьте выбранный канал" })).toBeTruthy();
    expect(screen.getByText(/Telegram был подключён заранее/)).toBeTruthy();
    expect(screen.queryByLabelText("Пароль Telegram")).toBeNull();
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ identifier: "sveta.design", channel: "telegram" });
  });

  it("verifies an email token and returns to login", async () => {
    const token = "v".repeat(43);
    window.history.replaceState({}, "", `/?auth=verify#token=${token}`);
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "email_verified" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("heading", { name: "Почта подтверждена" })).toBeTruthy();
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ token });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(screen.getByRole("dialog", { name: "Авторизация" })).toBeTruthy();
    expect(window.location.hash).toBe("");
  });

  it("shows a safe state for an expired verification link", async () => {
    const token = "x".repeat(43);
    window.history.replaceState({}, "", `/?auth=verify#token=${token}`);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "invalid_or_expired_token", message: "Ссылка недействительна или устарела" } }), { status: 400 })));
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("heading", { name: "Ссылка устарела" })).toBeTruthy();
    expect(screen.getByText(/недействительна, уже использована или истекла/)).toBeTruthy();
  });

  it("uses a fragment reset token, changes the password and returns to login", async () => {
    const token = "a".repeat(43);
    window.history.replaceState({}, "", `/?auth=reset#token=${token}`);
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("heading", { name: "Новый пароль" })).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Новый пароль *"), { target: { value: "Новый надёжный пароль 2026!" } });
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "Новый надёжный пароль 2026!" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect(await screen.findByRole("heading", { name: "Пароль изменён" })).toBeTruthy();
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ token, password: "Новый надёжный пароль 2026!", passwordConfirmation: "Новый надёжный пароль 2026!" });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(screen.getByRole("dialog", { name: "Авторизация" })).toBeTruthy();
    expect(window.location.hash).toBe("");
  });

  it("validates recovery identifiers and reports API and transport failures", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Слишком много запросов" } }), { status: 429 }))
      .mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><Trigger /></AccountAuthProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Открыть вход" }));
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    fireEvent.click(screen.getByRole("button", { name: "Получить в Telegram" }));
    expect(screen.getByRole("alert").textContent).toContain("Укажите логин");
    fireEvent.change(screen.getByLabelText("Логин или почта *"), { target: { value: "user" } });
    fireEvent.click(screen.getByRole("button", { name: "Получить по email" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Слишком много запросов");
    fireEvent.click(screen.getByRole("button", { name: "Получить в Telegram" }));
    expect((await screen.findByRole("alert")).textContent).toContain("временно недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Вернуться ко входу" }));
    expect(screen.getByRole("dialog", { name: "Авторизация" })).toBeTruthy();
  });

  it("handles invalid, rejected and unavailable verification links", async () => {
    window.history.replaceState({}, "", "/?auth=verify#token=short");
    const view = render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("heading", { name: "Ссылка устарела" })).toBeTruthy();
    view.unmount();
    window.history.replaceState({}, "", `/?auth=verify#token=${"z".repeat(43)}`);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "Проверка отклонена" } }), { status: 503 })));
    const rejected = render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect(await screen.findByRole("heading", { name: "Не удалось проверить ссылку" })).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("Проверка отклонена");
    rejected.unmount();
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    expect((await screen.findByRole("alert")).textContent).toContain("временно недоступен");
  });

  it("validates reset passwords and reports API and transport failures", async () => {
    const token = "r".repeat(43);
    window.history.replaceState({}, "", `/?auth=reset#token=${token}`);
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Ссылка уже использована" } }), { status: 400 }))
      .mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    render(<AccountAuthProvider><span>Контент</span></AccountAuthProvider>);
    await screen.findByRole("heading", { name: "Новый пароль" });
    fireEvent.change(screen.getByLabelText("Новый пароль *"), { target: { value: "короткий" } });
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "другой" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect(screen.getByRole("alert").textContent).toContain("не совпадают");
    fireEvent.change(screen.getByLabelText("Новый пароль *"), { target: { value: "Надёжный пароль 2026!" } });
    fireEvent.change(screen.getByLabelText("Подтверждение пароля *"), { target: { value: "Надёжный пароль 2026!" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Ссылка уже использована");
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect((await screen.findByRole("alert")).textContent).toContain("временно недоступен");
  });
});
