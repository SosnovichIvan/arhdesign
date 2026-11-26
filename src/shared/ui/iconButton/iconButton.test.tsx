// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { IconButton } from "./iconButton";

afterEach(cleanup);

describe("IconButton", () => {
  it("can place a tooltip below a button in the header", () => {
    const { container } = render(<IconButton aria-label="Обновления" tooltip="Что нового" tooltipPlacement="bottom">+</IconButton>);
    const tooltip = container.querySelector('[role="tooltip"]');
    expect(tooltip?.className).toContain("top-full");
    expect(tooltip?.className).not.toContain("bottom-full");
  });
  it("links an optional tooltip to the icon button for pointer and keyboard users", () => {
    render(<IconButton aria-describedby="context" aria-label="Настроить период" tooltip="Изменить период"><span aria-hidden="true">⚙</span></IconButton>);
    const button = screen.getByRole("button", { name: "Настроить период" });
    const tooltip = screen.getByRole("tooltip");
    expect(button.getAttribute("aria-describedby")).toContain("context");
    expect(button.getAttribute("aria-describedby")).toContain(tooltip.id);
    expect(tooltip.textContent).toBe("Изменить период");
    button.focus();
    expect(document.activeElement).toBe(button);
  });

  it("keeps the original button markup when no tooltip is requested", () => {
    const { container } = render(<IconButton aria-label="Закрыть">×</IconButton>);
    expect(container.firstElementChild?.tagName).toBe("BUTTON");
    expect(screen.queryByRole("tooltip")).toBeNull();
  });
});
