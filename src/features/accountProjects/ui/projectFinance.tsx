"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { Button, DatePicker, Dialog, IconButton, Input, Select, Textarea } from "@/shared/ui";

import { createProjectExpense, loadProjectFinanceSummary } from "../api/projects";
import type { ExpenseCategory, ProjectExpense, ProjectFinanceSummary } from "../model/types";

const categories = [
  { label: "Материалы", value: "materials" }, { label: "Мебель", value: "furniture" },
  { label: "Работы подрядчика", value: "contractor" }, { label: "Доставка", value: "delivery" },
  { label: "Монтаж", value: "installation" }, { label: "Проектирование", value: "design" },
  { label: "Другое", value: "other" },
] as const;

export function ProjectFinance({ autoApproveExpenses, projectId }: Readonly<{ autoApproveExpenses: boolean; projectId: string }>) {
  const [summary, setSummary] = useState<ProjectFinanceSummary | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [notice, setNotice] = useState("");

  async function refresh(signal?: AbortSignal) {
    setState("loading"); setError("");
    try { setSummary(await loadProjectFinanceSummary(projectId, signal)); setState("ready"); }
    catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить финансовую сводку"); setState("error");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    void loadProjectFinanceSummary(projectId, controller.signal).then((value) => { setSummary(value); setState("ready"); }).catch((reason: unknown) => {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "Не удалось загрузить финансовую сводку"); setState("error");
    });
    return () => controller.abort();
  }, [projectId]);

  return <section aria-labelledby="project-finance-title" className="mt-5 border border-border bg-surface p-5" data-cy="project-finance-summary">
    <header className="flex flex-col gap-4 tablet:flex-row tablet:items-start tablet:justify-between">
      <div className="min-w-0"><h2 className="text-xl font-medium" id="project-finance-title">Финансовая сводка</h2><p className="mt-1 text-xs text-secondary">Подтверждённые операции и расходы, которые ещё находятся в работе</p></div>
      <div className="flex shrink-0 flex-nowrap items-center gap-2">
        {state === "ready" && summary?.canCreateExpense ? <IconButton aria-label="Добавить расход" className="size-9 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="add-project-expense" onClick={() => { setNotice(""); setFormOpen(true); }} tooltip="Добавить расход" tooltipAlign="end"><PlusIcon /></IconButton> : null}
        <Link className="inline-flex min-h-9 whitespace-nowrap items-center justify-center rounded-full border border-border px-4 text-sm font-medium transition-colors hover:border-action hover:bg-page" data-cy="go-to-project-expenses" href={`/account/projects/${encodeURIComponent(projectId)}/finances`}>Перейти в расходы</Link>
      </div>
    </header>
    {state === "loading" ? <div aria-label="Загрузка финансовой сводки" className="mt-6 h-20 animate-pulse bg-page" role="status" /> : null}
    {state === "error" ? <div className="mt-6 border border-red-700/40 p-4" role="alert"><p className="text-sm text-red-700">{error}</p><Button className="mt-4" onClick={() => void refresh()} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" && summary ? <><div className="mt-6 grid gap-6 tablet:grid-cols-2 desktop:grid-cols-4"><FinanceMetric label="Поступления заказчика" value={formatMoney(summary.confirmedIncomeMinor, summary.currencyCode)} /><FinanceMetric label="Подтверждённые расходы" value={formatMoney(summary.confirmedExpenseMinor, summary.currencyCode)} /><FinanceMetric label="Остаток средств" value={formatMoney(summary.availableBalanceMinor, summary.currencyCode)} /><FinanceMetric label="Расходы в работе" value={formatMoney(summary.pendingExpenseMinor, summary.currencyCode)} /></div><p className="mt-6 border-t border-border pt-4 text-xs leading-5 text-secondary">Новые расходы не уменьшают остаток до подтверждения фактической оплаты.</p></> : null}
    {notice ? <p className="mt-4 text-sm text-action" role="status">{notice}</p> : null}
    <Dialog isOpen={formOpen} label="Добавление расхода" onClose={() => setFormOpen(false)}><ExpenseForm autoApproveExpenses={autoApproveExpenses} onCancel={() => setFormOpen(false)} onCreated={(expense) => { setFormOpen(false); setNotice(expense.status === "auto_approved" ? "Расход добавлен и автосогласован. Он ожидает оплаты." : "Расход добавлен и ожидает согласования."); void refresh(); }} projectId={projectId} /></Dialog>
  </section>;
}

export function ExpenseForm({ autoApproveExpenses, onCancel, onCreated, projectId }: Readonly<{ autoApproveExpenses: boolean; onCancel: () => void; onCreated: (expense: ProjectExpense) => void; projectId: string }>) {
  const [amount, setAmount] = useState("");
  const [category, setCategory] = useState<ExpenseCategory>("materials");
  const [description, setDescription] = useState("");
  const [vendor, setVendor] = useState("");
  const [plannedPaymentOn, setPlannedPaymentOn] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const errorRef = useRef<HTMLDivElement>(null);
  const idempotencyKey = useRef<string | null>(null);

  useEffect(() => { if (error) errorRef.current?.focus(); }, [error]);

  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    const amountMinor = parseMoney(amount);
    if (amountMinor === null) { setError("Укажите сумму больше нуля, не более двух знаков после запятой"); return; }
    if (description.trim().length < 2) { setError("Добавьте описание расхода — минимум 2 символа"); return; }
    idempotencyKey.current ??= createIdempotencyKey();
    setPending(true);
    try {
      const expense = await createProjectExpense(projectId, { amountMinor, category, description: description.trim(), plannedPaymentOn: plannedPaymentOn || null, vendorName: vendor.trim() || null }, idempotencyKey.current);
      onCreated(expense);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось добавить расход"); }
    finally { setPending(false); }
  }

  return <form data-cy="create-expense-form" noValidate onSubmit={submit}>
    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Финансы проекта</p><h2 className="mt-3 font-display text-5xl">Новый расход</h2>
    <p className="mt-4 text-sm leading-6 text-secondary">{autoApproveExpenses ? "В проекте включено автосогласование: запрос сразу перейдёт к ожиданию оплаты." : "После создания запрос будет ожидать согласования заказчиком или администратором."}</p>
    {error ? <div className="mt-5 border border-red-700/40 p-3 text-sm text-red-700" ref={errorRef} role="alert" tabIndex={-1}>{error}</div> : null}
    <div className="mt-7 grid gap-6 tablet:grid-cols-2">
      <label className="grid gap-2 text-sm font-medium">Сумма, ₽ *<Input autoFocus data-cy="expense-amount" inputMode="decimal" onChange={(event) => { setAmount(event.target.value); idempotencyKey.current = null; }} placeholder="0,00" value={amount} /></label>
      <Select dataCy="expense-category" label="Статья расхода" onValueChange={(value) => { setCategory(value); idempotencyKey.current = null; }} options={categories} required value={category} />
      <label className="grid gap-2 text-sm font-medium tablet:col-span-2">Описание *<Textarea data-cy="expense-description" maxLength={2000} onChange={(event) => { setDescription(event.target.value); idempotencyKey.current = null; }} value={description} /></label>
      <label className="grid gap-2 text-sm font-medium">Поставщик или подрядчик<Input data-cy="expense-vendor" maxLength={200} onChange={(event) => { setVendor(event.target.value); idempotencyKey.current = null; }} value={vendor} /></label>
      <DatePicker label="Плановая дата оплаты" onValueChange={(value) => { setPlannedPaymentOn(value); idempotencyKey.current = null; }} value={plannedPaymentOn} />
    </div>
    <div className="mt-7 flex flex-wrap gap-3"><Button data-cy="submit-project-expense" disabled={pending} type="submit">{pending ? "Добавляем…" : "Добавить расход"}</Button><Button onClick={onCancel} type="button" variant="secondary">Отмена</Button></div>
  </form>;
}

function FinanceMetric({ label, value }: Readonly<{ label: string; value: string }>) { return <div><p className="text-[10px] font-semibold uppercase tracking-[0.08em] text-secondary">{label}</p><p className="mt-2 font-display text-3xl tabular-nums">{value}</p></div>; }
export function formatMoney(valueMinor: number, currencyCode: string) { return new Intl.NumberFormat("ru-RU", { currency: currencyCode, maximumFractionDigits: 2, minimumFractionDigits: 2, style: "currency" }).format(valueMinor / 100); }
export function parseMoney(value: string) { const normalized = value.trim().replace(/\s/g, "").replace(",", "."); if (!/^\d{1,12}(?:\.\d{1,2})?$/.test(normalized)) return null; const [whole, fraction = ""] = normalized.split("."); const minor = Number(whole) * 100 + Number(fraction.padEnd(2, "0")); return Number.isSafeInteger(minor) && minor > 0 ? minor : null; }
function createIdempotencyKey() { return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`; }
function PlusIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>; }
