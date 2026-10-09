"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { Button, DatePicker, Dialog, IconButton, Input, Select, Textarea } from "@/shared/ui";

import { archiveProject, loadCurrentAccount, loadProject, ProjectApiError, updateProject } from "../api/projects";
import type { CurrentAccount, ProjectStatus, ProjectView } from "../model/types";
import { ProjectUpcoming } from "./projectUpcoming";
import { ProjectMembers } from "./projectMembers";
import { ProjectFinance } from "./projectFinance";

const projectTypes = [
  { label: "Дизайн интерьера", value: "interior_design" }, { label: "Архитектурный проект", value: "architecture" },
  { label: "Авторский надзор", value: "supervision" }, { label: "Комплектация", value: "procurement" },
] as const;
const statuses = [
  { label: "Черновик", value: "draft" }, { label: "В работе", value: "active" },
  { label: "На паузе", value: "paused" }, { label: "Завершён", value: "completed" },
] as const;

export function ProjectWorkspacePage({ projectId }: Readonly<{ projectId: string }>) {
  const router = useRouter();
  const [project, setProject] = useState<ProjectView | null>(null);
  const [account, setAccount] = useState<CurrentAccount | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  async function refresh(signal?: AbortSignal) {
    setState("loading"); setError("");
    try {
      const [nextProject, nextAccount] = await Promise.all([loadProject(projectId, signal), loadCurrentAccount(signal)]);
      setProject(nextProject); setAccount(nextAccount); setState("ready");
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      if (reason instanceof ProjectApiError && reason.status === 401) { router.replace("/?auth=login"); return; }
      setError(reason instanceof ProjectApiError && reason.status === 404 ? "Проект не найден или у вас нет к нему доступа" : reason instanceof Error ? reason.message : "Не удалось загрузить проект");
      setState("error");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([loadProject(projectId, controller.signal), loadCurrentAccount(controller.signal)]).then(([nextProject, nextAccount]) => {
      setProject(nextProject); setAccount(nextAccount); setState("ready");
    }).catch((reason: unknown) => {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      if (reason instanceof ProjectApiError && reason.status === 401) { router.replace("/?auth=login"); return; }
      setError(reason instanceof ProjectApiError && reason.status === 404 ? "Проект не найден или у вас нет к нему доступа" : reason instanceof Error ? reason.message : "Не удалось загрузить проект");
      setState("error");
    });
    return () => controller.abort();
  }, [projectId, router]);

  if (state === "loading") return <div aria-label="Загрузка проекта" className="h-96 animate-pulse border border-border bg-surface" role="status" />;
  if (state === "error" || !project || !account) return <ErrorState error={error} onRetry={() => void refresh()} />;

  return (
    <section aria-labelledby="project-title" data-cy="project-workspace">
      <h1 className="sr-only" id="project-title">{project.name}</h1>
      <header className="flex flex-col gap-4 tablet:flex-row tablet:items-center tablet:justify-between">
        <nav aria-label="Хлебные крошки" className="text-xs text-secondary"><Link className="hover:text-primary" href="/account">Проекты</Link><span aria-hidden="true" className="px-2">/</span><span aria-current="page">{project.name}</span></nav>
        <div className="flex flex-wrap gap-4">
          <IconButton aria-label="Редактировать проект" className="relative size-9 border-icon-button-border bg-icon-button text-icon-button-text before:absolute before:-inset-1 hover:border-action hover:bg-icon-button-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action" data-cy="edit-project" onClick={() => setEditing(true)} tooltip="Редактировать проект" tooltipAlign="end"><EditIcon /></IconButton>
          {account.globalRole === "super_admin" || account.globalRole === "technical_admin" ? <IconButton aria-label="Удалить проект" className="relative size-9 border-red-700 bg-icon-button text-red-700 before:absolute before:-inset-1 hover:bg-icon-button-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700" data-cy="delete-project" onClick={() => setConfirmDelete(true)} tooltip="Удалить проект" tooltipAlign="end"><DeleteIcon /></IconButton> : null}
        </div>
      </header>

      <div className="mt-8 grid gap-5 desktop:grid-cols-[minmax(0,2fr)_minmax(240px,1fr)]">
        <ProjectUpcoming canCreate={account.globalRole === "super_admin" || account.globalRole === "technical_admin" || project.customerUserId === account.id || Boolean(project.adminUserIds?.includes(account.id))} projectId={project.id} />
        <ProjectMembers projectId={project.id} />
      </div>

      <ProjectFinance autoApproveExpenses={project.autoApproveExpenses} projectId={project.id} />

      <dl className="mt-5 grid gap-px border border-border bg-border tablet:grid-cols-2 desktop:grid-cols-4">
        <Fact label="Статус" value={statusLabel(project.status)} /><Fact label="Тип" value={typeLabel(project.type)} /><Fact label="Срок" value={formatRange(project.plannedStartOn, project.plannedFinishOn)} /><Fact label="Адрес" value={project.address || "Не указан"} />
      </dl>

      <Dialog isOpen={editing} label="Редактирование проекта" onClose={() => setEditing(false)}><EditProjectForm onCancel={() => setEditing(false)} onSaved={(value) => { setProject(value); setEditing(false); }} project={project} /></Dialog>
      <Dialog isOpen={confirmDelete} label="Удаление проекта" onClose={() => setConfirmDelete(false)}><DeleteProjectDialog onCancel={() => setConfirmDelete(false)} onDeleted={() => { router.replace("/account"); router.refresh(); }} project={project} /></Dialog>
    </section>
  );
}

