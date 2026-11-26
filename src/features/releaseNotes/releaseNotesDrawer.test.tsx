// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { ReleaseNotesDrawer } from "./releaseNotesDrawer";

afterEach(() => { cleanup(); vi.restoreAllMocks(); localStorage.clear(); vi.unstubAllGlobals(); });

const storageKey = "arhdesign:release-notes:seen:reader";
const note = { version: "1.0.19", added: ["Изображения в чатах"], fixed: [{ description: "Продление сеанса", source: "Артемий (@Artemy)" }] };
const serveNotes = (revision: string) => Response.json([note], { headers: { "X-Release-Notes-Revision": revision } });

it("loads private notes, retries errors and restores focus on close", async () => {
  const load = vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(serveNotes("first"));
  vi.stubGlobal("fetch", load);
  render(<ReleaseNotesDrawer accountId="reader" />);
  const trigger = screen.getByRole("button", { name: "Что нового" });
  expect(load).toHaveBeenCalledTimes(1);
  trigger.focus();
  fireEvent.click(trigger);
  expect(screen.getByRole("dialog").className).toContain("max-w-[540px]");
  expect(screen.getByRole("status").textContent).toContain("Загружаем");
  expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить");
  expect(localStorage.getItem(storageKey)).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByRole("heading", { name: "Версия 1.0.19" })).toBeTruthy();
  expect(screen.getByText("Источник — Артемий (@Artemy)")).toBeTruthy();
  expect(screen.getByText("Изображения в чатах")).toBeTruthy();
  expect(localStorage.getItem(storageKey)).toBe("first");
  expect(load).toHaveBeenCalledWith("/api/v1/release-notes", expect.objectContaining({ credentials: "same-origin" }));
  fireEvent.keyDown(screen.getByRole("button", { name: "Закрыть список обновлений" }), { key: "Escape" });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});

it("uses the last opened revision per account and clears it when notes change", async () => {
  localStorage.setItem(storageKey, "first");
  const load = vi.fn().mockResolvedValueOnce(serveNotes("first")).mockResolvedValueOnce(serveNotes("second")).mockResolvedValueOnce(serveNotes("second"));
  vi.stubGlobal("fetch", load);
  const result = render(<ReleaseNotesDrawer accountId="reader" />);
  const trigger = screen.getByRole("button", { name: "Что нового" });
  fireEvent.click(trigger);
  await screen.findByRole("heading", { name: "Версия 1.0.19" });
  expect(trigger.getAttribute("data-unread")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Закрыть список обновлений" }));
  fireEvent.focus(window);
  await screen.findByText("Есть непрочитанные обновления");
  expect(trigger.getAttribute("data-unread")).toBe("true");
  expect(localStorage.getItem(storageKey)).toBeNull();
  fireEvent.click(trigger);
  expect(trigger.getAttribute("data-unread")).toBeNull();
  expect(localStorage.getItem(storageKey)).toBe("second");
  expect(screen.queryByRole("button", { name: /прочитанным/i })).toBeNull();
  result.unmount();
  render(<ReleaseNotesDrawer accountId="other" />);
  await screen.findByText("Есть непрочитанные обновления");
  expect(localStorage.getItem("arhdesign:release-notes:seen:other")).toBeNull();
});

it("syncs another tab and still works when local storage is disabled", async () => {
  const load = vi.fn().mockResolvedValue(serveNotes("first"));
  vi.stubGlobal("fetch", load);
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
  render(<ReleaseNotesDrawer accountId="reader" />);
  await screen.findByText("Есть непрочитанные обновления");
  const trigger = screen.getByRole("button", { name: "Что нового" });
  fireEvent(window, new StorageEvent("storage", { key: storageKey, newValue: "first" }));
  expect(trigger.getAttribute("data-unread")).toBeNull();
  fireEvent(window, new StorageEvent("storage", { key: storageKey, newValue: null }));
  expect(trigger.getAttribute("data-unread")).toBe("true");
  fireEvent.click(trigger);
  await waitFor(() => expect(trigger.getAttribute("data-unread")).toBeNull());
});
