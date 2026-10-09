"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { ThemeToggle } from "@/features/themeToggle";
import { FrontendErrorReporter, TechnicalSupportWidget } from "@/features/technicalSupport";
import { AccountNotificationCenter } from "@/features/accountNotifications";
import { loadCurrentAccount, ProjectApiError } from "@/features/accountProjects/api/projects";

type NavigationContextValue = { setProjectCount: (count: number) => void };
const NavigationContext = createContext<NavigationContextValue | null>(null);

const navigation = [
  { href: "/account", label: "Проекты", icon: ProjectsIcon },
  { href: "/account/calendar", label: "Календарь", icon: CalendarIcon },
  { href: "/account/chats", label: "Чаты", icon: ChatsIcon },
	{ href: "/account/users", label: "Пользователи", icon: UsersIcon, globalOnly: true },
] as const;

const projectNavigation = [
  { label: "Сводка", segment: "" },
  { label: "Задачи", segment: "tasks" },
  { label: "Календарь проекта", segment: "calendar" },
  { label: "Материалы", segment: "materials" },
  { label: "Финансы проекта", segment: "finances" },
  { label: "Документы", segment: "documents" },
  { label: "Чаты проекта", segment: "chat" },
] as const;

export function AccountShell({ children }: Readonly<{ children: ReactNode }>) {
  const pathname = usePathname() ?? "";
	const router = useRouter();
  const [projectCount, setProjectCount] = useState(0);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
	const [isGlobalAdministrator, setIsGlobalAdministrator] = useState(false);
	const [accountAuthorized, setAccountAuthorized] = useState(false);
	const [authorizedPath, setAuthorizedPath] = useState("");
	const projectRoute = getProjectWorkspaceRoute(pathname);
	const projectId = projectRoute?.projectId ?? null;
	const isAuthorized = accountAuthorized && (!projectRoute || authorizedPath === pathname);

	useEffect(() => {
		const controller = new AbortController();
		void loadCurrentAccount(controller.signal).then((account) => {
			setIsGlobalAdministrator(account.globalRole === "super_admin" || account.globalRole === "technical_admin");
			setAccountAuthorized(true);
		}).catch((reason: unknown) => {
			if (reason instanceof DOMException && reason.name === "AbortError") return;
			if (reason instanceof ProjectApiError && reason.status === 401) router.replace("/?auth=login");
		});
		return () => controller.abort();
	}, [router]);

	useEffect(() => {
		if (!projectId) return;
		const controller = new AbortController();
		const accessUrl = projectId ? `/api/v1/projects/${encodeURIComponent(projectId)}` : "/api/v1/projects?pageSize=1";
		void fetch(accessUrl, { credentials: "same-origin", signal: controller.signal }).then(async (response) => {
			if (response.status === 401) {
				router.replace("/?auth=login");
				return;
			}
			setAuthorizedPath(pathname);
		}).catch((reason: unknown) => {
			if (!(reason instanceof DOMException && reason.name === "AbortError")) setAuthorizedPath(pathname);
		});
		return () => controller.abort();
	}, [pathname, projectId, router]);

  async function logout() {
    setIsLoggingOut(true);
    const csrf = document.cookie.split("; ").find((value) => value.startsWith("arhdesign_csrf=") || value.startsWith("__Host-arhdesign_csrf="));
    const token = csrf ? decodeURIComponent(csrf.slice(csrf.indexOf("=") + 1)) : "";
    try {
      await fetch("/api/v1/auth/logout", { credentials: "same-origin", headers: { "X-CSRF-Token": token }, method: "POST" });
	} catch {
		// Local session is left behind only until it expires; navigation must
		// remain available when the API is temporarily unreachable.
    } finally {
		window.dispatchEvent(new CustomEvent("arhdesign:session-changed", { detail: { authenticated: false } }));
      window.scrollTo({ behavior: "auto", left: 0, top: 0 });
		sessionStorage.setItem("arhdesign:scroll-home-top", "1");
		router.replace("/", { scroll: true });
    }
  }
	const updateProjectCount = useCallback((count: number) => setProjectCount(count), []);
	const navigationContext = useMemo(() => ({ setProjectCount: updateProjectCount }), [updateProjectCount]);

  return (
    <NavigationContext.Provider value={navigationContext}>
      <FrontendErrorReporter />
      <div className="min-h-screen bg-page tablet:grid tablet:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="border-b border-border bg-surface tablet:sticky tablet:top-0 tablet:flex tablet:h-screen tablet:flex-col tablet:overflow-hidden tablet:border-b-0 tablet:border-r" aria-label="Навигация личного кабинета">
          <div className="flex min-h-[72px] items-center gap-3 border-b border-border px-5">
            <Link aria-label="Личный кабинет — проекты" className="mr-auto inline-flex items-center gap-3 text-sm font-semibold tracking-[0.08em]" href="/account"><Image alt="" aria-hidden="true" className="size-8 rounded-md" height={32} src="/icon.svg" width={32} /><span className="hidden tablet:inline">ПОЛИСМАКОВА С.</span></Link>
            <div className="tablet:hidden"><ThemeToggle /></div>
          </div>
		  {projectRoute ? <ProjectNavigation pathname={pathname} projectId={projectRoute.projectId} /> : <GlobalNavigation isGlobalAdministrator={isGlobalAdministrator} pathname={pathname} projectCount={projectCount} />}
          <div className="mt-auto hidden border-t border-border p-4 tablet:block">
            <button className="flex min-h-11 w-full items-center gap-3 px-4 text-left text-sm text-secondary hover:bg-page hover:text-primary" disabled={isLoggingOut} onClick={() => void logout()} type="button"><ExitIcon />{isLoggingOut ? "Выходим…" : "Выйти"}</button>
          </div>
        </aside>
        <div className="min-w-0">
          <header className="flex min-h-[72px] items-center justify-end gap-3 border-b border-border px-5 tablet:px-8"><span className="mr-auto text-sm text-secondary">Рабочее пространство</span><div className="hidden tablet:block"><ThemeToggle /></div><AccountNotificationCenter /><button aria-label="Выйти из личного кабинета" className="inline-flex min-h-11 items-center gap-2 rounded-full border border-border px-4 text-sm hover:bg-surface tablet:hidden" disabled={isLoggingOut} onClick={() => void logout()} type="button"><ExitIcon /> Выйти</button><Link aria-label="Настройки профиля" className="inline-flex size-10 items-center justify-center rounded-full bg-action font-display text-lg text-inverse-text" href="/account/profile">С</Link></header>
		  <main className="px-5 py-9 tablet:px-8 tablet:py-12 desktop:px-12"><div className="mx-auto max-w-[1280px]">{isAuthorized ? children : <div aria-label="Проверка доступа" className="h-72 animate-pulse border border-border bg-surface" role="status"><span className="sr-only">Проверяем доступ…</span></div>}</div></main>
        </div>
      </div>
      {isAuthorized ? <TechnicalSupportWidget /> : null}
    </NavigationContext.Provider>
  );
}

