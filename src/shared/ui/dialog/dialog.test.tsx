// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Dialog } from "./dialog";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Dialog", () => {
  it("does not render while closed", () => {
    render(<Dialog isOpen={false} label="Проверка" onClose={vi.fn()}><p>Содержимое</p></Dialog>);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("focuses the requested control, traps focus and closes by Escape", () => {
    const onClose = vi.fn();
    render(<Dialog isOpen label="Проверка" onClose={onClose}>
      <input data-dialog-initial-focus aria-label="Первое поле" />
      <button type="button">Последнее действие</button>
    </Dialog>);
    const dialog = screen.getByRole("dialog", { name: "Проверка" });
    const initial = screen.getByLabelText("Первое поле");
    const last = screen.getByRole("button", { name: "Последнее действие" });
    expect(document.activeElement).toBe(initial);
    last.focus();
    fireEvent.keyDown(dialog, { key: "Tab" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Закрыть форму" }));
    initial.focus();
    fireEvent.keyDown(dialog, { key: "Tab" });
    expect(document.activeElement).toBe(initial);
    fireEvent.keyDown(dialog, { key: "ArrowDown" });
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("wraps backwards and only closes for a click on the overlay", () => {
    const onClose = vi.fn();
    render(<Dialog isOpen label="Проверка" onClose={onClose}><button type="button">Действие</button></Dialog>);
    const close = screen.getByRole("button", { name: "Закрыть форму" });
    const dialog = screen.getByRole("dialog", { name: "Проверка" });
    close.focus();
    fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Действие" }));
    fireEvent.mouseDown(dialog);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.mouseDown(dialog.parentElement!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("leaves Tab alone when a dialog temporarily has no focusable controls", () => {
    render(<Dialog isOpen label="Проверка" onClose={vi.fn()}><p>Текст</p></Dialog>);
    const dialog = screen.getByRole("dialog", { name: "Проверка" });
    Object.defineProperty(dialog, "querySelectorAll", { configurable: true, value: () => [] });
    fireEvent.keyDown(dialog, { key: "Tab" });
    expect(dialog).toBeTruthy();
  });
});
