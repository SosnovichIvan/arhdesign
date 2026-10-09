// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { HomeScrollReset } from "./homeScrollReset";

afterEach(() => {
  cleanup();
  sessionStorage.clear();
  vi.unstubAllGlobals();
});

describe("HomeScrollReset", () => {
  it("does nothing without a pending home reset", () => {
    const scrollTo = vi.fn();
    vi.stubGlobal("scrollTo", scrollTo);
    render(<HomeScrollReset />);
    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("resets the home position and restores browser scroll handling", () => {
    const scrollTo = vi.fn();
    const frames: FrameRequestCallback[] = [];
    vi.stubGlobal("scrollTo", scrollTo);
    vi.stubGlobal("requestAnimationFrame", vi.fn((callback: FrameRequestCallback) => { frames.push(callback); return frames.length; }));
    Object.defineProperty(history, "scrollRestoration", { configurable: true, value: "auto", writable: true });
    sessionStorage.setItem("arhdesign:scroll-home-top", "1");
    document.documentElement.scrollTop = 120;
    document.body.scrollTop = 120;

    render(<HomeScrollReset />);
    expect(sessionStorage.getItem("arhdesign:scroll-home-top")).toBeNull();
    expect(history.scrollRestoration).toBe("manual");
    expect(scrollTo).toHaveBeenCalledTimes(1);
    act(() => { frames.shift()?.(0); frames.shift()?.(0); });
    expect(document.documentElement.scrollTop).toBe(0);
    expect(document.body.scrollTop).toBe(0);
    expect(scrollTo).toHaveBeenCalledTimes(2);
    expect(history.scrollRestoration).toBe("auto");
  });
});
