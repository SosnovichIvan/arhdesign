"use client";

import Link from "next/link";
import { useEffect, useMemo, useState, type FormEvent } from "react";

import { Button, DatePicker, Dialog, IconButton, Input, Select, Textarea } from "@/shared/ui";

import { createProjectTask, loadCurrentAccount, loadProject, loadProjectMembers, loadProjectTasks, updateProjectTaskStatus } from "../api/projects";
import type { ProjectMember, ProjectTask, ProjectTaskStatus, ProjectView } from "../model/types";
import { ContextChatAction } from "./contextChatAction";

const statusOptions = [
  { label: "Все статусы", value: "all" },
  { label: "Новая", value: "new" },
  { label: "В работе", value: "in_progress" },
  { label: "На проверке", value: "review" },
  { label: "Нужны доработки", value: "changes_requested" },
  { label: "Принята", value: "accepted" },
] as const;
const taskStatusOptions: readonly [{ label: string; value: ProjectTaskStatus }, ...{ label: string; value: ProjectTaskStatus }[]] = [
  { label: "Новая", value: "new" }, { label: "В работе", value: "in_progress" },
  { label: "На проверке", value: "review" }, { label: "Нужны доработки", value: "changes_requested" },
  { label: "Принята", value: "accepted" },
];

const statusLabels: Record<ProjectTaskStatus, string> = { accepted: "Принята", changes_requested: "Нужны доработки", in_progress: "В работе", new: "Новая", review: "На проверке" };
const deadlineOptions = [
  { label: "Все сроки", value: "all" },
  { label: "Просроченные", value: "overdue" },
  { label: "Ближайшие 7 дней", value: "next_7_days" },
] as const;
const sortOptions = [
  { label: "Сначала ближайшие", value: "due_asc" },
  { label: "Сначала поздние", value: "due_desc" },
  { label: "Сначала новые", value: "created_desc" },
] as const;
type TaskFilters = { assignee: string; deadline: string; sort: string; status: string };
const defaultTaskFilters: TaskFilters = { assignee: "all", deadline: "all", sort: "due_asc", status: "all" };

