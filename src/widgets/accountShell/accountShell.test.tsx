// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement, type ImgHTMLAttributes } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AccountShell, useAccountNavigation } from "./accountShell";

const replace = vi.fn();
const refresh = vi.fn();
const loadCurrentAccount = vi.hoisted(() => vi.fn().mockResolvedValue({ globalRole: null, id: "user-1", login: "user" }));
const ProjectApiErrorMock = vi.hoisted(() => class ProjectApiError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
	}
});
const router = { replace, refresh };
let pathname = "/account";

vi.mock("next/navigation", () => ({ usePathname: () => pathname, useRouter: () => router }));
vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
vi.mock("next/image", () => ({ default: ({ alt, ...props }: ImgHTMLAttributes<HTMLImageElement>) => createElement("img", { ...props, alt: alt ?? "" }) }));
vi.mock("@/features/themeToggle", () => ({ ThemeToggle: () => <button type="button">Тема</button> }));
vi.mock("@/features/accountNotifications", () => ({ AccountNotificationCenter: () => <button aria-label="Уведомления" type="button" /> }));
vi.mock("@/features/accountProjects/api/projects", () => ({
	loadCurrentAccount,
	ProjectApiError: ProjectApiErrorMock,
}));

afterEach(() => {
  cleanup();
  pathname = "/account";
  vi.clearAllMocks();
	loadCurrentAccount.mockResolvedValue({ globalRole: null, id: "user-1", login: "user" });
  vi.unstubAllGlobals();
});

function Counter() {
  const { setProjectCount } = useAccountNavigation();
  return <button onClick={() => setProjectCount(4)} type="button">Обновить счётчик</button>;
}

function OutsideConsumer() {
  useAccountNavigation();
  return null;
}

