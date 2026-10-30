// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RoleSelect } from "./roleSelect";

afterEach(cleanup);

describe("RoleSelect", () => {
  it("provides the supported project roles to the shared select", () => {
    const onChange = vi.fn();
    render(<RoleSelect onChange={onChange} value="customer" />);
    const trigger = screen.getByRole("combobox", { name: "Роль" });

    fireEvent.click(trigger);
    expect(screen.getAllByRole("option").map((option) => option.textContent)).toEqual([
      expect.stringContaining("Заказчик"),
      "Дизайнер",
      "Прораб",
      "Архитектор",
    ]);
    fireEvent.click(screen.getByRole("option", { name: "Архитектор" }));

    expect(onChange).toHaveBeenCalledWith("architect");
  });
});
