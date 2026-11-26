"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { ThemeToggle } from "@/features/themeToggle";
import { Button } from "@/shared/ui";

import { loadCurrentAccount } from "../api/projects";
import type { CurrentAccount } from "../model/types";

export function AccountProfilePage() {
  const [account, setAccount] = useState<CurrentAccount | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    void loadCurrentAccount(controller.signal).then(setAccount, () => {
      if (!controller.signal.aborted) setError("Не удалось загрузить профиль");
    });
    return () => controller.abort();
  }, [attempt]);

  return (
    <section aria-labelledby="profile-title" className="max-w-2xl">
      <h1 className="font-display text-4xl tablet:text-5xl" id="profile-title">Настройки профиля</h1>
      {error ? (
        <div className="mt-6" role="alert">
          <p className="text-danger">{error}</p>
          <Button className="mt-3" onClick={() => { setError(""); setAttempt((value) => value + 1); }} type="button">Повторить</Button>
        </div>
      ) : account ? (
        <dl className="mt-6 grid gap-4 border border-border bg-surface p-5">
          <div><dt className="text-sm text-secondary">Имя</dt><dd>{[account.firstName, account.lastName, account.middleName].filter(Boolean).join(" ") || account.login}</dd></div>
          <div><dt className="text-sm text-secondary">Логин</dt><dd className="wrap-anywhere">{account.login}</dd></div>
          {account.email ? <div><dt className="text-sm text-secondary">Электронная почта</dt><dd className="wrap-anywhere">{account.email}</dd></div> : null}
          <div><dt className="text-sm text-secondary">Роль</dt><dd>{account.globalRole === "super_admin" ? "Супер-администратор" : account.globalRole === "technical_admin" ? "Технический администратор" : account.professionalRole?.name ?? "Участник"}</dd></div>
        </dl>
      ) : <p className="mt-6 text-secondary" role="status">Загружаем профиль…</p>}
      <div className="mt-6 flex flex-wrap items-center gap-4">
        <div className="flex items-center gap-2 text-sm"><span>Оформление</span><ThemeToggle /></div>
        <Link className="inline-flex min-h-11 items-center rounded-full border border-border px-5 text-sm hover:bg-surface" href="/?auth=forgot">Изменить пароль</Link>
      </div>
    </section>
  );
}
