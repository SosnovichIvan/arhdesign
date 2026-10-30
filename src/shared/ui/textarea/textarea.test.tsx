// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { Textarea } from "./textarea";

afterEach(cleanup);

describe("Textarea", () => {
  it("matches the shared input spacing and focus treatment", () => {
    render(<Textarea aria-label="Описание" />);
    const textarea = screen.getByRole("textbox", { name: "Описание" });

    expect(textarea.className).toContain("px-3");
    expect(textarea.className).toContain("border-b");
    expect(textarea.className).toContain("focus:border-field-focus");
    expect(textarea.className).toContain("focus:outline-none");
  });
});
