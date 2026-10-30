"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import type { ProjectUpcomingItem } from "@/features/accountProjects";
import { Button } from "@/shared/ui/button";
import { Dialog } from "@/shared/ui/dialog";
import { IconButton } from "@/shared/ui/iconButton";

import { loadGlobalCalendar } from "../api/calendar";
import type { GlobalCalendarFeed, GlobalCalendarProject } from "../model/types";

type DisplayMode = "timeline" | "list";
type EventFilter = "all" | ProjectUpcomingItem["kind"];

const statusLabels: Record<GlobalCalendarProject["status"], string> = {
  active: "В работе",
  archived: "В архиве",
  completed: "Завершён",
  draft: "Черновик",
  paused: "На паузе",
  pending_deletion: "На удалении",
};

export function GlobalProjectsCalendarPage() {
  const [anchor, setAnchor] = useState(() => startOfMonth(new Date()));
  const [feed, setFeed] = useState<GlobalCalendarFeed | null>(null);
  const [projectCatalog, setProjectCatalog] = useState<GlobalCalendarProject[]>([]);
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([]);
  const [eventFilter, setEventFilter] = useState<EventFilter>("all");
  const [displayMode, setDisplayMode] = useState<DisplayMode>("timeline");
  const [filterOpen, setFilterOpen] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const range = useMemo(() => ({ start: anchor, end: addMonths(anchor, 4) }), [anchor]);

  useEffect(() => {
    if (typeof window !== "undefined" && window.matchMedia?.("(max-width: 767px)").matches) queueMicrotask(() => setDisplayMode("list"));
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => {
      setState("loading");
      setError("");
      void loadGlobalCalendar(range.start.toISOString(), range.end.toISOString(), selectedProjectIds, controller.signal).then((value) => {
        setFeed(value);
        if (selectedProjectIds.length === 0) setProjectCatalog(value.projects);
        setState("ready");
      }).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить календарь проектов");
        setState("error");
      });
    });
    return () => controller.abort();
  }, [range.end, range.start, reloadKey, selectedProjectIds]);

  const visibleProjects = useMemo(() => (feed?.projects ?? []).map((project) => ({ ...project, items: project.items.filter((item) => eventFilter === "all" || item.kind === eventFilter) })), [eventFilter, feed]);

  return <section aria-labelledby="global-calendar-title" data-cy="global-calendar-page">
    <div className="flex flex-wrap items-end justify-between gap-5">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Рабочее пространство</p>
        <h1 className="mt-2 font-display text-4xl leading-none tablet:text-5xl" id="global-calendar-title">Календарь проектов</h1>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-secondary">Сроки проектов, задачи и встречи в единой временной шкале.</p>
      </div>
      <div className="flex items-center gap-2">
        <IconButton aria-label="Предыдущие четыре месяца" className="border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="calendar-previous-period" onClick={() => setAnchor((value) => addMonths(value, -4))} tooltip="Предыдущий период"><ArrowIcon direction="left" /></IconButton>
        <Button onClick={() => setAnchor(startOfMonth(new Date()))} variant="secondary">Сегодня</Button>
        <IconButton aria-label="Следующие четыре месяца" className="border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="calendar-next-period" onClick={() => setAnchor((value) => addMonths(value, 4))} tooltip="Следующий период"><ArrowIcon direction="right" /></IconButton>
      </div>
    </div>

    <div className="mt-8 flex flex-wrap items-center justify-between gap-3 border border-border bg-surface p-3">
      <div aria-label="Тип событий" className="flex min-w-0 items-center gap-1 overflow-x-auto">
        {([['all', 'Все'], ['task', 'Задачи'], ['meeting', 'Встречи']] as const).map(([value, label]) => <button aria-pressed={eventFilter === value} className="min-h-10 shrink-0 border border-border px-4 text-xs font-semibold text-secondary hover:border-action aria-pressed:border-action aria-pressed:bg-action/15 aria-pressed:text-primary" data-cy={`global-calendar-filter-${value}`} key={value} onClick={() => setEventFilter(value)} type="button">{label}</button>)}
      </div>
      <div className="flex items-center gap-2">
        <span className="hidden text-sm capitalize text-secondary tablet:inline" data-cy="calendar-period-label">{formatPeriod(range.start, range.end)}</span>
        <span className="relative inline-flex">
          <IconButton aria-label="Фильтр проектов" className="border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="global-calendar-project-filter" onClick={() => setFilterOpen(true)} tooltip="Фильтр проектов"><FilterIcon /></IconButton>
          {selectedProjectIds.length > 0 ? <span aria-label={`Применено фильтров проектов: ${selectedProjectIds.length}`} className="absolute -right-1 -top-1 min-w-5 rounded-full bg-action px-1 text-center text-[11px] font-semibold leading-5 text-inverse-text">{selectedProjectIds.length}</span> : null}
        </span>
        <IconButton aria-label={displayMode === "timeline" ? "Показать календарь списком" : "Показать временную шкалу"} className="border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="global-calendar-display-toggle" onClick={() => setDisplayMode((value) => value === "timeline" ? "list" : "timeline")} tooltip={displayMode === "timeline" ? "Показать списком" : "Показать шкалой"}>{displayMode === "timeline" ? <ListIcon /> : <TimelineIcon />}</IconButton>
      </div>
    </div>

    {state === "loading" ? <CalendarSkeleton /> : null}
    {state === "error" ? <div className="mt-5 border border-red-700/40 bg-surface p-5" role="alert"><p className="text-sm text-red-700">{error}</p><Button className="mt-4" onClick={() => setReloadKey((value) => value + 1)} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" && feed ? <>
      {feed.hasMoreProjects || feed.truncatedEvents ? <p className="mt-4 border border-action/40 bg-action/10 px-4 py-3 text-sm" role="status">Показана часть календаря. Уточните период или выберите проекты в фильтре.</p> : null}
      {visibleProjects.length === 0 ? <EmptyCalendar filtered={selectedProjectIds.length > 0} onReset={() => setSelectedProjectIds([])} /> : displayMode === "timeline" ? <ProjectTimeline projects={visibleProjects} range={range} /> : <ProjectAgenda projects={visibleProjects} />}
    </> : null}

    <Dialog isOpen={filterOpen} label="Фильтр проектов" onClose={() => setFilterOpen(false)}><ProjectFilter catalog={projectCatalog} initialValue={selectedProjectIds} onApply={(value) => { setSelectedProjectIds(value); setFilterOpen(false); }} onCancel={() => setFilterOpen(false)} /></Dialog>
  </section>;
}

