import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("opens the contact form and exposes accessible validation", async ({ page }) => {
  await page.goto("/");
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: "Обсудить проект" }).first().click();

  const dialog = page.getByRole("dialog", { name: "Обсудить проект" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Отправить" }).click();
  await expect(dialog.getByText("Укажите имя")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Закрыть форму" })).toBeVisible();
});

test("renders the hero carousel with keyboard controls and no overflow", async ({ page }) => {
  await page.goto("/");
  const carousel = page.getByRole("region", { name: "Галерея: Избранные проекты" });

  await expect(carousel.getByText("01 / 30", { exact: true })).toBeVisible();
  await carousel.getByRole("button", { name: "Показать кадр 30" }).click();
  await carousel.getByRole("button", { name: "Следующее изображение" }).click();
  await expect(carousel.getByText("01 / 30", { exact: true })).toBeVisible();
  await carousel.press("ArrowRight");
  await expect(carousel.getByText("02 / 30", { exact: true })).toBeVisible();
  expect(await page.locator("html").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
});

test("has no automated accessibility violations in the contact dialog", async ({ page }) => {
  await page.goto("/");
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: "Обсудить проект" }).first().click();
  await expect(page.getByRole("dialog", { name: "Обсудить проект" })).toBeVisible();

  const result = await new AxeBuilder({ page }).include('[role="dialog"]').analyze();
  expect(result.violations).toEqual([]);
});

test("renders and updates the Retry-After countdown", async ({ page }) => {
  await page.route("**/api/v1/contact-submissions", async (route) => {
    await route.fulfill({ headers: { "Retry-After": "2" }, status: 429 });
  });
  await page.goto("/");
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: "Обсудить проект" }).first().click();

  const dialog = page.getByRole("dialog", { name: "Обсудить проект" });
  await dialog.getByLabel("Имя").fill("Тест");
  await dialog.getByLabel("Телефон или почта").fill("test@example.com");
  await dialog.getByLabel("Тип проекта").fill("Квартира");
  await dialog.getByLabel("О проекте").fill("Тестовая заявка для обратного отсчёта");
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "Отправить" }).click();

  await expect(dialog.getByText(/через 0:02/)).toBeVisible();
  await page.waitForTimeout(1100);
  await expect(dialog.getByText(/через 0:01/)).toBeVisible();
});

test("uses safe VK and Telegram links in the shared social controls", async ({ page }) => {
  await page.goto("/");
  const expectedLinks = ["https://vk.ru/studio_architecture_design", "https://t.me/studio_architecture_design"];

  for (const url of expectedLinks) {
    const links = page.locator(`a[href="${url}"]`);
    expect(await links.count()).toBe(2);
    for (const link of await links.all()) {
      await expect(link).toHaveAttribute("target", "_blank");
      await expect(link).toHaveAttribute("rel", "noopener noreferrer");
    }
  }
});

test("navigates from the fixed header to a landing section and closes the mobile menu", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 900 });
  await page.goto("/");
  await page.getByRole("link", { name: "Услуги" }).click();
  await expect(page).toHaveURL(/\/#services$/);
  await expect(page.locator("#services")).toBeInViewport();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("button", { name: "Открыть меню" }).click();
  await page.getByRole("navigation", { name: "Мобильная навигация" }).getByRole("link", { name: "Об авторе" }).click();
  await expect(page).toHaveURL(/\/#author$/);
  await expect(page.getByRole("button", { name: "Открыть меню" })).toBeVisible();
  await expect(page.locator("#author")).toBeInViewport();
});

test("back actions preserve history and provide a catalogue fallback", async ({ browser, page }) => {
  await page.goto("/#projects");
  await page.getByRole("link", { name: "Подробнее" }).first().click();
  await expect(page).toHaveURL(/\/projects\//);
  await page.getByRole("button", { name: "Назад" }).click();
  await expect(page).toHaveURL(/\/#projects$/);

  const directContext = await browser.newContext({ baseURL: "http://127.0.0.1:3100" });
  const directPage = await directContext.newPage();
  await directPage.goto("/projects/contemporary-harmony");
  await directPage.getByRole("button", { name: "Назад" }).click();
  await expect(directPage).toHaveURL(/\/projects$/);
  await directContext.close();
});

test("keeps the footer contrast in both themes", async ({ page }) => {
  await page.goto("/");
  const footer = page.locator("footer");
  await expect(footer).toBeVisible();
  const lightBackground = await footer.evaluate((element) => getComputedStyle(element).backgroundColor);

  await page.getByRole("button", { name: /тёмную тему/i }).click();
  const darkBackground = await footer.evaluate((element) => getComputedStyle(element).backgroundColor);
  expect(lightBackground).not.toBe(darkBackground);
  await expect(footer).toHaveCSS("color", "rgb(244, 241, 234)");
});

test("keeps every landing section within the viewport across responsive themes", async ({ page }) => {
  for (const viewport of [
    { name: "desktop", width: 1440, height: 900 },
    { name: "tablet", width: 768, height: 1024 },
    { name: "mobile", width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    for (const dark of [false, true]) {
      if (dark) await page.getByRole("button", { name: /тёмную тему/i }).click();
      for (const id of ["projects", "services", "process", "author", "contact"]) {
        await expect(page.locator(`#${id}`)).toBeVisible();
      }
      expect(await page.locator("html").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    }
  }
});
