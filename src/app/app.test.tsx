// @vitest-environment jsdom

import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { createElement, type ImgHTMLAttributes } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/image", () => ({
  default: ({ fill, alt, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => {
    void fill;
    return createElement("img", { ...props, alt: alt ?? "" });
  },
}));
vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
const { notFound } = vi.hoisted(() => ({ notFound: vi.fn(() => { throw new Error("not found"); }) }));
const router = vi.hoisted(() => ({ back: vi.fn(), push: vi.fn() }));
vi.mock("next/navigation", () => ({ notFound, useRouter: () => router }));

import RootLayout, { metadata } from "./layout";
import HomePage from "./page";
import ProjectsPage from "./projects/page";
import ProjectPage, { generateMetadata, generateStaticParams } from "./projects/[slug]/page";
import robots from "./robots";
import sitemap from "./sitemap";
import { ProjectCard } from "@/shared/components/projectCard";
import { ProjectCarousel } from "@/shared/components/projectCarousel";
import { BackButton } from "@/shared/components/backButton";
import { SiteFooter } from "@/shared/components/siteFooter";
import { SiteHeader } from "@/shared/components/siteHeader";
import { SocialLinks } from "@/shared/components/socialLinks";
import { projects } from "@/shared/config";
import { ThemeToggle } from "@/features/themeToggle";
import { CarouselControl, Container, Dialog, Typography } from "@/shared/ui";

afterEach(() => {
  cleanup();
  window.sessionStorage.clear();
  window.history.replaceState({}, "", "/");
  document.documentElement.dataset.theme = "light";
  vi.clearAllMocks();
});