function ProjectTimeline({ projects, range }: Readonly<{ projects: GlobalCalendarProject[]; range: { start: Date; end: Date } }>) {
  const weeks = weeksBetween(range.start, range.end);
  return <div className="mt-5 overflow-x-auto border border-border bg-surface" data-cy="global-calendar-timeline">
    <div className="min-w-[1120px]">
      <div className="grid grid-cols-[260px_minmax(0,1fr)] border-b border-border bg-page/70">
        <div className="sticky left-0 z-20 border-r border-border bg-page px-5 py-4 text-xs font-semibold uppercase tracking-[0.1em] text-secondary">Проекты</div>
        <div className="grid" style={{ gridTemplateColumns: `repeat(${weeks.length}, minmax(52px, 1fr))` }}>{weeks.map((week) => <div className="border-r border-border px-2 py-3 text-center last:border-r-0" key={week.start.toISOString()}><span className="block text-[11px] font-semibold uppercase text-secondary">{formatMonthShort(week.start)}</span><span className="mt-1 block text-xs tabular-nums">{formatWeekRange(week.start, week.end)}</span></div>)}</div>
      </div>
      {projects.map((project) => <TimelineLane key={project.id} project={project} range={range} weeks={weeks} />)}
    </div>
  </div>;
}

function TimelineLane({ project, range, weeks }: Readonly<{ project: GlobalCalendarProject; range: { start: Date; end: Date }; weeks: { start: Date; end: Date }[] }>) {
  const bar = plannedBar(project, range);
  return <div className="grid min-h-28 grid-cols-[260px_minmax(0,1fr)] border-b border-border last:border-b-0" data-cy={`calendar-project-${project.id}`}>
    <div className="sticky left-0 z-20 border-r border-border bg-surface p-5">
      <Link className="font-medium underline decoration-action/50 underline-offset-4 hover:text-action" href={`/account/projects/${encodeURIComponent(project.id)}`}>{project.name}</Link>
      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-secondary"><span>{statusLabels[project.status]}</span><span aria-hidden="true">·</span><span>{formatProjectDates(project)}</span></div>
    </div>
    <div className="relative min-h-28 overflow-hidden">
      <div aria-hidden="true" className="absolute inset-0 grid" style={{ gridTemplateColumns: `repeat(${weeks.length}, minmax(52px, 1fr))` }}>{weeks.map((week) => <span className="border-r border-border last:border-r-0" key={week.start.toISOString()} />)}</div>
      {bar ? <Link aria-label={`Проект ${project.name}: ${formatProjectDates(project)}`} className="absolute top-4 h-7 min-w-2 rounded-full bg-action/25 ring-1 ring-inset ring-action/50 transition-colors hover:bg-action/40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-action" href={`/account/projects/${encodeURIComponent(project.id)}`} style={{ left: `${bar.left}%`, width: `${bar.width}%` }} /> : null}
      {project.items.map((item, index) => <TimelineEvent item={item} key={`${item.kind}-${item.id}`} project={project} range={range} row={index % 2} />)}
      {!bar && project.items.length === 0 ? <span className="absolute left-5 top-1/2 -translate-y-1/2 text-xs text-secondary">Сроки и события пока не добавлены</span> : null}
    </div>
  </div>;
}

