"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { Button } from "@/shared/ui/button";
import { Dialog } from "@/shared/ui/dialog";
import { IconButton } from "@/shared/ui/iconButton";
import { Select } from "@/shared/ui/select";

import { loadProject, loadProjectCalendar } from "../api/projects";
import type { ProjectUpcomingItem, ProjectView } from "../model/types";

type CalendarView = "month" | "week";
type DisplayMode = "calendar" | "list";
type EventFilter = "all" | ProjectUpcomingItem["kind"];

const weekdays = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"];

export function ProjectCalendarPage({ projectId }: Readonly<{ projectId: string }>) {
  const [project, setProject] = useState<ProjectView | null>(null);
  const [items, setItems] = useState<ProjectUpcomingItem[]>([]);
  const [view, setView] = useState<CalendarView>("month");
  const [displayMode, setDisplayMode] = useState<DisplayMode>("calendar");
  const [filter, setFilter] = useState<EventFilter>("all");
  const [anchor, setAnchor] = useState(() => startOfDay(new Date()));
  const [selected, setSelected] = useState<ProjectUpcomingItem | null>(null);
  const [periodOpen, setPeriodOpen] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const range = useMemo(() => calendarRange(anchor, view), [anchor, view]);

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => {
      setState("loading"); setError("");
      void Promise.all([loadProject(projectId, controller.signal), loadProjectCalendar(projectId, range.start.toISOString(), range.end.toISOString(), controller.signal)]).then(([projectValue, feed]) => {
        setProject(projectValue); setItems(feed.items); setState("ready");
      }).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить календарь проекта"); setState("error");
      });
    });
    return () => controller.abort();
  }, [projectId, range.end, range.start, reloadKey]);

  const visibleItems = useMemo(() => items.filter((item) => filter === "all" || item.kind === filter), [filter, items]);
  const periodLabel = formatMonthYear(anchor);

  return <section aria-label="Календарь проекта" data-cy="project-calendar-page">
    <nav aria-label="Хлебные крошки" className="flex flex-wrap items-center gap-2 text-xs text-secondary"><Link className="hover:text-primary" href="/account">Проекты</Link><span aria-hidden="true">/</span><Link className="truncate hover:text-primary" href={`/account/projects/${encodeURIComponent(projectId)}`}>{project?.name ?? "Проект"}</Link><span aria-hidden="true">/</span><span aria-current="page">Календарь</span></nav>
    <div aria-label="Период календаря" className="mt-7 flex w-fit max-w-full items-center gap-1 overflow-x-auto rounded-full border border-border p-1">{([['month', 'Месяц'], ['week', 'Неделя']] as const).map(([value, label]) => <button aria-pressed={view === value} className="min-h-9 shrink-0 rounded-full px-4 text-xs font-semibold text-secondary aria-pressed:bg-action aria-pressed:text-inverse-text" data-cy={`calendar-view-${value}`} key={value} onClick={() => setView(value)} type="button">{label}</button>)}<button aria-label={`Выбор месяца и года: ${periodLabel}`} className="flex min-h-9 shrink-0 items-center gap-2 rounded-full border-l border-border px-4 text-xs font-semibold capitalize text-secondary hover:text-primary" data-cy="calendar-period-picker" onClick={() => setPeriodOpen(true)} type="button"><CalendarIcon />{periodLabel}</button></div>

    <div className="mt-5 flex items-center justify-between gap-3 border border-border bg-surface p-3">
      <div aria-label="Тип событий" className="flex min-w-0 items-center gap-1 overflow-x-auto">{([['all', 'Все'], ['task', 'Задачи'], ['meeting', 'Встречи']] as const).map(([value, label]) => <button aria-pressed={filter === value} className="min-h-10 shrink-0 border border-border px-4 text-xs font-semibold text-secondary hover:border-action aria-pressed:border-action aria-pressed:bg-action/15 aria-pressed:text-primary" key={value} onClick={() => setFilter(value)} type="button">{label}</button>)}</div>
      <IconButton aria-label={displayMode === "calendar" ? "Показать события списком" : "Показать события в календаре"} className="size-10 shrink-0 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="calendar-display-toggle" onClick={() => setDisplayMode((current) => current === "calendar" ? "list" : "calendar")} tooltip={displayMode === "calendar" ? "Показать списком" : "Показать календарём"} tooltipAlign="end">{displayMode === "calendar" ? <ListIcon /> : <GridIcon />}</IconButton>
    </div>

    {state === "loading" ? <div aria-label="Загрузка календаря" className="mt-5 h-[520px] animate-pulse border border-border bg-surface" role="status" /> : null}
    {state === "error" ? <div className="mt-5 border border-red-700/40 bg-surface p-5" role="alert"><p className="text-sm text-red-700">{error}</p><Button className="mt-4" onClick={() => setReloadKey((value) => value + 1)} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" ? displayMode === "list" ? <CalendarList items={visibleItems} onSelect={setSelected} /> : <CalendarGrid anchor={anchor} items={visibleItems} onSelect={setSelected} range={range} view={view} /> : null}
    <Dialog isOpen={periodOpen} label="Выбор месяца и года" onClose={() => setPeriodOpen(false)}><PeriodPicker anchor={anchor} onApply={(value) => { setAnchor(value); setPeriodOpen(false); }} onCancel={() => setPeriodOpen(false)} /></Dialog>
    <Dialog isOpen={selected !== null} label="Событие календаря" onClose={() => setSelected(null)}>{selected ? <EventDetails item={selected} /> : null}</Dialog>
  </section>;
}