function GlobalNavigation({ isGlobalAdministrator, pathname, projectCount }: Readonly<{ isGlobalAdministrator: boolean; pathname: string; projectCount: number }>) {
  return <nav aria-label="Основные разделы" className="flex gap-2 overflow-x-auto p-3 tablet:min-h-0 tablet:flex-1 tablet:flex-col tablet:gap-1 tablet:overflow-y-auto tablet:p-4">
	{navigation.map((item) => {
	  if ("globalOnly" in item && item.globalOnly && !isGlobalAdministrator) return null;
	  const { href, label, icon: Icon } = item;
      const isActive = href === "/account" ? pathname === href || pathname === "/account/projects/new" : pathname.startsWith(href);
      return <Link aria-current={isActive ? "page" : undefined} className={`flex min-h-11 shrink-0 items-center gap-3 rounded-sm px-4 text-sm font-medium transition-colors ${isActive ? "bg-primary text-inverse-text" : "text-secondary hover:bg-page hover:text-primary"}`} data-cy={`account-nav-${label.toLocaleLowerCase("ru")}`} href={href} key={href}><Icon /><span>{label}</span>{label === "Проекты" && projectCount > 0 ? <span className={`ml-auto min-w-6 rounded-full px-2 py-0.5 text-center text-xs ${isActive ? "bg-page text-primary" : "bg-action text-inverse-text"}`}>{projectCount}</span> : null}</Link>;
    })}
  </nav>;
}