describe("public portfolio UI", () => {
  it("returns through browser history and uses a fallback for direct visits", () => {
    window.history.replaceState({}, "", "/projects/contemporary-harmony");
    window.sessionStorage.setItem("arhdesign:project-return", JSON.stringify({ target: "/projects/contemporary-harmony" }));
    Object.defineProperty(window.history, "length", { configurable: true, value: 2 });
    render(<BackButton fallbackHref="/projects" />);
    fireEvent.click(screen.getByRole("button", { name: "Назад" }));
    expect(router.back).toHaveBeenCalledOnce();

    cleanup();
    Object.defineProperty(window.history, "length", { configurable: true, value: 1 });
    render(<BackButton fallbackHref="/projects" />);
    fireEvent.click(screen.getByRole("button", { name: "Назад" }));
    expect(router.push).toHaveBeenCalledWith("/projects");
  });

  it("clears invalid return markers and accepts a same-origin referrer", () => {
    Object.defineProperty(window.history, "length", { configurable: true, value: 1 });
    window.sessionStorage.setItem("arhdesign:project-return", "not-json");
    render(<BackButton fallbackHref="/projects" />);
    fireEvent.click(screen.getByRole("button", { name: "Назад" }));
    expect(window.sessionStorage.getItem("arhdesign:project-return")).toBeNull();
    expect(router.push).toHaveBeenCalledWith("/projects");

    cleanup();
    Object.defineProperty(window.history, "length", { configurable: true, value: 2 });
    Object.defineProperty(document, "referrer", { configurable: true, value: window.location.origin + "/#projects" });
    render(<BackButton fallbackHref="/projects" />);
    fireEvent.click(screen.getByRole("button", { name: "Назад" }));
    expect(router.back).toHaveBeenCalledOnce();
    Object.defineProperty(document, "referrer", { configurable: true, value: "" });
  });

  it("renders layout, landing and catalog content", () => {
    render(<RootLayout><HomePage /></RootLayout>);
    expect(screen.getByRole("heading", { name: "Пространства, которые остаются" })).toBeTruthy();
    expect(screen.getAllByRole("link", { name: "Подробнее" })).toHaveLength(3);
    expect(screen.getByRole("link", { name: "Перейти" }).getAttribute("href")).toBe("/projects");
    expect(screen.getAllByText("Полисмакова Светлана", { exact: false }).length).toBeGreaterThan(0);

    render(<ProjectsPage />);
    expect(screen.getByRole("heading", { name: "Проекты" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Назад" })).toBeTruthy();
  });

  it("renders shared cards, controls and social links", () => {
    render(<><Container><Typography as="h2" variant="title">Тест</Typography><ProjectCard project={projects[0]} /><CarouselControl direction="next" /><CarouselControl direction="previous" /></Container><SiteHeader /><SiteFooter /><SocialLinks /></>);
    expect(screen.getByRole("img", { name: projects[0].title })).toBeTruthy();
    expect(screen.getAllByRole("link", { name: /Открыть VK/ }).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Следующее изображение" })).toBeTruthy();
    fireEvent.click(screen.getByRole("link", { name: "Подробнее" }));
    expect(JSON.parse(window.sessionStorage.getItem("arhdesign:project-return") ?? "{}").target).toBe(`/projects/${projects[0].slug}`);
  });

  it("opens and closes the mobile navigation menu", () => {
    render(<SiteHeader />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть меню" }));
    const menu = screen.getByRole("navigation", { name: "Мобильная навигация" });
    expect(menu).toBeTruthy();
    fireEvent.click(within(menu).getByRole("link", { name: "Услуги" }));
    expect(screen.queryByRole("navigation", { name: "Мобильная навигация" })).toBeNull();
  });

  it("changes slides through controls and keyboard", () => {
    render(<ProjectCarousel images={projects[0].images.slice(0, 3)} title="Тестовая галерея" />);
    const carousel = screen.getByRole("region", { name: "Галерея: Тестовая галерея" });
    fireEvent.click(screen.getByRole("button", { name: "Предыдущее изображение" }));
    expect(screen.getByText("03 / 03")).toBeTruthy();
    fireEvent.keyDown(carousel, { key: "Home" });
    expect(screen.getByText("01 / 03")).toBeTruthy();
    fireEvent.keyDown(carousel, { key: "End" });
    expect(screen.getByText("03 / 03")).toBeTruthy();
    fireEvent.keyDown(carousel, { key: "ArrowLeft" });
    expect(screen.getByText("02 / 03")).toBeTruthy();
    fireEvent.keyDown(carousel, { key: "ArrowRight" });
    expect(screen.getByText("03 / 03")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Показать кадр 1" }));
    expect(screen.getByText("01 / 03")).toBeTruthy();
  });

  it("opens the active image fullscreen and changes slides with a pointer swipe", () => {
    render(<ProjectCarousel images={projects[0].images.slice(0, 3)} title="Тестовая галерея" />);
    const imageButton = screen.getByRole("button", { name: "Открыть изображение 1 на полный экран" });
    fireEvent.pointerUp(imageButton, { clientX: 120 });
    fireEvent.pointerDown(imageButton, { clientX: 120 });
    fireEvent.pointerUp(imageButton, { clientX: 100 });
    expect(screen.getByText("01 / 03")).toBeTruthy();
    fireEvent.pointerDown(imageButton, { clientX: 260 });
    fireEvent.pointerUp(imageButton, { clientX: 120 });
    expect(screen.getByText("02 / 03")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: /Полноэкранный просмотр/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Открыть изображение 2 на полный экран" }));
    fireEvent.click(screen.getByRole("button", { name: "Открыть изображение 2 на полный экран" }));
    expect(screen.getByRole("dialog", { name: "Полноэкранный просмотр: Тестовая галерея" })).toBeTruthy();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: /Полноэкранный просмотр/ })).toBeNull();
  });

  it("pauses automatic carousel changes for ten seconds after manual activity", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-09T12:00:00Z"));
    render(<ProjectCarousel images={projects[0].images.slice(0, 3)} title="Тестовая галерея" />);
    const carousel = screen.getByRole("region", { name: "Галерея: Тестовая галерея" });
    act(() => vi.advanceTimersByTime(5_000));
    expect(screen.getByText("02 / 03")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Следующее изображение" }));
    act(() => vi.advanceTimersByTime(5_000));
    expect(screen.getByText("03 / 03")).toBeTruthy();
    act(() => vi.advanceTimersByTime(5_000));
    expect(screen.getByText("01 / 03")).toBeTruthy();
    void carousel;
    vi.useRealTimers();
  });

  it("toggles the document theme and manages dialog keyboard interactions", () => {
    const onClose = vi.fn();
    render(<><ThemeToggle /><Dialog isOpen label="Тестовый диалог" onClose={onClose}><button type="button">Первый</button><button type="button">Последний</button></Dialog></>);
    fireEvent.click(screen.getByRole("button", { name: "Включить тёмную тему" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
    fireEvent.click(screen.getByRole("button", { name: "Включить светлую тему" }));
    expect(document.documentElement.dataset.theme).toBe("light");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
    const dialog = screen.getByRole("dialog", { name: "Тестовый диалог" });
    fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
    fireEvent.keyDown(dialog, { key: "Tab" });
  });

  it("generates page metadata and routing manifests", async () => {
    expect(metadata.alternates?.canonical).toBe("/");
    expect(generateStaticParams()).toHaveLength(3);
    expect((await generateMetadata({ params: Promise.resolve({ slug: projects[0].slug }) })).title).toBe(projects[0].title);
    expect(await generateMetadata({ params: Promise.resolve({ slug: "missing" }) })).toEqual({});
    expect(robots().rules).toEqual({ allow: "/", userAgent: "*" });
    expect(sitemap()).toHaveLength(5);
  });

  it("renders a project page and sends unknown projects to notFound", async () => {
    render(await ProjectPage({ params: Promise.resolve({ slug: projects[1].slug }) }));
    expect(screen.getByRole("heading", { name: projects[1].title })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Назад" })).toBeTruthy();
    await expect(ProjectPage({ params: Promise.resolve({ slug: "missing" }) })).rejects.toThrow("not found");
    expect(notFound).toHaveBeenCalledOnce();
  });
});
