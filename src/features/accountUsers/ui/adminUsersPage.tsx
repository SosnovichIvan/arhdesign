"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { Button, Dialog, Input, Select } from "@/shared/ui";

import { AdminUsersApiError, changeAdminUserStatus, loadAdminUsers } from "../api/users";
import type { AccountStatus, AdminUser } from "../model/types";

const statusOptions = [
  { label: "Все статусы", value: "all" },
  { label: "Активные", value: "active" },
  { label: "Ожидают подтверждения", value: "pending_verification" },
  { label: "Отключённые", value: "disabled" },
] as const;

const statusLabels: Record<AccountStatus, string> = { active: "Активен", disabled: "Отключён", pending_verification: "Не подтверждён" };

export function AdminUsersPage() {
  const router = useRouter();
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [identifier, setIdentifier] = useState("");
  const [appliedIdentifier, setAppliedIdentifier] = useState("");
  const [status, setStatus] = useState<(typeof statusOptions)[number]["value"]>("all");
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<AdminUser | null>(null);
  const [isChanging, setIsChanging] = useState(false);

  const load = useCallback(async (cursor?: string) => {
    setError("");
    if (!cursor) setState("loading");
    try {
      const page = await loadAdminUsers({ cursor, identifier: appliedIdentifier, status: status === "all" ? undefined : status });
      setUsers((current) => cursor ? [...current, ...page.items] : page.items);
      setNextCursor(page.nextCursor);
      setState("ready");
    } catch (reason) {
      if (reason instanceof AdminUsersApiError && reason.status === 401) { router.replace("/?auth=login"); return; }
      if (reason instanceof AdminUsersApiError && reason.status === 403) { router.replace("/account"); return; }
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить пользователей");
      setState("error");
    }
  }, [appliedIdentifier, router, status]);

  useEffect(() => { const controller = new AbortController(); void loadAdminUsers({ identifier: appliedIdentifier, status: status === "all" ? undefined : status }, controller.signal).then((page) => { setUsers(page.items); setNextCursor(page.nextCursor); setState("ready"); }).catch((reason: unknown) => { if (reason instanceof DOMException && reason.name === "AbortError") return; if (reason instanceof AdminUsersApiError && reason.status === 401) router.replace("/?auth=login"); else if (reason instanceof AdminUsersApiError && reason.status === 403) router.replace("/account"); else { setError(reason instanceof Error ? reason.message : "Не удалось загрузить пользователей"); setState("error"); } }); return () => controller.abort(); }, [appliedIdentifier, router, status]);

  async function confirmStatusChange() {
    if (!selected) return;
    setIsChanging(true); setError("");
    try {
      const updated = await changeAdminUserStatus(selected, selected.status === "disabled" ? "restore" : "disable");
      setUsers((current) => current.map((user) => user.id === updated.id ? updated : user));
      setSelected(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Не удалось изменить доступ");
      setSelected(null);
    } finally { setIsChanging(false); }
  }

  return <section aria-labelledby="users-title" data-cy="admin-users"><div className="border-b border-border pb-7"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Администрирование</p><h1 className="mt-3 font-display text-5xl leading-none tablet:text-6xl" id="users-title">Пользователи</h1><p className="mt-4 max-w-2xl text-secondary">Проверяйте дату регистрации и последнюю активность, временно отключайте и восстанавливайте доступ к платформе.</p></div>
    <form className="mt-7 grid gap-4 tablet:grid-cols-[minmax(0,1fr)_280px_auto] tablet:items-end" onSubmit={(event) => { event.preventDefault(); setAppliedIdentifier(identifier.trim()); }}><label className="grid gap-2 text-sm font-medium">Точный логин или почта<Input data-cy="users-identifier" onChange={(event) => setIdentifier(event.target.value)} placeholder="sveta.design или name@example.com" type="search" value={identifier} /></label><Select dataCy="users-status" label="Статус" onValueChange={setStatus} options={statusOptions} value={status} /><Button type="submit">Найти</Button></form>
    {error ? <div className="mt-6 border border-red-700/40 bg-surface p-5 text-sm text-red-700" role="alert">{error}<Button className="mt-4" onClick={() => void load()} type="button" variant="secondary">Повторить</Button></div> : null}
    {state === "loading" ? <div aria-label="Загрузка пользователей" className="mt-8 h-72 animate-pulse border border-border bg-surface" role="status" /> : null}
    {state === "ready" && users.length === 0 ? <div className="mt-8 border border-border bg-surface p-10 text-center"><h2 className="font-display text-3xl">Пользователи не найдены</h2><p className="mt-3 text-secondary">Проверьте точный логин, почту или выбранный статус.</p></div> : null}
    {state === "ready" && users.length > 0 ? <div className="mt-8 overflow-x-auto border border-border bg-surface"><table className="w-full min-w-[920px] text-left text-sm"><thead className="border-b border-border text-xs uppercase tracking-[0.08em] text-secondary"><tr><th className="px-5 py-4">Пользователь</th><th className="px-5 py-4">Роль</th><th className="px-5 py-4">Статус</th><th className="px-5 py-4">Регистрация</th><th className="px-5 py-4">Последний вход</th><th className="px-5 py-4 text-right">Доступ</th></tr></thead><tbody>{users.map((user) => <UserRow key={user.id} onSelect={setSelected} user={user} />)}</tbody></table></div> : null}
    {nextCursor ? <Button className="mt-6" onClick={() => void load(nextCursor)} type="button" variant="secondary">Показать ещё</Button> : null}
    <Dialog isOpen={selected !== null} label={selected?.status === "disabled" ? "Восстановление доступа" : "Отключение доступа"} onClose={() => { if (!isChanging) setSelected(null); }}><div className="pr-10"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Права доступа</p><h2 className="mt-4 font-display text-4xl">{selected?.status === "disabled" ? "Восстановить пользователя?" : "Отключить пользователя?"}</h2><p className="mt-4 leading-7 text-secondary">{selected?.status === "disabled" ? `Пользователь ${selected.login} снова сможет войти в личный кабинет.` : `Все активные сеансы ${selected?.login ?? "пользователя"} будут немедленно завершены.`}</p><div className="mt-7 flex flex-wrap gap-3"><Button data-dialog-initial-focus disabled={isChanging} onClick={() => void confirmStatusChange()} type="button">{selected?.status === "disabled" ? "Восстановить" : "Отключить"}</Button><Button disabled={isChanging} onClick={() => setSelected(null)} type="button" variant="secondary">Отмена</Button></div></div></Dialog>
  </section>;
}

function UserRow({ onSelect, user }: Readonly<{ onSelect: (user: AdminUser) => void; user: AdminUser }>) {
  const name = [user.lastName, user.firstName, user.middleName].filter(Boolean).join(" ");
  return <tr className="border-b border-border last:border-b-0"><td className="px-5 py-4"><p className="font-medium text-primary">{name}</p><p className="mt-1 text-xs text-secondary">@{user.login} · {user.email}</p></td><td className="px-5 py-4 text-secondary">{user.globalRole === "super_admin" ? "Супер-администратор" : user.globalRole === "technical_admin" ? "Технический администратор" : user.professionalRole.name}</td><td className="px-5 py-4"><span className="rounded-full border border-border px-3 py-1 text-xs">{statusLabels[user.status]}</span></td><td className="px-5 py-4 text-secondary">{formatDate(user.registeredAt)}</td><td className="px-5 py-4 text-secondary">{user.lastInteractiveLoginAt ? formatDate(user.lastInteractiveLoginAt) : "Ещё не входил"}</td><td className="px-5 py-4 text-right"><Button aria-label={`${user.status === "disabled" ? "Восстановить" : "Отключить"} пользователя ${user.login}`} onClick={() => onSelect(user)} type="button" variant="secondary">{user.status === "disabled" ? "Восстановить" : "Отключить"}</Button></td></tr>;
}

function formatDate(value: string) { return new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)); }