export function ProjectTasksPage({ projectId }: Readonly<{ projectId: string }>) {
  const [project, setProject] = useState<ProjectView | null>(null);
  const [tasks, setTasks] = useState<ProjectTask[]>([]);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [currentUserId, setCurrentUserId] = useState("");
  const [canCreate, setCanCreate] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<TaskFilters>(defaultTaskFilters);
  const [scope, setScope] = useState<"all" | "mine">("all");
  const [formOpen, setFormOpen] = useState(false);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [renderedAt] = useState(() => Date.now());

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => {
      if (controller.signal.aborted) return;
      setState("loading"); setError("");
      void Promise.all([loadProject(projectId, controller.signal), loadProjectTasks(projectId, controller.signal), loadProjectMembers(projectId, controller.signal), loadCurrentAccount(controller.signal)]).then(([projectValue, taskList, memberList, account]) => {
        setProject(projectValue); setTasks(taskList.items); setCanCreate(taskList.canCreate); setMembers(memberList.items); setCurrentUserId(account.id); setState("ready");
      }).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить задачи проекта"); setState("error");
      });
    });
    return () => controller.abort();
  }, [projectId, reloadKey]);

  const visibleTasks = useMemo(() => {
    const normalizedSearch = search.trim().toLocaleLowerCase("ru");
    const now = renderedAt;
    const nextWeek = now + 7 * 24 * 60 * 60 * 1000;
    return tasks.filter((task) => {
      if (filters.status !== "all" && task.status !== filters.status) return false;
      if (filters.assignee !== "all" && !task.assignees.some((item) => item.userId === filters.assignee)) return false;
      if (scope === "mine" && !task.assignees.some((item) => item.userId === currentUserId)) return false;
      const dueAt = new Date(task.dueAt).getTime();
      if (filters.deadline === "overdue" && (task.status === "accepted" || dueAt >= now)) return false;
      if (filters.deadline === "next_7_days" && (dueAt < now || dueAt > nextWeek)) return false;
      return !normalizedSearch || `${task.title} ${task.description ?? ""}`.toLocaleLowerCase("ru").includes(normalizedSearch);
    }).sort((left, right) => filters.sort === "created_desc"
      ? new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime()
      : (new Date(left.dueAt).getTime() - new Date(right.dueAt).getTime()) * (filters.sort === "due_desc" ? -1 : 1));
  }, [currentUserId, filters, renderedAt, scope, search, tasks]);

  const activeFilterCount = Number(filters.status !== "all") + Number(filters.assignee !== "all") + Number(filters.deadline !== "all");

  function reload(message?: string) { if (message) setNotice(message); setReloadKey((value) => value + 1); }

  return <section aria-labelledby="project-tasks-title" data-cy="project-tasks-page">
    <div className="flex flex-col gap-6 tablet:flex-row tablet:items-start tablet:justify-between">
      <div className="min-w-0">
        <nav aria-label="Хлебные крошки" className="flex flex-wrap items-center gap-2 text-xs text-secondary"><Link className="hover:text-primary" href="/account">Проекты</Link><span aria-hidden="true">/</span><Link className="truncate hover:text-primary" href={`/account/projects/${encodeURIComponent(projectId)}`}>{project?.name ?? "Проект"}</Link><span aria-hidden="true">/</span><span aria-current="page">Задачи</span></nav>
        <p className="mt-7 text-xs font-semibold uppercase tracking-[0.14em] text-action">Работа по проекту</p>
        <h1 className="mt-3 font-display text-5xl" id="project-tasks-title">Задачи проекта</h1>
        <p className="mt-4 max-w-2xl text-sm leading-6 text-secondary">Сроки, ответственные и текущее состояние работ в одном списке.</p>
      </div>
      {state === "ready" && canCreate ? <IconButton aria-label="Создать задачу" className="size-10 shrink-0 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="create-task-page" onClick={() => { setNotice(""); setFormOpen(true); }} tooltip="Создать задачу" tooltipAlign="end"><PlusIcon /></IconButton> : null}
    </div>

    {notice ? <p className="mt-6 border border-action/40 bg-action/10 p-3 text-sm" role="status">{notice}</p> : null}
    {state === "loading" ? <div aria-label="Загрузка задач" className="mt-8 h-72 animate-pulse border border-border bg-surface" role="status" /> : null}
    {state === "error" ? <div className="mt-8 border border-red-700/40 bg-surface p-5" role="alert"><p className="text-sm text-red-700">{error}</p><Button className="mt-4" onClick={() => reload()} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" ? <>
      <div className="mt-8 flex min-w-0 items-center gap-3 border border-border bg-surface p-3" data-cy="task-toolbar">
        <div aria-label="Область задач" className="flex min-h-11 shrink-0 items-center gap-1 rounded-full border border-border p-1">{([['all', 'Все'], ['mine', 'Мои']] as const).map(([value, label]) => <button aria-pressed={scope === value} className="min-h-9 rounded-full px-4 text-xs font-semibold text-secondary aria-pressed:bg-action aria-pressed:text-inverse-text" key={value} onClick={() => setScope(value)} type="button">{label}</button>)}</div>
        <label className="min-w-0 flex-1"><span className="sr-only">Поиск задач</span><Input data-cy="task-search" onChange={(event) => setSearch(event.target.value)} placeholder="Поиск по названию или описанию" type="search" value={search} /></label>
        <div className="relative shrink-0">
          <IconButton aria-label={activeFilterCount > 0 ? `Настроить фильтры, применено: ${activeFilterCount}` : "Настроить фильтры"} className="size-11 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="task-filter-settings" onClick={() => setFiltersOpen(true)} tooltip="Настроить фильтры" tooltipAlign="end"><FilterIcon /></IconButton>
          {activeFilterCount > 0 ? <span aria-hidden="true" className="absolute -right-1 -top-1 flex min-h-5 min-w-5 items-center justify-center rounded-full bg-action px-1 text-[10px] font-bold text-inverse-text" data-cy="task-filter-count">{activeFilterCount}</span> : null}
        </div>
      </div>
      {tasks.length === 0 ? <EmptyState canCreate={canCreate} onCreate={() => setFormOpen(true)} title="В проекте пока нет задач" /> : visibleTasks.length === 0 ? <EmptyState canCreate={false} onCreate={() => undefined} title="По выбранным фильтрам задач нет" /> : <ul className="mt-5 grid gap-3" data-cy="project-task-list">{visibleTasks.map((task) => <TaskRow key={task.id} onUpdated={(updated) => { setTasks((current) => current.map((item) => item.id === updated.id ? updated : item)); setNotice("Статус задачи обновлён"); }} projectId={projectId} task={task} />)}</ul>}
    </> : null}
    {filtersOpen ? <Dialog isOpen label="Расширенные фильтры задач" onClose={() => setFiltersOpen(false)}><TaskFiltersForm initial={filters} members={members} onApply={(nextFilters) => { setFilters(nextFilters); setFiltersOpen(false); }} onCancel={() => setFiltersOpen(false)} /></Dialog> : null}
    <Dialog isOpen={formOpen} label="Создание задачи" onClose={() => setFormOpen(false)}><CreateTaskForm members={members} onCancel={() => setFormOpen(false)} onCreated={() => { setFormOpen(false); reload("Задача создана"); }} projectId={projectId} /></Dialog>
  </section>;
}