function ProjectNavigation({ pathname, projectId }: Readonly<{ pathname: string; projectId: string }>) {
  const baseHref = `/account/projects/${encodeURIComponent(projectId)}`;
  return <div className="min-w-0 px-4 pb-4 pt-4 tablet:flex tablet:min-h-0 tablet:flex-1 tablet:flex-col tablet:overflow-hidden">
    <Link className="flex min-h-[38px] items-center rounded-md bg-action px-3 text-xs font-semibold text-inverse-text transition-colors hover:bg-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action" data-cy="back-to-projects" href="/account"><span aria-hidden="true">←</span><span className="ml-2">Назад к проектам</span></Link>
    <nav aria-label="Разделы проекта" className="mt-2 grid grid-cols-2 gap-1 min-[40rem]:grid-cols-3 tablet:flex tablet:min-h-0 tablet:flex-1 tablet:flex-col tablet:overflow-y-auto tablet:pr-1">
      {projectNavigation.map(({ label, segment }) => {
        const href = segment ? `${baseHref}/${segment}` : baseHref;
        const isActive = segment ? pathname === href || pathname.startsWith(`${href}/`) : pathname === baseHref;
        const isAvailable = segment === "" || segment === "tasks" || segment === "calendar" || segment === "materials" || segment === "finances" || segment === "documents" || segment === "chat";
        const classes = `flex min-h-[38px] min-w-0 items-center px-3 text-xs transition-colors tablet:min-h-[42px] tablet:text-sm ${isActive ? "bg-action/30 font-semibold text-primary" : "text-secondary"}`;
        return isAvailable ? <Link aria-current={isActive ? "page" : undefined} className={`${classes} hover:bg-page hover:text-primary`} data-cy={`project-nav-${segment || "summary"}`} href={href} key={label}>{label}</Link> : <span aria-disabled="true" className={`${classes} cursor-not-allowed opacity-75`} data-cy={`project-nav-${segment}`} key={label} title="Раздел будет доступен в одном из следующих релизов">{label}</span>;
      })}
    </nav>
  </div>;
}

function getProjectWorkspaceRoute(pathname: string) {
  const match = /^\/account\/projects\/([^/]+)(?:\/(.*))?$/.exec(pathname);
  if (!match || match[1] === "new") return null;
  return { projectId: decodeURIComponent(match[1]), segment: match[2] ?? "" };
}

export function useAccountNavigation() {
  const context = useContext(NavigationContext);
  if (!context) throw new Error("useAccountNavigation must be used inside AccountShell");
  return context;
}

function ProjectsIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><path d="M3 5.5h5l1.2 1.5H17v8.5H3z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.4" /></svg>; }
function CalendarIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><rect height="13" rx="1" stroke="currentColor" strokeWidth="1.4" width="14" x="3" y="4"/><path d="M3 8h14M7 2.5v3M13 2.5v3" stroke="currentColor" strokeWidth="1.4"/></svg>; }
function ChatsIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><path d="M3 4.5h14v10H8l-4 3v-3H3z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.4"/></svg>; }
function UsersIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><circle cx="7" cy="7" r="3" stroke="currentColor" strokeWidth="1.4"/><path d="M2.5 16c.4-3 2-4.5 4.5-4.5s4.1 1.5 4.5 4.5M12 5.2a3 3 0 0 1 0 5.6M13 12c2.5 0 4 1.3 4.5 4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4"/></svg>; }
function ExitIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="M6 3H3v10h3M10 5l3 3-3 3M5 8h8" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.4"/></svg>; }
