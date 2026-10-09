// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Select, type SelectOption } from "./select";

const options = [
  { value: "first", label: "Первый" },
  { value: "second", label: "Второй" },
  { value: "third", label: "Третий" },
] as const satisfies readonly [SelectOption, ...SelectOption[]];

afterEach(cleanup);

describe("Select", () => {
  it("selects a hovered option and returns focus to the trigger", () => {
    const onValueChange = vi.fn();
    render(<Select label="Вариант" onValueChange={onValueChange} options={options} required value="first" />);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });

    expect(trigger.getAttribute("aria-required")).toBe("true");
    expect(trigger.className).toContain("px-3");
    fireEvent.click(trigger);
    expect(screen.getByRole("listbox").className).toContain("fixed");
    expect(screen.getByRole("listbox").className).toContain("overflow-y-auto");
    const third = screen.getByRole("option", { name: "Третий" });
    fireEvent.mouseEnter(third);
    expect(trigger.getAttribute("aria-activedescendant")).toBe(third.id);
    fireEvent.click(third);

    expect(onValueChange).toHaveBeenCalledWith("third");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(trigger);
  });

  it("closes on a repeated click and an outside pointer", () => {
    render(<><Select label="Вариант" onValueChange={vi.fn()} options={options} value="second" /><button type="button">Снаружи</button></>);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });

    fireEvent.click(trigger);
    expect(screen.getByRole("listbox")).toBeTruthy();
    fireEvent.click(trigger);
    expect(screen.queryByRole("listbox")).toBeNull();

    fireEvent.click(trigger);
    fireEvent.pointerDown(screen.getByRole("button", { name: "Снаружи" }));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("supports wrapping arrows, Home, End, Space and Escape", () => {
    const onValueChange = vi.fn();
    render(<Select label="Вариант" onValueChange={onValueChange} options={options} value="first" />);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });

    fireEvent.keyDown(trigger, { key: "ArrowUp" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-2");
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-0");
    fireEvent.keyDown(trigger, { key: "End" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-2");
    fireEvent.keyDown(trigger, { key: "Home" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-0");
    fireEvent.keyDown(trigger, { key: " " });
    expect(onValueChange).toHaveBeenCalledWith("first");

    fireEvent.keyDown(trigger, { key: "Enter" });
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    fireEvent.keyDown(trigger, { key: "Escape" });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
  });

  it("supports optional styling, test selectors, disabled state and an unknown persisted value", () => {
    const { container } = render(<Select className="custom-select" dataCy="example" disabled label="Вариант" onValueChange={vi.fn()} options={options} value="legacy" />);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });

    expect(container.firstElementChild?.className).toContain("custom-select");
    expect(trigger.getAttribute("data-cy")).toBe("example");
    expect(trigger).toHaveProperty("disabled", true);
    expect(trigger.textContent).toContain("Первый");
    fireEvent.click(trigger);
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("positions the menu above near the viewport bottom and responds to resize", () => {
    const rect = { bottom: 790, height: 48, left: -20, right: 180, top: 742, width: 200, x: -20, y: 742, toJSON: () => ({}) } as DOMRect;
    const spy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(rect);
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 800 });
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 300 });
    render(<Select dataCy="positioned" label="Вариант" onValueChange={vi.fn()} options={options} value="second" />);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });
    fireEvent.click(trigger);
    const listbox = screen.getByRole("listbox");
    expect(listbox.style.bottom).not.toBe("");
    expect(screen.getByRole("option", { name: "Первый" }).getAttribute("data-cy")).toBe("positioned-option-first");
    fireEvent(window, new Event("resize"));
    spy.mockRestore();
  });

  it("moves within an open menu with arrows and opens with Space", () => {
    render(<Select label="Вариант" onValueChange={vi.fn()} options={options} value="second" />);
    const trigger = screen.getByRole("combobox", { name: "Вариант" });
    fireEvent.keyDown(trigger, { key: " " });
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-2");
    fireEvent.keyDown(trigger, { key: "ArrowUp" });
    expect(trigger.getAttribute("aria-activedescendant")).toContain("option-1");
  });
});