function TaskFiltersForm({ initial, members, onApply, onCancel }: Readonly<{ initial: TaskFilters; members: ProjectMember[]; onApply: (filters: TaskFilters) => void; onCancel: () => void }>) {
  const [draft, setDraft] = useState<TaskFilters>(initial);
  function update(key: keyof TaskFilters, value: string) { setDraft((current) => ({ ...current, [key]: value })); }
  return <form data-cy="task-filters-form" onSubmit={(event) => { event.preventDefault(); onApply(draft); }}>
    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Задачи проекта</p>
    <h2 className="mt-3 font-display text-5xl">Расширенные фильтры</h2>
    <p className="mt-3 text-sm leading-6 text-secondary">Уточните список задач. Изменения применятся только после сохранения.</p>
    <div className="mt-8 grid gap-6 tablet:grid-cols-2">
      <Select label="Статус" onValueChange={(value) => update("status", value)} options={statusOptions} value={draft.status} />
      <Select label="Ответственный" onValueChange={(value) => update("assignee", value)} options={[{ label: "Все ответственные", value: "all" }, ...members.map((member) => ({ label: memberName(member), value: member.userId }))]} value={draft.assignee} />
      <Select label="Срок" onValueChange={(value) => update("deadline", value)} options={deadlineOptions} value={draft.deadline} />
      <Select label="Сортировка" onValueChange={(value) => update("sort", value)} options={sortOptions} value={draft.sort} />
    </div>
    <div className="mt-8 flex flex-wrap gap-3">
      <Button data-cy="apply-task-filters" type="submit">Сохранить</Button>
      <Button data-cy="reset-task-filters" onClick={() => setDraft(defaultTaskFilters)} type="button" variant="secondary">Сбросить</Button>
      <Button onClick={onCancel} type="button" variant="secondary">Отмена</Button>
    </div>
  </form>;
}