function TimelineEvent({ item, project, range, row }: Readonly<{ item: ProjectUpcomingItem; project: GlobalCalendarProject; range: { start: Date; end: Date }; row: number }>) {
  const position = percentInRange(new Date(item.effectiveAt), range);
  if (position < 0 || position > 100) return null;
  return <Link aria-label={`${item.kind === "task" ? "Задача" : "Встреча"} ${item.title}, проект ${project.name}, ${formatDateTime(item.effectiveAt)}`} className={`absolute z-10 flex max-w-40 -translate-x-1/2 items-center gap-1.5 rounded-sm border px-2 py-1 text-[11px] shadow-sm transition-transform hover:z-20 hover:scale-105 focus-visible:z-20 focus-visible:outline focus-visible:outline-2 focus-visible:outline-action ${item.kind === "task" ? "border-action bg-surface text-primary" : "border-primary bg-primary text-inverse-text"}`} data-cy={`global-calendar-event-${item.id}`} href={eventHref(item)} style={{ left: `${position}%`, top: `${58 + row * 28}px` }} title={item.title}><span aria-hidden="true">{item.kind === "task" ? "✓" : "●"}</span><span className="truncate">{item.title}</span></Link>;
}

function ProjectAgenda({ projects }: Readonly<{ projects: GlobalCalendarProject[] }>) {
  return <div className="mt-5 grid gap-4" data-cy="global-calendar-list">{projects.map((project) => <section className="border border-border bg-surface" key={project.id}><header className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4"><div><Link className="font-medium underline decoration-action/50 underline-offset-4" href={`/account/projects/${encodeURIComponent(project.id)}`}>{project.name}</Link><p className="mt-1 text-xs text-secondary">{formatProjectDates(project)}</p></div><span className="text-xs text-secondary">{statusLabels[project.status]}</span></header>{project.items.length === 0 ? <p className="px-5 py-6 text-sm text-secondary">В выбранном периоде задач и встреч нет.</p> : <ul>{project.items.map((item) => <li className="border-b border-border last:border-b-0" key={`${item.kind}-${item.id}`}><Link className="grid gap-2 px-5 py-4 hover:bg-page tablet:grid-cols-[150px_100px_minmax(0,1fr)]" href={eventHref(item)}><time className="text-sm tabular-nums" dateTime={item.effectiveAt}>{formatDateTime(item.effectiveAt)}</time><span className="text-xs font-semibold uppercase tracking-[0.08em] text-action">{item.kind === "task" ? "Задача" : "Встреча"}</span><span className="text-sm">{item.title}</span></Link></li>)}</ul>}</section>)}</div>;
}

function ProjectFilter({ catalog, initialValue, onApply, onCancel }: Readonly<{ catalog: GlobalCalendarProject[]; initialValue: string[]; onApply: (value: string[]) => void; onCancel: () => void }>) {
  const [selected, setSelected] = useState(() => new Set(initialValue));
  function toggle(projectId: string) { setSelected((current) => { const next = new Set(current); if (next.has(projectId)) next.delete(projectId); else next.add(projectId); return next; }); }
  return <form data-cy="global-calendar-project-filter-form" onSubmit={(event) => { event.preventDefault(); onApply([...selected]); }}>
    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Отображение</p><h2 className="mt-3 pr-10 font-display text-4xl">Выберите проекты</h2><p className="mt-3 text-sm text-secondary">Без выбора показываются все доступные проекты.</p>
    <div className="mt-7 max-h-72 overflow-y-auto border border-border">{catalog.length === 0 ? <p className="p-4 text-sm text-secondary">Доступных проектов пока нет.</p> : catalog.map((project) => <label className="flex min-h-12 cursor-pointer items-center gap-3 border-b border-border px-4 last:border-b-0 hover:bg-page" key={project.id}><input checked={selected.has(project.id)} className="size-4 accent-[var(--color-action)]" onChange={() => toggle(project.id)} type="checkbox" /><span className="min-w-0 flex-1 truncate text-sm">{project.name}</span><span className="text-xs text-secondary">{statusLabels[project.status]}</span></label>)}</div>
    <div className="mt-7 flex flex-wrap gap-3"><Button type="submit">Применить</Button><Button onClick={() => { setSelected(new Set()); onApply([]); }} type="button" variant="secondary">Сбросить</Button><Button onClick={onCancel} type="button" variant="text">Отмена</Button></div>
  </form>;
}

function EmptyCalendar({ filtered, onReset }: Readonly<{ filtered: boolean; onReset: () => void }>) { return <div className="mt-5 border border-border bg-surface px-6 py-16 text-center" data-cy="global-calendar-empty"><h2 className="font-display text-3xl">{filtered ? "Нет проектов по фильтру" : "Проектов пока нет"}</h2><p className="mx-auto mt-3 max-w-lg text-sm leading-6 text-secondary">{filtered ? "Сбросьте фильтр, чтобы снова увидеть все доступные проекты." : "Создайте первый проект — его сроки, задачи и встречи появятся на этой странице."}</p>{filtered ? <Button className="mt-6" onClick={onReset} variant="secondary">Сбросить фильтр</Button> : <Link className="mt-6 inline-flex min-h-11 items-center rounded-full bg-primary px-5 text-sm font-medium text-inverse-text hover:bg-action" href="/account/projects/new">Создать проект</Link>}</div>; }
function CalendarSkeleton() { return <div aria-label="Загрузка календаря проектов" className="mt-5 h-[520px] animate-pulse border border-border bg-surface" role="status"><span className="sr-only">Загружаем календарь…</span></div>; }

function eventHref(item: ProjectUpcomingItem) { return item.kind === "task" ? `/account/projects/${encodeURIComponent(item.projectId)}/tasks?taskId=${encodeURIComponent(item.id)}` : `/account/projects/${encodeURIComponent(item.projectId)}/calendar?meetingId=${encodeURIComponent(item.id)}`; }
function plannedBar(project: GlobalCalendarProject, range: { start: Date; end: Date }) { if (!project.plannedStartOn && !project.plannedFinishOn) return null; const start = project.plannedStartOn ? parseDate(project.plannedStartOn) : range.start; const end = project.plannedFinishOn ? addDays(parseDate(project.plannedFinishOn), 1) : range.end; const clippedStart = new Date(Math.max(start.getTime(), range.start.getTime())); const clippedEnd = new Date(Math.min(end.getTime(), range.end.getTime())); if (clippedEnd <= clippedStart) return null; const left = percentInRange(clippedStart, range); return { left, width: Math.max(0.8, percentInRange(clippedEnd, range) - left) }; }
function percentInRange(value: Date, range: { start: Date; end: Date }) { return ((value.getTime() - range.start.getTime()) / (range.end.getTime() - range.start.getTime())) * 100; }
function startOfMonth(value: Date) { return new Date(value.getFullYear(), value.getMonth(), 1); }
function addMonths(value: Date, count: number) { return new Date(value.getFullYear(), value.getMonth() + count, 1); }
function addDays(value: Date, count: number) { const next = new Date(value); next.setDate(next.getDate() + count); return next; }
function startOfWeek(value: Date) { const next = new Date(value); const weekday = next.getDay() || 7; next.setDate(next.getDate() - weekday + 1); return next; }
function weeksBetween(start: Date, end: Date) { const weeks: { start: Date; end: Date }[] = []; for (let value = startOfWeek(start); value < end; value = addDays(value, 7)) weeks.push({ start: value, end: addDays(value, 6) }); return weeks; }
function parseDate(value: string) { const [year, month, day] = value.split("-").map(Number); return new Date(year, month - 1, day); }
function formatMonthShort(value: Date) { return new Intl.DateTimeFormat("ru-RU", { month: "short" }).format(value).replace(".", ""); }
function formatWeekRange(start: Date, end: Date) { return `${String(start.getDate()).padStart(2, "0")}–${String(end.getDate()).padStart(2, "0")}`; }
function formatPeriod(start: Date, end: Date) { return `${new Intl.DateTimeFormat("ru-RU", { month: "long", year: "numeric" }).format(start)} — ${new Intl.DateTimeFormat("ru-RU", { month: "long", year: "numeric" }).format(addDays(end, -1))}`; }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { day: "numeric", hour: "2-digit", minute: "2-digit", month: "long" }).format(new Date(value)); }
function formatProjectDates(project: GlobalCalendarProject) { if (!project.plannedStartOn && !project.plannedFinishOn) return "Сроки не заданы"; const format = (value: string) => new Intl.DateTimeFormat("ru-RU", { day: "2-digit", month: "short", year: "numeric" }).format(parseDate(value)); if (project.plannedStartOn && project.plannedFinishOn) return `${format(project.plannedStartOn)} — ${format(project.plannedFinishOn)}`; return project.plannedStartOn ? `Старт ${format(project.plannedStartOn)}` : `Сдача ${format(project.plannedFinishOn!)}`; }

function ArrowIcon({ direction }: Readonly<{ direction: "left" | "right" }>) { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d={direction === "left" ? "M10 3 5 8l5 5" : "m6 3 5 5-5 5"} stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" /></svg>; }
function FilterIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="M2 3h12M4 8h8M6 13h4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4" /></svg>; }
function ListIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="M5 4h9M5 8h9M5 12h9M2 4h.01M2 8h.01M2 12h.01" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4" /></svg>; }
function TimelineIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="M2 3v10h12M5 6h6M7 9h5M4 12h4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4" /></svg>; }
