"use client";

import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";

import { Button, DatePicker, Dialog, IconButton, Input, Textarea } from "@/shared/ui";

import { createProjectMeeting, createProjectTask, loadProjectUpcoming, loadProjectUserSettings, updateProjectUserSettings } from "../api/projects";
import type { ProjectUpcomingItem } from "../model/types";

type Filter = "all" | "task" | "meeting";

export function ProjectUpcoming({ canCreate, projectId }: Readonly<{ canCreate: boolean; projectId: string }>) {
  const [days, setDays] = useState(7);
  const [items, setItems] = useState<ProjectUpcomingItem[]>([]);
  const [filter, setFilter] = useState<Filter>("all");
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [taskOpen, setTaskOpen] = useState(false);
  const [meetingOpen, setMeetingOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [settingsReloadKey, setSettingsReloadKey] = useState(0);
  const [settingsProjectId, setSettingsProjectId] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => {
      if (controller.signal.aborted) return;
      setState("loading"); setError(""); setSettingsProjectId("");
      void loadProjectUserSettings(projectId, controller.signal).then((settings) => {
        setDays(settings.upcomingDays); setSettingsProjectId(projectId);
      }).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить настройки проекта"); setState("error");
      });
    });
    return () => controller.abort();
  }, [projectId, settingsReloadKey]);

  useEffect(() => {
    if (settingsProjectId !== projectId) return;
    const controller = new AbortController();
    queueMicrotask(() => {
      if (controller.signal.aborted) return;
      setState("loading"); setError("");
      void loadProjectUpcoming(projectId, days, controller.signal).then((feed) => {
        setItems(feed.items); setState("ready");
      }).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить ближайшие события"); setState("error");
      });
    });
    return () => controller.abort();
  }, [days, projectId, reloadKey, settingsProjectId]);

  const visibleItems = useMemo(() => filter === "all" ? items : items.filter((item) => item.kind === filter), [filter, items]);
  function created(kind: "task" | "meeting") {
    setSuccess(kind === "task" ? "Задача создана" : "Встреча создана");
    setTaskOpen(false); setMeetingOpen(false); setFilter("all"); setState("loading"); setError(""); setReloadKey((value) => value + 1);
  }
  async function savePeriod(value: number) {
    const settings = await updateProjectUserSettings(projectId, { upcomingDays: value });
    setState("loading"); setError(""); setDays(settings.upcomingDays); setSettingsOpen(false); setSuccess(`Период обновлён: ${settings.upcomingDays} ${dayLabel(settings.upcomingDays)}`);
  }

  return (
    <section className="border border-border bg-surface p-5" aria-labelledby="project-upcoming-title" data-cy="project-upcoming">
      <div className="flex flex-col gap-4 tablet:flex-row tablet:items-start tablet:justify-between">
        <div><h2 className="text-xl font-medium" id="project-upcoming-title">Ближайшее по проекту</h2><p className="mt-1 text-xs text-secondary">Задачи и встречи на ближайшие {days} {dayLabel(days)}</p></div>
        <div aria-label="Действия с планами проекта" className="flex gap-4">
          {canCreate ? <ActionIcon label="Создать задачу" onClick={() => { setSuccess(""); setTaskOpen(true); }}><TaskIcon /></ActionIcon> : null}
          {canCreate ? <ActionIcon label="Создать встречу" onClick={() => { setSuccess(""); setMeetingOpen(true); }}><MeetingIcon /></ActionIcon> : null}
          <ActionIcon label="Настроить период" onClick={() => setSettingsOpen(true)}><SettingsIcon /></ActionIcon>
        </div>
      </div>
      <div className="mt-5 flex flex-wrap gap-2" aria-label="Фильтр ближайших событий">
        {([['all', 'Все'], ['task', 'Задачи'], ['meeting', 'Встречи']] as const).map(([value, label]) => <button aria-pressed={filter === value} className="min-h-9 rounded-full px-4 py-2 text-xs font-semibold text-secondary transition-colors hover:text-primary aria-pressed:bg-action/30 aria-pressed:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action" key={value} onClick={() => setFilter(value)} type="button">{label}</button>)}
      </div>
      {success ? <p className="mt-4 text-sm text-green-700 dark:text-green-400" role="status">{success}</p> : null}
      <div className="mt-5 border-t border-border pt-5">
        {state === "loading" ? <p className="text-sm text-secondary" role="status">Загружаем ближайшие события…</p> : null}
        {state === "error" ? <div role="alert"><p className="text-sm text-red-700">{error}</p><Button className="mt-4" onClick={() => { setState("loading"); setError(""); setReloadKey((value) => value + 1); setSettingsReloadKey((value) => value + 1); }} variant="secondary">Повторить</Button></div> : null}
        {state === "ready" && visibleItems.length === 0 ? <div className="py-3 text-sm text-secondary"><p className="font-medium text-primary">{items.length === 0 ? "Ближайших событий пока нет" : "В выбранном фильтре событий нет"}</p><p className="mt-2">{items.length === 0 ? canCreate ? "Создайте задачу или встречу — она появится здесь для участников проекта." : "Новые события появятся здесь после назначения организатором проекта." : "Выберите другой тип события."}</p></div> : null}
        {state === "ready" && visibleItems.length > 0 ? <ul className="grid gap-3" data-cy="upcoming-list">{visibleItems.map((item) => <UpcomingCard item={item} key={`${item.kind}-${item.id}`} />)}</ul> : null}
      </div>
      <Dialog isOpen={taskOpen} label="Создание задачи" onClose={() => setTaskOpen(false)}><TaskForm onCancel={() => setTaskOpen(false)} onCreated={() => created("task")} projectId={projectId} /></Dialog>
      <Dialog isOpen={meetingOpen} label="Создание встречи" onClose={() => setMeetingOpen(false)}><MeetingForm onCancel={() => setMeetingOpen(false)} onCreated={() => created("meeting")} projectId={projectId} /></Dialog>
      <Dialog isOpen={settingsOpen} label="Настройка периода" onClose={() => setSettingsOpen(false)}><PeriodForm days={days} onCancel={() => setSettingsOpen(false)} onSave={savePeriod} /></Dialog>
    </section>
  );
}

