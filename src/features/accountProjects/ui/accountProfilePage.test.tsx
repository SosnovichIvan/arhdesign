// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { AccountProfilePage } from "./accountProfilePage";

vi.mock("@/features/themeToggle", () => ({ ThemeToggle: () => <button type="button">Тема</button> }));

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("recovers from a profile request failure and shows the current account", async () => {
  const request = vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(
    Response.json({ id: "user-1", login: "ivan", firstName: "Иван", lastName: "Петров", email: "ivan@example.test", globalRole: "super_admin" }),
  );
  vi.stubGlobal("fetch", request);
  render(<AccountProfilePage />);
  expect(screen.getByRole("status").textContent).toBe("Загружаем профиль…");
  expect(await screen.findByRole("alert")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByText("Иван Петров")).toBeTruthy();
  expect(screen.getByText("Супер-администратор")).toBeTruthy();
  expect(screen.getByRole("link", { name: "Изменить пароль" }).getAttribute("href")).toBe("/?auth=forgot");
});

it.each([
  [{ globalRole: "technical_admin" }, "Технический администратор"],
  [{ globalRole: null, professionalRole: { name: "Заказчик" } }, "Заказчик"],
  [{ globalRole: null }, "Участник"],
])("displays the proper role and login fallback", async (details, title) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json({ login: "reviewer", ...details })));
  render(<AccountProfilePage />);
  expect(await screen.findByText(title)).toBeTruthy();
  expect(screen.getAllByText("reviewer")).toHaveLength(2);
  expect(screen.queryByText("Электронная почта")).toBeNull();
});