function EditProjectForm({ onCancel, onSaved, project }: Readonly<{ onCancel: () => void; onSaved: (project: ProjectView) => void; project: ProjectView }>) {
  const [form, setForm] = useState({ name: project.name, type: project.type, status: project.status as Exclude<ProjectStatus, "pending_deletion" | "archived">, address: project.address ?? "", start: project.plannedStartOn ?? "", finish: project.plannedFinishOn ?? "", description: project.description ?? "", autoApprove: project.autoApproveExpenses });
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  const dateError = form.start && form.finish && form.start > form.finish ? "Дата начала не может быть позже даты завершения" : "";
  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    if (form.name.trim().length < 2) { setError("Укажите название проекта — минимум 2 символа"); return; }
    if (dateError) return;
    setPending(true);
    try { onSaved(await updateProject(project.id, project.version, { address: form.address.trim() || null, autoApproveExpenses: form.autoApprove, description: form.description.trim() || null, name: form.name.trim(), plannedFinishOn: form.finish || null, plannedStartOn: form.start || null, status: form.status, type: form.type })); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось сохранить проект"); }
    finally { setPending(false); }
  }
  return <form data-cy="edit-project-form" noValidate onSubmit={submit}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Проект</p><h2 className="mt-3 font-display text-5xl">Редактирование</h2><div className="mt-8 grid gap-6 tablet:grid-cols-2"><label className="grid gap-2 text-sm font-medium">Название *<Input autoFocus data-cy="edit-project-name" data-dialog-initial-focus maxLength={200} onChange={(event) => setForm({ ...form, name: event.target.value })} value={form.name} /></label><Select label="Тип" onValueChange={(type) => setForm({ ...form, type })} options={projectTypes} value={form.type} /><Select label="Статус" onValueChange={(status) => setForm({ ...form, status: status as typeof form.status })} options={statuses} value={form.status} /><label className="grid gap-2 text-sm font-medium">Адрес<Input maxLength={500} onChange={(event) => setForm({ ...form, address: event.target.value })} value={form.address} /></label><DatePicker label="Начало" onValueChange={(start) => setForm({ ...form, start })} value={form.start} /><DatePicker error={dateError} label="Завершение" min={form.start || undefined} onValueChange={(finish) => setForm({ ...form, finish })} value={form.finish} /><label className="grid gap-2 text-sm font-medium tablet:col-span-2">Описание<Textarea maxLength={5000} onChange={(event) => setForm({ ...form, description: event.target.value })} value={form.description} /></label></div><label className="mt-6 flex gap-3 text-sm"><input checked={form.autoApprove} className="size-4 accent-action" onChange={(event) => setForm({ ...form, autoApprove: event.target.checked })} type="checkbox" /> Автосогласование расходов</label>{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button data-cy="save-project" disabled={pending || Boolean(dateError)} type="submit">{pending ? "Сохраняем…" : "Сохранить"}</Button><Button className="border border-border bg-transparent text-primary hover:bg-background" onClick={onCancel} type="button">Отмена</Button></div></form>;
}

function DeleteProjectDialog({ onCancel, onDeleted, project }: Readonly<{ onCancel: () => void; onDeleted: () => void; project: ProjectView }>) {
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  async function remove() { setPending(true); setError(""); try { await archiveProject(project.id, project.version); onDeleted(); } catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось удалить проект"); setPending(false); } }
  return <div data-cy="delete-project-dialog"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-red-700">Необратимое действие</p><h2 className="mt-3 font-display text-5xl">Удалить проект?</h2><p className="mt-5 leading-7 text-secondary">Проект «{project.name}» исчезнет у всех участников. Удалить его может только супер-администратор.</p>{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button className="bg-red-700 text-white hover:bg-red-800" data-cy="confirm-delete-project" data-dialog-initial-focus disabled={pending} onClick={() => void remove()} type="button">{pending ? "Удаляем…" : "Удалить"}</Button><Button className="border border-border bg-transparent text-primary hover:bg-background" onClick={onCancel} type="button">Отмена</Button></div></div>;
}

function Fact({ label, value }: Readonly<{ label: string; value: string }>) { return <div className="bg-surface p-6"><dt className="text-xs uppercase tracking-[0.12em] text-secondary">{label}</dt><dd className="mt-2 font-medium">{value}</dd></div>; }
function ErrorState({ error, onRetry }: Readonly<{ error: string; onRetry: () => void }>) { return <div className="border border-red-700/40 bg-surface p-8" role="alert"><h1 className="font-display text-4xl">Проект не открылся</h1><p className="mt-3 text-secondary">{error}</p><div className="mt-6 flex gap-3"><Button onClick={onRetry}>Повторить</Button><Link className="inline-flex min-h-11 items-center rounded-full border border-border px-5" href="/account">К проектам</Link></div></div>; }
function EditIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M13.5 6.5 17.5 10.5M4 20l3.6-.7L19 7.9a2.8 2.8 0 0 0-4-4L3.7 15.3 3 19.9 4 20Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" /></svg>; }
function DeleteIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13M10 11v5m4-5v5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" /></svg>; }
function statusLabel(status: ProjectStatus) { return statuses.find((item) => item.value === status)?.label ?? status; }
function typeLabel(type: string) { return projectTypes.find((item) => item.value === type)?.label ?? type; }
function formatRange(start: string | null, finish: string | null) { if (!start && !finish) return "Не указан"; const format = (value: string) => new Intl.DateTimeFormat("ru-RU", { day: "2-digit", month: "short", year: "numeric" }).format(new Date(`${value}T00:00:00`)); return start && finish ? `${format(start)} — ${format(finish)}` : format(start ?? finish!); }
