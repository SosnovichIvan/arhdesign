// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Autocomplete, type AutocompleteOption } from "./autocomplete";

const options: AutocompleteOption[] = [
  { id: "1", label: "Анна Дизайнер", description: "@anna · anna@example.com" },
  { id: "2", label: "Иван Архитектор", description: "@ivan · ivan@example.com" },
];

afterEach(cleanup);

describe("Autocomplete", () => {
  it("supports keyboard navigation and returns the selected option", () => {
    const onSelect = vi.fn();
    render(<Autocomplete label="Участник" onSelect={onSelect} onValueChange={vi.fn()} options={options} required value="ан" />);
    const input = screen.getByRole("combobox", { name: "Участник" });

    fireEvent.focus(input);
    expect(screen.getByRole("listbox")).toBeTruthy();
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(input.getAttribute("aria-activedescendant")).toContain("option-1");
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onSelect).toHaveBeenCalledWith(options[1]);
    expect(input.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(input);
  });

  it("shows loading, empty and error states and clears the query", () => {
    const onValueChange = vi.fn();
    const { rerender } = render(<Autocomplete error="Поиск временно недоступен" label="Участник" loading onSelect={vi.fn()} onValueChange={onValueChange} options={[]} value="ан" />);
    const input = screen.getByRole("combobox", { name: "Участник" });
    fireEvent.focus(input);
    expect(screen.getByRole("status").textContent).toContain("Ищем");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("alert").textContent).toContain("недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Очистить поле «Участник»" }));
    expect(onValueChange).toHaveBeenCalledWith("");

    rerender(<Autocomplete emptyText="Введите минимум 2 символа" label="Участник" onSelect={vi.fn()} onValueChange={onValueChange} options={[]} value="" />);
    expect(screen.getByText("Введите минимум 2 символа")).toBeTruthy();
  });

  it("selects by pointer and closes on Escape or an outside pointer", () => {
    const onSelect = vi.fn();
    render(<><Autocomplete label="Участник" onSelect={onSelect} onValueChange={vi.fn()} options={options} value="а" /><button type="button">Снаружи</button></>);
    const input = screen.getByRole("combobox", { name: "Участник" });
    fireEvent.focus(input);
    fireEvent.click(screen.getByRole("option", { name: /Анна Дизайнер/ }));
    expect(onSelect).toHaveBeenCalledWith(options[0]);
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
    fireEvent.focus(input);
    fireEvent.pointerDown(screen.getByRole("button", { name: "Снаружи" }));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("supports Home, End and reverse wrapping and ignores navigation without options", () => {
    const { rerender } = render(<Autocomplete label="Участник" onSelect={vi.fn()} onValueChange={vi.fn()} options={options} value="а" />);
    const input = screen.getByRole("combobox", { name: "Участник" });
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowUp" });
    expect(input.getAttribute("aria-activedescendant")).toContain("option-1");
    fireEvent.keyDown(input, { key: "Home" });
    expect(input.getAttribute("aria-activedescendant")).toContain("option-0");
    fireEvent.keyDown(input, { key: "End" });
    expect(input.getAttribute("aria-activedescendant")).toContain("option-1");
    rerender(<Autocomplete disabled label="Участник" onSelect={vi.fn()} onValueChange={vi.fn()} options={[]} value="а" />);
    expect(screen.queryByRole("button", { name: /Очистить/ })).toBeNull();
    fireEvent.keyDown(screen.getByRole("combobox", { name: "Участник" }), { key: "ArrowDown" });
  });

  it("renders an option without a description and resets active selection when typing", () => {
    const onValueChange = vi.fn();
    render(<Autocomplete dataCy="candidate" label="Участник" onSelect={vi.fn()} onValueChange={onValueChange} options={[{ id: "3", label: "Без описания" }]} placeholder="Поиск" value="б" />);
    const input = screen.getByRole("combobox", { name: "Участник" });
    expect(input.getAttribute("data-cy")).toBe("candidate");
    fireEvent.change(input, { target: { value: "бе" } });
    expect(onValueChange).toHaveBeenCalledWith("бе");
    expect(screen.getByRole("option", { name: "Без описания" })).toBeTruthy();
  });
});
