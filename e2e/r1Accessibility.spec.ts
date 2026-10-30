import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const viewports = [
  { name: "desktop", width: 1440, height: 900 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "mobile", width: 390, height: 844 },
] as const;
const themes = ["light", "dark"] as const;

async function mockAccountApi(page: Page, theme: "light" | "dark") {
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/v1/me") return route.fulfill({ json: { globalRole: "super_admin", id: "admin-1", login: "svetaZZZ" } });
    if (url.pathname === "/api/v1/settings") return route.fulfill({ json: { theme } });
    if (url.pathname === "/api/v1/notifications") return route.fulfill({ json: { hasMore: false, items: [], nextCursor: null, unreadCount: 0 } });
    if (url.pathname === "/api/v1/projects") return route.fulfill({ json: { hasMore: false, items: [], nextCursor: null } });
    return route.fulfill({ json: {} });
  });
}

for (const viewport of viewports) {
  for (const theme of themes) {
    test(`R1 auth and account axe: ${viewport.name}, ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await mockAccountApi(page, theme);
      await page.goto("/");
      if (viewport.name === "mobile") {
        await page.getByRole("button", { name: "Открыть меню" }).click();
        await page.locator('[data-cy="auth-open-mobile"]').click();
      } else {
        await page.locator('[data-cy="auth-open"]').click();
      }
      const dialog = page.getByRole("dialog", { name: "Авторизация" });
      await expect(dialog).toBeVisible();
      await expect(page.locator('[data-cy="login-identifier"]')).toBeFocused();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      const dialogAudit = await new AxeBuilder({ page }).include('[role="dialog"]').analyze();
      expect(dialogAudit.violations, JSON.stringify(dialogAudit.violations, null, 2)).toEqual([]);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();

      await page.goto("/account");
      await expect(page.getByRole("heading", { name: "Проекты" })).toBeVisible();
	  await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
	  // Theme tokens change immediately while controls with transition-colors
	  // finish their visual interpolation. Audit the stable state, not a frame
	  // in the middle of that intentional transition.
	  await page.waitForTimeout(250);
      const accountAudit = await new AxeBuilder({ page }).include("main").analyze();
      expect(accountAudit.violations, JSON.stringify(accountAudit.violations, null, 2)).toEqual([]);
      const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(horizontalOverflow).toBeLessThanOrEqual(1);
    });
  }
}

test("R1 remains operable at 200% zoom equivalent and reduced motion", async ({ page }) => {
	// A 384 CSS-pixel viewport is the layout viewport produced when a
	// 768-pixel tablet viewport is zoomed to 200%. Setting CSS `zoom` here
	// would not update media queries and would therefore test an impossible
	// browser state (tablet breakpoints inside a mobile-sized layout viewport).
	await page.setViewportSize({ width: 384, height: 1024 });
	await page.emulateMedia({ reducedMotion: "reduce" });
	await mockAccountApi(page, "dark");
	await page.goto("/account");
	await expect(page.getByRole("heading", { name: "Проекты" })).toBeVisible();
  await expect(page.locator('[data-cy="create-project-link"]')).toBeVisible();
  const transitionDuration = await page.locator('[data-cy="create-project-link"]').evaluate((element) => getComputedStyle(element).transitionDuration);
  expect(Number.parseFloat(transitionDuration)).toBeLessThanOrEqual(0.01);
  const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(horizontalOverflow).toBeLessThanOrEqual(1);
});
