"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { focusField } from "@/shared/lib/focusField";
import { Button, DatePicker, Input, Select, Textarea } from "@/shared/ui";

import { createProject, ProjectApiError } from "../api/projects";

import { projectTypes } from "../model/projectTypes";

export function CreateProjectForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [type, setType] = useState<(typeof projectTypes)[number]["value"]>("interior_design");
  const [address, setAddress] = useState("");
  const [start, setStart] = useState("");
  const [finish, setFinish] = useState("");
  const [description, setDescription] = useState("");
  const [autoApprove, setAutoApprove] = useState(true);
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState("");
  const dateError = start && finish && start > finish ? "Дата начала не может быть позже даты завершения" : "";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (name.trim().length < 2) {
      setError("Укажите название проекта — минимум 2 символа");
      focusField(event.currentTarget, "#project-name");
      return;
    }
    if (dateError) {
      focusField(event.currentTarget, '[data-cy="project-finish"]');
      return;
    }
    setIsPending(true);
    try {
      await createProject({
        address: address.trim() || undefined,
        autoApproveExpenses: autoApprove,
        currencyCode: "RUB",
        customerUserId: null,
        description: description.trim() || undefined,
        name: name.trim(),
        plannedFinishOn: finish || undefined,
        plannedStartOn: start || undefined,
        type,
      });
      router.push("/account");
      router.refresh();
    } catch (reason) {
      if (reason instanceof ProjectApiError && reason.status === 401) {
		router.replace("/?auth=login");
        return;
      }
      setError(reason instanceof Error ? reason.message : "Не удалось создать проект");
    } finally {
      setIsPending(false);
    }
  }

  return (
    <form className="max-w-4xl" data-cy="create-project-form" noValidate onSubmit={submit}>
      <div className="border-b border-border pb-7">
        <Link className="inline-flex min-h-11 items-center gap-2 text-sm text-secondary hover:text-primary" href="/account"><span aria-hidden="true">←</span> К проектам</Link>
        <p className="mt-6 text-xs font-semibold uppercase tracking-[0.14em] text-action">Новый проект</p>
        <h1 className="mt-3 font-display text-5xl leading-none tablet:text-6xl">Создание проекта</h1>
        <p className="mt-4 max-w-2xl text-secondary">После создания вы сможете добавить участников, задачи и финансовые данные. Заказчик обычного аккаунта назначается автоматически; супер-администратор может назначить его позже.</p>
      </div>

      <div className="mt-9 grid gap-x-8 gap-y-7 tablet:grid-cols-2">
        <div className="grid gap-2 text-sm font-medium"><label htmlFor="project-name">Название проекта *</label><Input aria-describedby={error && name.trim().length < 2 ? "project-name-error" : undefined} aria-invalid={Boolean(error && name.trim().length < 2)} autoFocus id="project-name" data-cy="project-name" maxLength={200} onChange={(event) => setName(event.target.value)} placeholder="Например, Квартира на Полянке" required value={name} />{error && name.trim().length < 2 ? <p className="text-sm text-danger" data-cy="project-form-error" id="project-name-error" role="alert">{error}</p> : null}</div>
        <Select dataCy="project-type" label="Тип проекта" onValueChange={setType} options={projectTypes} required value={type} />
        <label className="grid gap-2 text-sm font-medium tablet:col-span-2">Адрес<Input data-cy="project-address" maxLength={500} onChange={(event) => setAddress(event.target.value)} placeholder="Город, улица, дом" value={address} /></label>
        <DatePicker dataCy="project-start" label="Планируемое начало" onValueChange={setStart} value={start} />
        <DatePicker dataCy="project-finish" error={dateError} label="Планируемое завершение" min={start || undefined} onValueChange={setFinish} value={finish} />
        <label className="grid gap-2 text-sm font-medium tablet:col-span-2">Описание<Textarea data-cy="project-description" maxLength={5000} onChange={(event) => setDescription(event.target.value)} placeholder="Кратко опишите объект, состав работ и важные особенности" value={description} /></label>
      </div>

      <label className="mt-8 flex cursor-pointer items-start gap-3 border border-border bg-surface p-5 text-sm"><input checked={autoApprove} className="mt-0.5 size-4 shrink-0 accent-action" data-cy="project-auto-approve" onChange={(event) => setAutoApprove(event.target.checked)} type="checkbox" /><span><strong className="block text-primary">Автосогласование расходов</strong><span className="mt-1 block leading-6 text-secondary">Новые расходы будут согласованы автоматически. Настройку можно изменить в финансах проекта.</span></span></label>

      {error && name.trim().length >= 2 ? <p className="mt-6 text-sm text-danger" data-cy="project-form-error" role="alert">{error}</p> : null}
      <div className="mt-8 flex flex-wrap gap-3"><Button data-cy="project-submit" disabled={isPending} type="submit">{isPending ? "Создаём…" : "Создать проект"}</Button><Link className="inline-flex min-h-11 items-center rounded-full border border-border px-5 text-sm font-medium hover:bg-surface" href="/account">Отмена</Link></div>
    </form>
  );
}
