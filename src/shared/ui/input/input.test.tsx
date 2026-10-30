// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { Input } from "./input";

afterEach(cleanup);

describe("Input", () => {
  it("uses the shared field spacing and keeps consumer classes", () => {
    render(<Input aria-label="Логин" className="custom-field" placeholder="Введите логин" />);
    const input = screen.getByRole("textbox", { name: "Логин" });

    expect(input.className).toContain("px-3");
    expect(input.className).toContain("border-b");
    expect(input.className).toContain("focus:border-field-focus");
    expect(input.className).toContain("custom-field");
  });
});