describe("AccountShell", () => {
  it("requires the provider", () => {
    expect(() => render(<OutsideConsumer />)).toThrow("useAccountNavigation must be used inside AccountShell");
  });

  it("renders account navigation, active project state and count", async () => {
    render(<AccountShell><Counter /></AccountShell>);
    expect(screen.getByRole("navigation").textContent).toContain("Проекты");
    expect(screen.getByRole("link", { name: /Проекты/ }).getAttribute("aria-current")).toBe("page");
	fireEvent.click(await screen.findByRole("button", { name: "Обновить счётчик" }));
    expect(screen.getByRole("link", { name: /Проекты/ }).textContent).toContain("4");
    expect(screen.getAllByRole("button", { name: "Тема" })).toHaveLength(2);
  });

	it("shows user administration navigation only to a global administrator", async () => {
		loadCurrentAccount.mockResolvedValueOnce({ globalRole: "super_admin", id: "admin-1", login: "admin" });
		render(<AccountShell><span>Проекты</span></AccountShell>);
		expect(await screen.findByRole("link", { name: "Пользователи" })).toBeTruthy();
	});

	it("also grants administration navigation to the technical administrator", async () => {
		loadCurrentAccount.mockResolvedValueOnce({ globalRole: "technical_admin", id: "tech-1", login: "tech" });
		render(<AccountShell><span>Проекты</span></AccountShell>);
		expect(await screen.findByRole("link", { name: "Пользователи" })).toBeTruthy();
	});

	it("marks nested project and calendar routes active", () => {
		pathname = "/account/projects/new";
		render(<AccountShell><span>Форма</span></AccountShell>);
		expect(screen.getByRole("link", { name: /Проекты/ }).getAttribute("aria-current")).toBe("page");
		cleanup();
		pathname = "/account/calendar";
		render(<AccountShell><span>Календарь</span></AccountShell>);
    expect(screen.getByRole("link", { name: "Календарь" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: /Проекты/ }).getAttribute("aria-current")).toBeNull();
  });

  it("switches to project navigation inside a project", async () => {
    pathname = "/account/projects/project-1";
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "project-1", name: "Дом в Миллениум Парк" }), { headers: { "Content-Type": "application/json" }, status: 200 })));
    render(<AccountShell><span>Сводка проекта</span></AccountShell>);
    expect(await screen.findByText("Сводка проекта")).toBeTruthy();
    expect(screen.queryByText("Дом в Миллениум Парк")).toBeNull();
    expect(screen.queryByText("ПРОЕКТ")).toBeNull();
    expect(screen.getByRole("navigation", { name: "Разделы проекта" }).textContent).toContain("Финансы проекта");
    expect(screen.getByRole("navigation", { name: "Разделы проекта" }).className).toContain("grid-cols-2");
    expect(screen.getByRole("navigation", { name: "Разделы проекта" }).className).not.toContain("overflow-x-auto");
    expect(screen.getByRole("link", { name: "Задачи" }).getAttribute("href")).toBe("/account/projects/project-1/tasks");
    expect(screen.getByRole("link", { name: "Календарь проекта" }).getAttribute("href")).toBe("/account/projects/project-1/calendar");
    expect(screen.getByRole("link", { name: "Финансы проекта" }).getAttribute("href")).toBe("/account/projects/project-1/finances");
    expect(screen.getByRole("link", { name: "Сводка" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: /Назад к проектам/ }).getAttribute("href")).toBe("/account");
    expect(screen.queryByRole("link", { name: /Проекты/ })).toBeNull();
  });

  it("logs out with local CSRF and returns home even on transport failure", async () => {
    const scrollTo = vi.fn();
    vi.stubGlobal("scrollTo", scrollTo);
    Object.defineProperty(document, "cookie", { configurable: true, value: "arhdesign_csrf=local-csrf" });
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(null, { status: 204 })).mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);
    const { unmount } = render(<AccountShell><span>Проекты</span></AccountShell>);
    fireEvent.click(screen.getAllByRole("button", { name: /Выйти/ })[0]);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/", { scroll: true }));
    expect(scrollTo).toHaveBeenCalledWith({ behavior: "auto", left: 0, top: 0 });
    expect((fetchMock.mock.calls[0][1] as RequestInit).headers).toEqual({ "X-CSRF-Token": "local-csrf" });
    unmount();
    Object.defineProperty(document, "cookie", { configurable: true, value: "" });
    render(<AccountShell><span>Проекты</span></AccountShell>);
    fireEvent.click(screen.getAllByRole("button", { name: /Выйти/ })[0]);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(refresh).not.toHaveBeenCalled();
  });

	it("checks protected nested routes and redirects an expired session", async () => {
		pathname = "/account/projects/new";
		const fetchMock = vi.fn().mockResolvedValueOnce(new Response("{}", { status: 200 })).mockResolvedValueOnce(new Response("{}", { status: 401 })).mockRejectedValueOnce(new Error("offline"));
		vi.stubGlobal("fetch", fetchMock);
		const { unmount } = render(<AccountShell><span>Защищённая форма</span></AccountShell>);
		expect(screen.getByRole("status", { name: "Проверка доступа" })).toBeTruthy();
		expect(await screen.findByText("Защищённая форма")).toBeTruthy();
		unmount();
		pathname = "/account/chats";
		loadCurrentAccount.mockRejectedValueOnce(new ProjectApiErrorMock("Сессия истекла", 401));
		const second = render(<AccountShell><span>Чаты</span></AccountShell>);
		await waitFor(() => expect(replace).toHaveBeenCalledWith("/?auth=login"));
		second.unmount();
		pathname = "/account/profile";
		render(<AccountShell><span>Профиль</span></AccountShell>);
		expect(await screen.findByText("Профиль")).toBeTruthy();
	});

	it("keeps a project route usable after a transport error and marks nested chat navigation active", async () => {
		pathname = "/account/projects/project-1/chat/chat-2";
		vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
		render(<AccountShell><span>Переписка</span></AccountShell>);
		expect(await screen.findByText("Переписка")).toBeTruthy();
		expect(screen.getByRole("link", { name: "Чаты проекта" }).getAttribute("aria-current")).toBe("page");
		expect(screen.getByRole("link", { name: "Сводка" }).getAttribute("aria-current")).toBeNull();
	});

	it("uses the host CSRF cookie while logging out from a global chat route", async () => {
		pathname = "/account/chats";
		Object.defineProperty(document, "cookie", { configurable: true, value: "__Host-arhdesign_csrf=host-token" });
		vi.stubGlobal("scrollTo", vi.fn());
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal("fetch", fetchMock);
		render(<AccountShell><span>Чаты</span></AccountShell>);
		expect(await screen.findByText("Чаты")).toBeTruthy();
		expect(screen.getByRole("link", { name: "Чаты" }).getAttribute("aria-current")).toBe("page");
		fireEvent.click(screen.getAllByRole("button", { name: /Выйти/ })[0]);
		await waitFor(() => expect(fetchMock).toHaveBeenCalled());
		expect((fetchMock.mock.calls[0][1] as RequestInit).headers).toEqual({ "X-CSRF-Token": "host-token" });
	});
});