function CalendarGrid({ anchor, items, onSelect, range, view }: Readonly<{ anchor: Date; items: ProjectUpcomingItem[]; onSelect: (item: ProjectUpcomingItem) => void; range: { start: Date; end: Date }; view: Exclude<CalendarView, "list"> }>) {
  const days = datesBetween(range.start, range.end);
  return <div className="mt-5 overflow-x-auto border border-border bg-surface" data-cy="calendar-grid"><div className="min-w-[780px]"><div className="grid grid-cols-7 border-b border-border">{weekdays.map((day) => <div className="px-3 py-3 text-xs font-semibold uppercase tracking-[0.1em] text-secondary" key={day}>{day}</div>)}</div><div className="grid grid-cols-7">{days.map((day) => { const dayItems = items.filter((item) => sameDay(new Date(item.effectiveAt), day)); const outside = view === "month" && day.getMonth() !== anchor.getMonth(); return <div className={`min-h-32 border-b border-r border-border p-2 ${outside ? "bg-page/60 text-secondary" : "bg-surface"}`} data-date={dateKey(day)} key={dateKey(day)}><time className={`inline-flex size-7 items-center justify-center rounded-full text-xs ${sameDay(day, new Date()) ? "bg-action font-semibold text-inverse-text" : ""}`} dateTime={dateKey(day)}>{day.getDate()}</time><div className="mt-1 grid gap-1">{dayItems.map((item) => <EventButton item={item} key={`${item.kind}-${item.id}`} onSelect={onSelect} />)}</div></div>; })}</div></div></div>;
}

function CalendarList({ items, onSelect }: Readonly<{ items: ProjectUpcomingItem[]; onSelect: (item: ProjectUpcomingItem) => void }>) {
  const grouped = items.reduce<Record<string, ProjectUpcomingItem[]>>((result, item) => { const key = dateKey(new Date(item.effectiveAt)); (result[key] ??= []).push(item); return result; }, {});
  const groups = Object.entries(grouped).sort(([left], [right]) => left.localeCompare(right));
  if (groups.length === 0) return <EmptyCalendar />;
  return <div className="mt-5 border border-border bg-surface" data-cy="calendar-list">{groups.map(([date, events]) => <section className="grid border-b border-border last:border-b-0 tablet:grid-cols-[180px_minmax(0,1fr)]" key={date}><h2 className="p-5 text-sm font-semibold capitalize">{formatDay(new Date(`${date}T12:00:00`))}</h2><div className="grid gap-2 border-t border-border p-4 tablet:border-l tablet:border-t-0">{events?.map((item) => <EventButton item={item} key={`${item.kind}-${item.id}`} onSelect={onSelect} />)}</div></section>)}</div>;
}

