"use client";

import { useEffect, useState } from "react";

import { Autocomplete, Button, Dialog, IconButton, type AutocompleteOption } from "@/shared/ui";

import { addProjectMember, loadProjectMembers, removeProjectMember, searchProjectMemberCandidates } from "../api/projects";
import type { ProjectMember, ProjectMemberCandidate, ProjectRole } from "../model/types";

type CandidateOption = AutocompleteOption & { candidate: ProjectMemberCandidate };

export function ProjectMembers({ projectId }: Readonly<{ projectId: string }>) {
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [canManage, setCanManage] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [removeMember, setRemoveMember] = useState<ProjectMember | null>(null);

  async function refresh(signal?: AbortSignal) {
    setState("loading");
    setError("");
    try {
      const value = await loadProjectMembers(projectId, signal);
      setMembers(value.items);
      setCanManage(value.canManage);
      setState("ready");
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить участников");
      setState("error");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    void loadProjectMembers(projectId, controller.signal).then((value) => {
      setMembers(value.items);
      setCanManage(value.canManage);
      setState("ready");
    }).catch((reason: unknown) => {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить участников");
      setState("error");
    });
    return () => controller.abort();
  }, [projectId]);

  return (
    <section aria-labelledby="project-members-title" className="border border-border bg-surface p-5" data-cy="project-members">
      <header className="flex items-center justify-between gap-4">
        <div><h2 className="text-xl font-medium" id="project-members-title">Участники</h2><p className="mt-1 text-xs text-secondary">Команда и роли в этом проекте</p></div>
        {state === "ready" && canManage ? <IconButton aria-label="Настроить участников" className="size-9 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="manage-project-members" onClick={() => setSettingsOpen(true)} tooltip="Настроить участников" tooltipAlign="end"><SettingsIcon /></IconButton> : null}
      </header>
      {state === "loading" ? <div aria-label="Загрузка участников" className="mt-6 h-20 animate-pulse bg-page" role="status" /> : null}
      {state === "error" ? <div className="mt-6 text-sm" role="alert"><p className="text-red-700">{error}</p><Button className="mt-3 border border-border bg-transparent text-primary hover:bg-page" onClick={() => void refresh()} type="button">Повторить</Button></div> : null}
      {state === "ready" && members.length === 0 ? <p className="mt-6 text-sm text-secondary">В проекте пока нет участников.</p> : null}
      {state === "ready" && members.length > 0 ? <ul className="mt-6 divide-y divide-border">{members.map((member) => <MemberRow key={member.userId} member={member} />)}</ul> : null}
      {state === "ready" && !members.some((member) => member.projectRoles.includes("customer")) ? <p className="mt-6 border-t border-border pt-4 text-xs text-secondary">Заказчик пока не назначен.</p> : null}

      <Dialog isOpen={settingsOpen} label="Настройка участников" onClose={() => setSettingsOpen(false)}><MemberSettingsDialog members={members} onAdded={(member) => setMembers((current) => [...current, member])} onClose={() => setSettingsOpen(false)} onRemove={(member) => { setSettingsOpen(false); setRemoveMember(member); }} projectId={projectId} /></Dialog>
      <Dialog isOpen={Boolean(removeMember)} label="Удаление участника" onClose={() => setRemoveMember(null)}>{removeMember ? <RemoveMemberDialog member={removeMember} onCancel={() => setRemoveMember(null)} onRemoved={() => { setMembers((current) => current.filter((item) => item.userId !== removeMember.userId)); setRemoveMember(null); }} projectId={projectId} /> : null}</Dialog>
    </section>
  );
}

function MemberRow({ member }: Readonly<{ member: ProjectMember }>) {
  return <li className="flex items-center gap-3 py-4 first:pt-0 last:pb-0"><span aria-hidden="true" className="grid size-10 shrink-0 place-items-center rounded-full bg-page text-sm font-semibold text-action">{member.firstName.slice(0, 1).toUpperCase()}</span><span className="min-w-0"><span className="block truncate text-sm font-semibold">{fullName(member)}</span><span className="mt-1 block truncate text-xs text-secondary">{projectRolesLabel(member.projectRoles)} · {member.professionalRole.name}</span></span></li>;
}

function MemberSettingsDialog({ members, onAdded, onClose, onRemove, projectId }: Readonly<{ members: ProjectMember[]; onAdded: (member: ProjectMember) => void; onClose: () => void; onRemove: (member: ProjectMember) => void; projectId: string }>) {
  const [query, setQuery] = useState("");
  const [candidates, setCandidates] = useState<ProjectMemberCandidate[]>([]);
  const [selected, setSelected] = useState<ProjectMemberCandidate | null>(null);
  const [searching, setSearching] = useState(false);
  const [pending, setPending] = useState(false);
  const [searchError, setSearchError] = useState("");
  const [submitError, setSubmitError] = useState("");
  const normalizedQuery = query.trim();
  const options: CandidateOption[] = candidates.map((candidate) => ({ candidate, description: `@${candidate.login} · ${candidate.email} · ${candidate.professionalRole.name}`, id: candidate.userId, label: fullName(candidate) }));

  useEffect(() => {
    if (normalizedQuery.length < 2 || selected) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setSearching(true);
      setSearchError("");
      void searchProjectMemberCandidates(projectId, normalizedQuery, controller.signal).then((value) => setCandidates(value.items)).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setSearchError(reason instanceof Error ? reason.message : "Не удалось выполнить поиск пользователей");
      }).finally(() => { if (!controller.signal.aborted) setSearching(false); });
    }, 300);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [normalizedQuery, projectId, selected]);

  function changeQuery(value: string) {
    setQuery(value);
    setSelected(null);
    setCandidates([]);
    setSearchError("");
    setSubmitError("");
  }

  async function add() {
    if (!selected) { setSubmitError("Выберите пользователя из результатов поиска"); return; }
    setPending(true); setSubmitError("");
    try {
      const member = await addProjectMember(projectId, selected.userId);
      onAdded(member);
      setQuery(""); setCandidates([]); setSelected(null);
    } catch (reason) {
      setSubmitError(reason instanceof Error ? reason.message : "Не удалось добавить участника");
    } finally { setPending(false); }
  }

  return <div data-cy="project-member-settings"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Команда проекта</p><h2 className="mt-3 font-display text-5xl">Участники</h2><p className="mt-4 text-sm leading-6 text-secondary">Найдите пользователя по логину или электронной почте. Новый участник будет добавлен как исполнитель.</p><div className="mt-7"><Autocomplete autoFocus dataCy="project-member-search" emptyText={normalizedQuery.length < 2 ? "Введите минимум 2 символа логина или почты" : "Пользователи не найдены. Проверьте запрос."} error={searchError || submitError} label="Пользователь" loading={searching} onSelect={(option) => { setSelected(option.candidate); setQuery(option.label); setSubmitError(""); }} onValueChange={changeQuery} options={options} placeholder="Логин или электронная почта" required value={query} /></div><Button className="mt-5" data-cy="add-project-member" disabled={pending || !selected} onClick={() => void add()} type="button">{pending ? "Добавляем…" : "Добавить участника"}</Button><div className="mt-8 border-t border-border pt-6"><h3 className="text-sm font-semibold">Уже в проекте</h3>{members.length === 0 ? <p className="mt-4 text-sm text-secondary">Список пока пуст.</p> : <ul className="mt-3 divide-y divide-border">{members.map((member) => <li className="flex items-center justify-between gap-4 py-3" key={member.userId}><span className="min-w-0"><span className="block truncate text-sm font-medium">{fullName(member)}</span><span className="mt-1 block truncate text-xs text-secondary">@{member.login} · {projectRolesLabel(member.projectRoles)}</span></span>{member.removable ? <IconButton aria-label={`Удалить участника ${fullName(member)}`} className="size-9 shrink-0 border-red-700 text-red-700 hover:bg-page" onClick={() => onRemove(member)} tooltip="Удалить из проекта" tooltipAlign="end"><TrashIcon /></IconButton> : <span className="shrink-0 text-xs text-secondary">Нельзя удалить</span>}</li>)}</ul>}</div><div className="mt-7"><Button className="border border-border bg-transparent text-primary hover:bg-page" onClick={onClose} type="button">Закрыть</Button></div></div>;
}

