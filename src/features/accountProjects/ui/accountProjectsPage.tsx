"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { useAccountNavigation } from "@/widgets/accountShell";
import { Button, Input, Select } from "@/shared/ui";

import { loadProjects, ProjectApiError } from "../api/projects";
import type { ProjectStatus, ProjectView } from "../model/types";

const statusOptions = [
  { label: "Все статусы", value: "all" },
  { label: "Черновики", value: "draft" },
  { label: "В работе", value: "active" },
  { label: "На паузе", value: "paused" },
  { label: "Завершённые", value: "completed" },
] as const;

const statusLabels: Record<ProjectStatus, string> = {
  active: "В работе", archived: "В архиве", completed: "Завершён", draft: "Черновик",
  paused: "На паузе", pending_deletion: "Ожидает удаления",
};

export function AccountProjectsPage() {
  const [projects, setProjects] = useState<ProjectView[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<(typeof statusOptions)[number]["value"]>("all");
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const { setProjectCount } = useAccountNavigation();
	const router = useRouter();

  async function refresh(signal?: AbortSignal) {
    setState("loading");
    setError("");
    try {
      const page = await loadProjects(signal);
      setProjects(page.items);
      setProjectCount(page.items.filter((project) => project.status === "active").length);
      setState("ready");
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      if (reason instanceof ProjectApiError && reason.status === 401) {
		router.replace("/?auth=login");
        return;
      }
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить проекты");
      setState("error");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
	void loadProjects(controller.signal).then((page) => {
		setProjects(page.items);
		setProjectCount(page.items.filter((project) => project.status === "active").length);
		setState("ready");
	}).catch((reason: unknown) => {
		if (reason instanceof DOMException && reason.name === "AbortError") return;
		if (reason instanceof ProjectApiError && reason.status === 401) {
			router.replace("/?auth=login");
			return;
		}
		setError(reason instanceof Error ? reason.message : "Не удалось загрузить проекты");
		setState("error");
	});
    return () => controller.abort();
  }, [router, setProjectCount]);

  const visibleProjects = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("ru");
    return projects.filter((project) => {
      const matchesStatus = status === "all" || project.status === status;
      const matchesQuery = !normalizedQuery || [project.name, project.type, project.address ?? ""].some((value) => value.toLocaleLowerCase("ru").includes(normalizedQuery));
      return matchesStatus && matchesQuery;
    });
  }, [projects, query, status]);

  return (
    <section aria-labelledby="projects-title" data-cy="account-projects">
      <div className="flex flex-col gap-5 border-b border-border pb-7 tablet:flex-row tablet:items-end tablet:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Личный кабинет</p>
          <h1 className="mt-3 font-display text-5xl leading-none tablet:text-6xl" id="projects-title">Проекты</h1>
          <p className="mt-4 max-w-2xl text-secondary">Все объекты, в которых вы участвуете. Откройте проект, чтобы перейти к задачам, финансам и материалам.</p>
        </div>
        <Link className="inline-flex min-h-11 shrink-0 items-center justify-center rounded-full bg-primary px-5 text-sm font-medium text-inverse-text transition-colors hover:bg-action" data-cy="create-project-link" href="/account/projects/new">Создать проект <span aria-hidden="true">→</span></Link>
      </div>

      <div className="mt-7 grid gap-5 tablet:grid-cols-[minmax(0,1fr)_260px]">
        <label className="grid gap-2 text-sm font-medium">Поиск проекта<Input data-cy="project-search" onChange={(event) => setQuery(event.target.value)} placeholder="Название, тип или адрес" type="search" value={query} /></label>
        <Select dataCy="project-status" label="Статус" onValueChange={setStatus} options={statusOptions} value={status} />
      </div>

      {state === "loading" ? <ProjectSkeleton /> : null}
      {state === "error" ? (
        <div className="mt-10 border border-red-700/40 bg-surface p-8" role="alert">
          <h2 className="font-display text-3xl">Проекты не загрузились</h2>
          <p className="mt-3 text-secondary">{error}</p>
          <Button className="mt-6" onClick={() => void refresh()} type="button">Повторить</Button>
        </div>
      ) : null}
      {state === "ready" && projects.length === 0 ? <EmptyProjects /> : null}
      {state === "ready" && projects.length > 0 && visibleProjects.length === 0 ? (
        <div className="mt-10 border border-border bg-surface p-8 text-center"><h2 className="font-display text-3xl">Ничего не найдено</h2><p className="mt-2 text-secondary">Измените запрос или выберите другой статус.</p></div>
      ) : null}
      {state === "ready" && visibleProjects.length > 0 ? (
        <ul className="mt-10 grid gap-5 tablet:grid-cols-2" data-cy="project-grid">
          {visibleProjects.map((project) => <ProjectCard key={project.id} project={project} />)}
        </ul>
      ) : null}
    </section>
  );
}

function ProjectCard({ project }: Readonly<{ project: ProjectView }>) {
  return (
    <li className="group border border-border bg-surface p-6 transition-transform duration-300 hover:-translate-y-1 hover:shadow-surface">
      <Link className="block focus-visible:outline-offset-4" href={`/account/projects/${project.id}`}>
        <div className="flex items-start justify-between gap-4"><span className="rounded-full border border-border px-3 py-1 text-xs font-medium text-secondary">{statusLabels[project.status]}</span><span aria-hidden="true" className="text-2xl text-action transition-transform group-hover:translate-x-1">→</span></div>
        <h2 className="mt-8 font-display text-4xl leading-none">{project.name}</h2>
        <p className="mt-3 text-sm text-secondary">{project.type}{project.address ? ` · ${project.address}` : ""}</p>
        <dl className="mt-8 grid grid-cols-2 gap-4 border-t border-border pt-5 text-sm">
          <div><dt className="text-secondary">Срок проекта</dt><dd className="mt-1 font-medium">{formatRange(project.plannedStartOn, project.plannedFinishOn)}</dd></div>
          <div><dt className="text-secondary">Заказчик</dt><dd className="mt-1 font-medium">{project.customerUserId ? "Назначен" : "Не назначен"}</dd></div>
        </dl>
      </Link>
    </li>
  );
}

function formatRange(start: string | null, finish: string | null) {
  if (!start && !finish) return "Не указан";
  const format = (value: string) => new Intl.DateTimeFormat("ru-RU", { day: "2-digit", month: "short", year: "numeric" }).format(new Date(`${value}T00:00:00`));
  if (start && finish) return `${format(start)} — ${format(finish)}`;
  return format(start ?? finish!);
}

function EmptyProjects() {
  return <div className="mt-10 border border-border bg-surface px-6 py-16 text-center" data-cy="projects-empty"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Первый шаг</p><h2 className="mt-4 font-display text-4xl">У вас пока нет проектов</h2><p className="mx-auto mt-4 max-w-lg text-secondary">Создайте проект: для обычной учётной записи вы автоматически станете его заказчиком, а супер-администратор сможет назначить заказчика позже.</p><Link className="mt-7 inline-flex min-h-11 items-center rounded-full bg-primary px-5 text-sm font-medium text-inverse-text hover:bg-action" href="/account/projects/new">Создать первый проект</Link></div>;
}

function ProjectSkeleton() {
  return <div aria-label="Загрузка проектов" className="mt-10 grid gap-5 tablet:grid-cols-2" role="status">{[0, 1].map((item) => <div className="h-64 animate-pulse border border-border bg-surface" key={item} />)}<span className="sr-only">Загрузка…</span></div>;
}
