// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DatePicker } from "./datePicker";

afterEach(cleanup);

describe("DatePicker", () => {
  it("uses the project styling and selects a day from another month", () => {
    const onValueChange = vi.fn();
    render(<DatePicker dataCy="start-date" label="Начало" onValueChange={onValueChange} required value="2026-10-10" />);
    const trigger = screen.getByRole("combobox", { name: "Начало" });

    expect(trigger.textContent).toContain("10 октября 2026");
    expect(trigger.className).toContain("pl-3");
    fireEvent.click(trigger);
    const calendar = screen.getByRole("dialog", { name: "Календарь: Начало" });
    expect(calendar).toBeTruthy();
    expect(calendar.className).toContain("w-[min(20rem,calc(100vw-2rem))]");
    expect(calendar.className).toContain("fixed");
    expect(screen.getByText("Октябрь 2026")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Следующий месяц" }));
    expect(screen.getByText("Ноябрь 2026")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: formatLongDate(new Date(2026, 10, 5)) }));

    expect(onValueChange).toHaveBeenCalledWith("2026-11-05");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("disables dates before the minimum and closes with Escape", () => {
    const onValueChange = vi.fn();
    render(<DatePicker label="Завершение" min="2026-10-10" onValueChange={onValueChange} value="" />);
    const trigger = screen.getByRole("combobox", { name: "Завершение" });

    fireEvent.click(trigger);
    expect(screen.getByRole("button", { name: formatLongDate(new Date(2026, 9, 9)) })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: formatLongDate(new Date(2026, 9, 10)) })).toHaveProperty("disabled", false);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("clears a value and closes on an outside pointer", () => {
    const onValueChange = vi.fn();
    render(<><DatePicker label="Дата" onValueChange={onValueChange} value="2026-10-10" /><button type="button">Снаружи</button></>);

    fireEvent.click(screen.getByRole("button", { name: "Очистить поле «Дата»" }));
    expect(onValueChange).toHaveBeenCalledWith("");
    fireEvent.click(screen.getByRole("combobox", { name: "Дата" }));
    fireEvent.pointerDown(screen.getByRole("button", { name: "Снаружи" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("supports the calendar icon, previous month, in-dialog clear and disabled state", () => {
    const onValueChange = vi.fn();
    const { rerender } = render(<DatePicker disabled label="Дата" onValueChange={onValueChange} value="" />);
    expect(screen.getByRole("combobox", { name: "Дата" })).toHaveProperty("disabled", true);
    expect(screen.getByRole("button", { name: "Открыть календарь: Дата" })).toHaveProperty("disabled", true);

    rerender(<DatePicker label="Дата" onValueChange={onValueChange} value="2026-10-10" />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть календарь: Дата" }));
    fireEvent.click(screen.getByRole("button", { name: "Предыдущий месяц" }));
    expect(screen.getByText("Сентябрь 2026")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Очистить дату" }));
    expect(onValueChange).toHaveBeenCalledWith("");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("associates an inline validation error with the date trigger", () => {
    render(<DatePicker dataCy="finish-date" error="Проверьте диапазон дат" label="Завершение" onValueChange={vi.fn()} value="2026-10-10" />);
    const trigger = screen.getByRole("combobox", { name: "Завершение" });
    const error = screen.getByText("Проверьте диапазон дат");

    expect(trigger.getAttribute("aria-invalid")).toBe("true");
    expect(trigger.getAttribute("aria-describedby")).toBe(error.id);
    expect(error.getAttribute("data-cy")).toBe("finish-date-error");
    expect(trigger.parentElement?.className).toContain("border-red-700");
  });

  it("keeps the calendar inside the viewport and opens above when there is no space below", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
      if (this.getAttribute("role") === "combobox") return rectangle({ bottom: 748, left: 900, right: 948, top: 700, width: 48, height: 48 });
      if (this.getAttribute("role") === "dialog") return rectangle({ bottom: 400, left: 0, right: 320, top: 0, width: 320, height: 400 });
      return rectangle({ bottom: 0, left: 0, right: 0, top: 0, width: 0, height: 0 });
    });
    render(<DatePicker label="Окончание" onValueChange={vi.fn()} value="" />);

    fireEvent.click(screen.getByRole("combobox", { name: "Окончание" }));
    const calendar = screen.getByRole("dialog", { name: "Календарь: Окончание" });

    expect(calendar.getAttribute("data-placement")).toBe("top");
    expect(calendar.style.left).toBe("688px");
    expect(calendar.style.top).toBe("292px");
    expect(calendar.style.maxHeight).toBe("676px");
    expect(calendar.style.visibility).toBe("");
  });
});

function rectangle({ bottom, height, left, right, top, width }: { bottom: number; height: number; left: number; right: number; top: number; width: number }): DOMRect {
  return { bottom, height, left, right, top, width, x: left, y: top, toJSON: () => ({}) } as DOMRect;
}

function formatLongDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}