function RemoveMemberDialog({ member, onCancel, onRemoved, projectId }: Readonly<{ member: ProjectMember; onCancel: () => void; onRemoved: () => void; projectId: string }>) {
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  async function remove() { setPending(true); setError(""); try { await removeProjectMember(projectId, member.userId); onRemoved(); } catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось удалить участника"); setPending(false); } }
  return <div data-cy="remove-project-member-dialog"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-red-700">Изменение доступа</p><h2 className="mt-3 font-display text-5xl">Удалить участника?</h2><p className="mt-5 leading-7 text-secondary">{fullName(member)} потеряет доступ к проекту и связанным данным.</p>{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button className="bg-red-700 text-white hover:bg-red-800" data-cy="confirm-remove-project-member" disabled={pending} onClick={() => void remove()} type="button">{pending ? "Удаляем…" : "Удалить"}</Button><Button className="border border-border bg-transparent text-primary hover:bg-page" onClick={onCancel} type="button">Отмена</Button></div></div>;
}

function fullName(person: Pick<ProjectMemberCandidate, "firstName" | "lastName" | "middleName">) { return [person.firstName, person.middleName, person.lastName].filter(Boolean).join(" "); }
function projectRolesLabel(roles: ProjectRole[]) { return roles.map((role) => ({ customer: "Заказчик", project_admin: "Администратор проекта", executor: "Исполнитель" })[role]).join(" · "); }
function SettingsIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M9.6 3.4h4.8l.7 2.2 2 .8 2.1-1 2.4 4.2-1.7 1.5v2.3l1.7 1.5-2.4 4.2-2.1-1-2 .8-.7 2.2H9.6l-.7-2.2-2-.8-2.1 1-2.4-4.2 1.7-1.5v-2.3L2.4 9.6l2.4-4.2 2.1 1 2-.8.7-2.2Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.5"/><circle cx="12" cy="12.25" r="3" stroke="currentColor" strokeWidth="1.5"/></svg>; }
function TrashIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13M10 11v5m4-5v5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" /></svg>; }
