// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PasswordInput } from "./passwordInput";

afterEach(cleanup);

describe("PasswordInput", () => {
  it("shows and hides the password without changing its value", () => {
    const onChange = vi.fn();
    render(<PasswordInput dataCy="account-password" label="Пароль" onChange={onChange} required value="Секрет 2026" />);
    const input = screen.getByLabelText("Пароль *");

    expect(input).toHaveProperty("type", "password");
    expect(input.className).toContain("px-3");
    expect(input.className).toContain("pr-12");
    const show = screen.getByRole("button", { name: "Показать пароль" });
    expect(show.getAttribute("aria-pressed")).toBe("false");
    fireEvent.click(show);
    expect(input).toHaveProperty("type", "text");
    expect(input).toHaveProperty("value", "Секрет 2026");

    const hide = screen.getByRole("button", { name: "Скрыть пароль" });
    expect(hide.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(hide);
    expect(input).toHaveProperty("type", "password");
    expect(screen.getByRole("button", { name: "Показать пароль" }).getAttribute("data-cy")).toBe("account-password-visibility");
  });

  it("disables the input and its visibility action together", () => {
    render(<PasswordInput disabled label="Код доступа" />);
    expect(screen.getByLabelText("Код доступа")).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: "Показать пароль" })).toHaveProperty("disabled", true);
  });
});
