// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { ReleaseNotesDrawer } from "./releaseNotesDrawer";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("loads private notes only when opened, retries errors and restores focus on close", async () => {
  const load = vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json([
    { version: "1.0.19", added: ["Изображения в чатах"], fixed: [{ description: "Продление сеанса", source: "Артемий (@Artemy)" }] },
  ]));
  vi.stubGlobal("fetch", load);
  render(<ReleaseNotesDrawer />);
  const trigger = screen.getByRole("button", { name: "Что нового" });
  expect(load).not.toHaveBeenCalled();
  trigger.focus();
  fireEvent.click(trigger);
  expect(screen.getByRole("dialog").className).toContain("max-w-[540px]");
  expect(screen.getByRole("status").textContent).toContain("Загружаем");
  expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить");
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByRole("heading", { name: "Версия 1.0.19" })).toBeTruthy();
  expect(screen.getByText("Источник — Артемий (@Artemy)")).toBeTruthy();
  expect(screen.getByText("Изображения в чатах")).toBeTruthy();
  expect(load).toHaveBeenCalledWith("/api/v1/release-notes", expect.objectContaining({ credentials: "same-origin" }));
  fireEvent.keyDown(screen.getByRole("button", { name: "Закрыть список обновлений" }), { key: "Escape" });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});