function EventButton({ item, onSelect }: Readonly<{ item: ProjectUpcomingItem; onSelect: (item: ProjectUpcomingItem) => void }>) { return <button aria-label={`${item.kind === "task" ? "Задача" : "Встреча"}: ${item.title}, ${formatTime(item.effectiveAt)}`} className={`flex min-w-0 items-center gap-2 border-l-4 px-2 py-1.5 text-left text-xs hover:bg-page focus-visible:outline focus-visible:outline-2 focus-visible:outline-action ${item.kind === "task" ? "border-action bg-action/10" : "border-primary bg-page"}`} onClick={() => onSelect(item)} type="button"><span className="shrink-0 font-semibold">{formatTime(item.effectiveAt)}</span><span className="truncate">{item.title}</span></button>; }
function EventDetails({ item }: Readonly<{ item: ProjectUpcomingItem }>) { return <div data-cy="calendar-event-details"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">{item.kind === "task" ? "Задача" : "Встреча"}</p><h2 className="mt-3 pr-10 font-display text-4xl">{item.title}</h2><dl className="mt-7 grid gap-5 text-sm"><div><dt className="text-xs uppercase tracking-[0.1em] text-secondary">Начало / срок</dt><dd className="mt-1">{formatDateTime(item.effectiveAt)}</dd></div>{item.endsAt ? <div><dt className="text-xs uppercase tracking-[0.1em] text-secondary">Окончание</dt><dd className="mt-1">{formatDateTime(item.endsAt)}</dd></div> : null}{item.location ? <div><dt className="text-xs uppercase tracking-[0.1em] text-secondary">Место</dt><dd className="mt-1">{item.location}</dd></div> : null}{item.description ? <div><dt className="text-xs uppercase tracking-[0.1em] text-secondary">Описание</dt><dd className="mt-1 leading-6">{item.description}</dd></div> : null}</dl></div>; }
function EmptyCalendar() { return <div className="mt-5 border border-border bg-surface px-5 py-14 text-center"><h2 className="text-lg font-medium">В этом периоде событий нет</h2><p className="mt-2 text-sm text-secondary">Задачи появятся по дедлайну, встречи — по времени начала.</p></div>; }

function PeriodPicker({ anchor, onApply, onCancel }: Readonly<{ anchor: Date; onApply: (value: Date) => void; onCancel: () => void }>) {
  const [month, setMonth] = useState(String(anchor.getMonth()));
  const [year, setYear] = useState(String(anchor.getFullYear()));
  const currentYear = new Date().getFullYear();
  const monthOptions = ["Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"].map((label, index) => ({ label, value: String(index) })) as [{ label: string; value: string }, ...{ label: string; value: string }[]];
  const yearOptions = Array.from({ length: 21 }, (_, index) => ({ label: String(currentYear - 10 + index), value: String(currentYear - 10 + index) })) as [{ label: string; value: string }, ...{ label: string; value: string }[]];
  return <form data-cy="calendar-period-form" onSubmit={(event) => { event.preventDefault(); onApply(new Date(Number(year), Number(month), 1)); }}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Период</p><h2 className="mt-3 pr-10 font-display text-4xl">Выберите месяц и год</h2><div className="mt-8 grid gap-6 tablet:grid-cols-2"><Select label="Месяц" onValueChange={setMonth} options={monthOptions} value={month} /><Select label="Год" onValueChange={setYear} options={yearOptions} value={year} /></div><div className="mt-8 flex flex-wrap gap-3"><Button type="submit">Показать</Button><Button onClick={onCancel} type="button" variant="secondary">Отмена</Button></div></form>;
}

function calendarRange(anchor: Date, view: CalendarView) { if (view === "week") { const start = startOfWeek(anchor); return { start, end: addDays(start, 7) }; } const first = new Date(anchor.getFullYear(), anchor.getMonth(), 1); const start = startOfWeek(first); return { start, end: addDays(start, 42) }; }
function startOfWeek(value: Date) { const date = startOfDay(value); const weekday = date.getDay() || 7; date.setDate(date.getDate() - weekday + 1); return date; }
function startOfDay(value: Date) { return new Date(value.getFullYear(), value.getMonth(), value.getDate()); }
function addDays(value: Date, count: number) { const date = new Date(value); date.setDate(date.getDate() + count); return date; }
function datesBetween(start: Date, end: Date) { const values: Date[] = []; for (let date = new Date(start); date < end; date = addDays(date, 1)) values.push(date); return values; }
function sameDay(left: Date, right: Date) { return dateKey(left) === dateKey(right); }
function dateKey(value: Date) { return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}-${String(value.getDate()).padStart(2, "0")}`; }
function formatTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { hour: "2-digit", minute: "2-digit" }).format(new Date(value)); }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { day: "numeric", hour: "2-digit", minute: "2-digit", month: "long", year: "numeric" }).format(new Date(value)); }
function formatDay(value: Date) { return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long" }).format(value); }
function formatMonthYear(value: Date) { return new Intl.DateTimeFormat("ru-RU", { month: "long", year: "numeric" }).format(value); }
function CalendarIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 20 20"><rect height="13" rx="1" stroke="currentColor" strokeWidth="1.4" width="14" x="3" y="4"/><path d="M3 8h14M7 2.5v3M13 2.5v3" stroke="currentColor" strokeWidth="1.4"/></svg>; }
function ListIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 20 20"><path d="M7 5h10M7 10h10M7 15h10M3 5h.01M3 10h.01M3 15h.01" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6"/></svg>; }
function GridIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 20 20"><rect height="12" width="14" x="3" y="4" stroke="currentColor" strokeWidth="1.4"/><path d="M3 8h14M8 4v12M13 4v12" stroke="currentColor" strokeWidth="1.2"/></svg>; }