function TaskRow({ onUpdated, projectId, task }: Readonly<{ onUpdated: (task: ProjectTask) => void; projectId: string; task: ProjectTask }>) {
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  const [renderedAt] = useState(() => Date.now());
  const overdue = task.status !== "accepted" && new Date(task.dueAt).getTime() < renderedAt;
  async function changeStatus(value: string) {
    if (value === task.status) return;
    setPending(true); setError("");
    try { onUpdated(await updateProjectTaskStatus(projectId, task.id, task.version, value as ProjectTaskStatus)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось изменить статус"); }
    finally { setPending(false); }
  }
  return <li className="border border-border bg-surface p-5" data-cy={`project-task-${task.id}`}>
    <div className="grid gap-5 desktop:grid-cols-[minmax(0,1fr)_220px_190px] desktop:items-start">
      <div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><span className={`rounded-full px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.08em] ${statusTone(task.status)}`}>{statusLabels[task.status]}</span>{overdue ? <span className="text-xs font-semibold text-red-700">Просрочено</span> : null}</div><h2 className="mt-3 text-lg font-medium">{task.title}</h2>{task.description ? <p className="mt-2 text-sm leading-6 text-secondary">{task.description}</p> : null}<p className="mt-4 text-xs text-secondary">Ответственные: {task.assignees.length > 0 ? task.assignees.map(assigneeName).join(", ") : "не назначены"}</p>{error ? <p className="mt-3 text-sm text-red-700" role="alert">{error}</p> : null}</div>
      <div><p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-secondary">Дедлайн</p><time className={`mt-2 block text-sm ${overdue ? "font-semibold text-red-700" : "text-primary"}`} dateTime={task.dueAt}>{formatDateTime(task.dueAt)}</time>{task.startedAt ? <p className="mt-3 text-xs text-secondary">Старт: {formatDateTime(task.startedAt)}</p> : null}{task.completedAt ? <p className="mt-1 text-xs text-secondary">Завершение: {formatDateTime(task.completedAt)}</p> : null}</div>
      <div className="flex items-start gap-2"><div className="min-w-0 flex-1">{task.canChangeStatus ? <Select disabled={pending} label="Изменить статус" onValueChange={(value) => void changeStatus(value)} options={statusOptionsForTask(task)} value={task.status} /> : <p className="pt-2 text-xs text-secondary">Только просмотр</p>}</div><ContextChatAction chatId={task.contextChatId} contextId={task.id} contextTitle={task.title} contextType="task" projectId={projectId} /></div>
    </div>
  </li>;
}

function CreateTaskForm({ members, onCancel, onCreated, projectId }: Readonly<{ members: ProjectMember[]; onCancel: () => void; onCreated: () => void; projectId: string }>) {
  const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [assignee, setAssignee] = useState("");
  const [date, setDate] = useState(""); const [time, setTime] = useState("12:00"); const [pending, setPending] = useState(false); const [error, setError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    if (title.trim().length < 2) { setError("Укажите название задачи — минимум 2 символа"); return; }
    if (!assignee) { setError("Выберите ответственного из участников проекта"); return; }
    const dueAt = localInstant(date, time);
    if (!dueAt || dueAt <= new Date()) { setError("Выберите будущие дату и время дедлайна"); return; }
    setPending(true);
    try { await createProjectTask(projectId, { assigneeUserIds: [assignee], description: description.trim() || undefined, dueAt: dueAt.toISOString(), title: title.trim() }); onCreated(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось создать задачу"); }
    finally { setPending(false); }
  }
  return <form data-cy="create-task-page-form" noValidate onSubmit={submit}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Задача проекта</p><h2 className="mt-3 font-display text-5xl">Новая задача</h2><div className="mt-8 grid gap-6 tablet:grid-cols-2"><label className="grid gap-2 text-sm font-medium tablet:col-span-2">Название *<Input autoFocus data-dialog-initial-focus maxLength={200} onChange={(event) => setTitle(event.target.value)} value={title} /></label><label className="grid gap-2 text-sm font-medium tablet:col-span-2">Описание<Textarea maxLength={5000} onChange={(event) => setDescription(event.target.value)} value={description} /></label><Select label="Ответственный" onValueChange={setAssignee} options={[{ label: "Выберите участника", value: "" }, ...members.map((member) => ({ label: memberName(member), value: member.userId }))]} required value={assignee} /><div className="grid gap-4"><DatePicker label="Дедлайн, дата" min={todayDateValue()} onValueChange={setDate} required value={date} /><label className="grid gap-2 text-sm font-medium">Дедлайн, время *<Input aria-label="Дедлайн, время *" onChange={(event) => setTime(event.target.value)} type="time" value={time} /></label></div></div>{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button data-cy="submit-task-page" disabled={pending} type="submit">{pending ? "Создаём…" : "Создать задачу"}</Button><Button disabled={pending} onClick={onCancel} type="button" variant="secondary">Отмена</Button></div></form>;
}

function EmptyState({ canCreate, onCreate, title }: Readonly<{ canCreate: boolean; onCreate: () => void; title: string }>) { return <div className="mt-5 border border-border bg-surface px-5 py-12 text-center"><h2 className="text-lg font-medium">{title}</h2><p className="mx-auto mt-2 max-w-lg text-sm leading-6 text-secondary">Измените фильтры или создайте первую задачу с ответственным и дедлайном.</p>{canCreate ? <Button className="mt-5" onClick={onCreate}>Создать задачу</Button> : null}</div>; }
function memberName(member: Pick<ProjectMember, "firstName" | "lastName" | "login">) { return `${[member.firstName, member.lastName].filter(Boolean).join(" ")} · @${member.login}`; }
function assigneeName(member: ProjectTask["assignees"][number]) { return [member.firstName, member.lastName].filter(Boolean).join(" ") || `@${member.login}`; }
function localInstant(date: string, time: string) { if (!date || !time) return null; const value = new Date(`${date}T${time}:00`); return Number.isNaN(value.getTime()) ? null : value; }
function todayDateValue() { const today = new Date(); return `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`; }
function formatDateTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { day: "numeric", hour: "2-digit", minute: "2-digit", month: "short", year: "numeric" }).format(new Date(value)); }
function statusOptionsForTask(task: ProjectTask): typeof taskStatusOptions {
  if (task.canEdit) return taskStatusOptions;
  const allowed: ProjectTaskStatus[] = task.status === "review"
    ? ["review", "changes_requested", "accepted"]
    : task.status === "in_progress"
      ? ["in_progress", "review"]
      : [task.status, "in_progress"];
  return taskStatusOptions.filter((option) => allowed.includes(option.value)) as unknown as typeof taskStatusOptions;
}
function statusTone(status: ProjectTaskStatus) { if (status === "accepted") return "bg-green-700/15 text-green-800 dark:text-green-300"; if (status === "changes_requested") return "bg-red-700/10 text-red-700 dark:text-red-300"; if (status === "review") return "bg-amber-600/15 text-amber-800 dark:text-amber-300"; if (status === "in_progress") return "bg-action/20 text-primary"; return "bg-page text-secondary"; }
function PlusIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>; }
function FilterIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M4 7h10M18 7h2M4 17h2M10 17h10M14 4v6M10 14v6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>; }