function ActionIcon({ children, label, onClick }: Readonly<{ children: ReactNode; label: string; onClick: () => void }>) {
  return <IconButton aria-label={label} className="relative size-9 border-icon-button-border bg-icon-button text-icon-button-text before:absolute before:-inset-1 hover:border-action hover:bg-icon-button-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action" onClick={onClick} tooltip={label} tooltipAlign={label === "Настроить период" ? "end" : "center"}>{children}</IconButton>;
}

function TaskForm({ onCancel, onCreated, projectId }: Readonly<{ onCancel: () => void; onCreated: () => void; projectId: string }>) {
  const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [date, setDate] = useState(""); const [time, setTime] = useState("12:00");
  const [pending, setPending] = useState(false); const [error, setError] = useState(""); const [deadlineError, setDeadlineError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault(); setError(""); setDeadlineError("");
    if (title.trim().length < 2) { setError("Укажите название задачи — минимум 2 символа"); return; }
    if (!date) { setDeadlineError("Выберите дату дедлайна"); return; }
    if (!time) { setDeadlineError("Укажите время дедлайна"); return; }
    const dueAt = localInstant(date, time);
    if (!dueAt) { setDeadlineError("Проверьте дату и время дедлайна"); return; }
    if (dueAt <= new Date()) { setDeadlineError("Выбранный дедлайн уже прошёл. Выберите более позднее время или следующий день"); return; }
    setPending(true);
    try { await createProjectTask(projectId, { title: title.trim(), description: description.trim() || undefined, dueAt: dueAt.toISOString() }); onCreated(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось создать задачу"); }
    finally { setPending(false); }
  }
  return <form data-cy="create-task-form" noValidate onSubmit={submit}><FormHeading eyebrow="Задача" title="Новая задача" /><div className="mt-8 grid gap-6"><label className="grid gap-2 text-sm font-medium">Название *<Input autoFocus data-dialog-initial-focus maxLength={200} onChange={(event) => setTitle(event.target.value)} value={title} /></label><label className="grid gap-2 text-sm font-medium">Описание<Textarea maxLength={5000} onChange={(event) => setDescription(event.target.value)} value={description} /></label><DateTimeFields date={date} dateLabel="Дедлайн" error={deadlineError} min={todayDateValue()} onDateChange={(value) => { setDate(value); setDeadlineError(""); }} onTimeChange={(value) => { setTime(value); setDeadlineError(""); }} time={time} /></div><FormActions error={error} onCancel={onCancel} pending={pending} submitLabel="Создать задачу" /></form>;
}

function MeetingForm({ onCancel, onCreated, projectId }: Readonly<{ onCancel: () => void; onCreated: () => void; projectId: string }>) {
  const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [location, setLocation] = useState("");
  const [startDate, setStartDate] = useState(""); const [startTime, setStartTime] = useState("12:00"); const [endDate, setEndDate] = useState(""); const [endTime, setEndTime] = useState("13:00");
  const [pending, setPending] = useState(false); const [error, setError] = useState(""); const [startError, setStartError] = useState(""); const [endError, setEndError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault(); setError(""); setStartError(""); setEndError("");
    if (title.trim().length < 2) { setError("Укажите название встречи — минимум 2 символа"); return; }
    if (!startDate) { setStartError("Выберите дату начала встречи"); return; }
    if (!startTime) { setStartError("Укажите время начала встречи"); return; }
    const startsAt = localInstant(startDate, startTime);
    if (!startsAt) { setStartError("Проверьте дату и время начала встречи"); return; }
    if (startsAt <= new Date()) { setStartError("Выбранное время начала уже прошло. Выберите более позднее время или следующий день"); return; }
    if (!endDate) { setEndError("Выберите дату окончания встречи"); return; }
    if (!endTime) { setEndError("Укажите время окончания встречи"); return; }
    const endsAt = localInstant(endDate, endTime);
    if (!endsAt) { setEndError("Проверьте дату и время окончания встречи"); return; }
    if (endsAt <= startsAt) { setEndError("Окончание встречи должно быть позже начала"); return; }
    setPending(true);
    try { await createProjectMeeting(projectId, { title: title.trim(), description: description.trim() || undefined, location: location.trim() || undefined, startsAt: startsAt.toISOString(), endsAt: endsAt.toISOString() }); onCreated(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось создать встречу"); }
    finally { setPending(false); }
  }
  return <form data-cy="create-meeting-form" noValidate onSubmit={submit}><FormHeading eyebrow="Встреча" title="Новая встреча" /><div className="mt-8 grid gap-6"><label className="grid gap-2 text-sm font-medium">Название *<Input autoFocus data-dialog-initial-focus maxLength={200} onChange={(event) => setTitle(event.target.value)} value={title} /></label><label className="grid gap-2 text-sm font-medium">Место<Input maxLength={500} onChange={(event) => setLocation(event.target.value)} value={location} /></label><label className="grid gap-2 text-sm font-medium">Описание<Textarea maxLength={5000} onChange={(event) => setDescription(event.target.value)} value={description} /></label><div className="grid gap-6 tablet:grid-cols-2"><DateTimeFields date={startDate} dateLabel="Начало" error={startError} min={todayDateValue()} onDateChange={(value) => { setStartDate(value); setStartError(""); }} onTimeChange={(value) => { setStartTime(value); setStartError(""); }} time={startTime} /><DateTimeFields date={endDate} dateLabel="Окончание" error={endError} min={startDate || todayDateValue()} onDateChange={(value) => { setEndDate(value); setEndError(""); }} onTimeChange={(value) => { setEndTime(value); setEndError(""); }} time={endTime} /></div></div><FormActions error={error} onCancel={onCancel} pending={pending} submitLabel="Создать встречу" /></form>;
}

function DateTimeFields({ date, dateLabel, error, min, onDateChange, onTimeChange, time }: Readonly<{ date: string; dateLabel: string; error?: string; min?: string; onDateChange: (value: string) => void; onTimeChange: (value: string) => void; time: string }>) {
  return <div className="grid gap-4"><DatePicker error={error} label={`${dateLabel}, дата`} min={min} onValueChange={onDateChange} required value={date} /><label className="grid gap-2 text-sm font-medium">{dateLabel}, время *<Input aria-invalid={Boolean(error)} aria-label={`${dateLabel}, время *`} onChange={(event) => onTimeChange(event.target.value)} type="time" value={time} /></label></div>;
}

function PeriodForm({ days, onCancel, onSave }: Readonly<{ days: number; onCancel: () => void; onSave: (days: number) => Promise<void> }>) {
  const [value, setValue] = useState(String(days)); const [pending, setPending] = useState(false); const [error, setError] = useState(""); const parsed = Number(value); const invalid = !Number.isInteger(parsed) || parsed < 1 || parsed > 90;
  async function submit(event: FormEvent) { event.preventDefault(); if (invalid || pending) return; setPending(true); setError(""); try { await onSave(parsed); } catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось сохранить настройки проекта"); } finally { setPending(false); } }
  return <form data-cy="upcoming-period-form" onSubmit={submit}><FormHeading eyebrow="Период" title="Ближайшие события" /><p className="mt-5 text-sm leading-6 text-secondary">Выберите, на сколько дней вперёд показывать задачи и встречи в сводке проекта.</p><label className="mt-7 grid gap-2 text-sm font-medium">Количество дней *<Input aria-invalid={invalid} autoFocus data-dialog-initial-focus max={90} min={1} onChange={(event) => setValue(event.target.value)} type="number" value={value} /></label>{invalid ? <p className="mt-3 text-sm text-red-700" role="alert">Введите целое число от 1 до 90</p> : null}{error ? <p className="mt-3 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button disabled={invalid || pending} type="submit">{pending ? "Сохраняем…" : "Сохранить"}</Button><Button disabled={pending} onClick={onCancel} type="button" variant="secondary">Отмена</Button></div></form>;
}

function FormHeading({ eyebrow, title }: Readonly<{ eyebrow: string; title: string }>) { return <><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">{eyebrow}</p><h2 className="mt-3 font-display text-5xl">{title}</h2></>; }
function FormActions({ error, onCancel, pending, submitLabel }: Readonly<{ error: string; onCancel: () => void; pending: boolean; submitLabel: string }>) { return <>{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button disabled={pending} type="submit">{pending ? "Сохраняем…" : submitLabel}</Button><Button onClick={onCancel} type="button" variant="secondary">Отмена</Button></div></>; }

function UpcomingCard({ item }: Readonly<{ item: ProjectUpcomingItem }>) {
  return <li className="grid gap-3 border border-border p-4 tablet:grid-cols-[132px_minmax(0,1fr)]"><div><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">{item.kind === "task" ? "Задача" : "Встреча"}</p><time className="mt-2 block text-xs text-secondary" dateTime={item.effectiveAt}>{formatUpcomingDate(item.effectiveAt)}</time>{item.endsAt ? <p className="mt-1 text-xs text-secondary">до {formatTime(item.endsAt)}</p> : null}</div><div><h3 className="font-medium">{item.title}</h3>{item.description ? <p className="mt-2 text-sm leading-6 text-secondary">{item.description}</p> : null}{item.location ? <p className="mt-2 text-xs text-secondary">Место: {item.location}</p> : null}</div></li>;
}

function localInstant(date: string, time: string) { if (!date || !time) return null; const value = new Date(`${date}T${time}:00`); return Number.isNaN(value.getTime()) ? null : value; }
function todayDateValue() { const today = new Date(); const month = String(today.getMonth() + 1).padStart(2, "0"); const day = String(today.getDate()).padStart(2, "0"); return `${today.getFullYear()}-${month}-${day}`; }
function formatUpcomingDate(value: string) { return new Intl.DateTimeFormat("ru-RU", { day: "numeric", hour: "2-digit", minute: "2-digit", month: "short" }).format(new Date(value)); }
function formatTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { hour: "2-digit", minute: "2-digit" }).format(new Date(value)); }
function dayLabel(value: number) { const modulo100 = value % 100; const modulo10 = value % 10; if (modulo100 >= 11 && modulo100 <= 14) return "дней"; if (modulo10 === 1) return "день"; if (modulo10 >= 2 && modulo10 <= 4) return "дня"; return "дней"; }
function TaskIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M9 5h10M9 12h10M9 19h6M4 5h.01M4 12h.01M4 19h.01" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" /><path d="M18 16v6m-3-3h6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" /></svg>; }
function MeetingIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><rect height="16" rx="2" stroke="currentColor" strokeWidth="1.7" width="18" x="3" y="5" /><path d="M7 3v4m10-4v4M3 10h18" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /><path d="M8 14h3m2 0h3m-8 3h3" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>; }
function SettingsIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.09a2 2 0 0 1 1 1.74v.5a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.09a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" /><circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.6" /></svg>; }
