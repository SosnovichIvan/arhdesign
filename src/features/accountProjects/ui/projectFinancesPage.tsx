"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/shared/ui";

import { loadProject, loadProjectExpenses, ProjectApiError } from "../api/projects";
import type { ProjectExpense, ProjectView } from "../model/types";
import { ContextChatAction } from "./contextChatAction";
import { formatMoney } from "./projectFinance";

export function ProjectFinancesPage({ projectId }: Readonly<{ projectId: string }>) {
  const [project, setProject] = useState<ProjectView | null>(null);
  const [expenses, setExpenses] = useState<ProjectExpense[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  async function load(signal?: AbortSignal) {
    setState("loading"); setError("");
    try { const [nextProject, list] = await Promise.all([loadProject(projectId, signal), loadProjectExpenses(projectId, signal)]); setProject(nextProject); setExpenses(list.items); setState("ready"); }
    catch (reason) { if (reason instanceof DOMException && reason.name === "AbortError") return; setError(reason instanceof ProjectApiError && reason.status === 404 ? "Проект не найден или у вас нет к нему доступа" : reason instanceof Error ? reason.message : "Не удалось загрузить расходы"); setState("error"); }
  }
  useEffect(() => {
    void Promise.all([loadProject(projectId), loadProjectExpenses(projectId)]).then(([nextProject, list]) => {
      setProject(nextProject); setExpenses(list.items); setState("ready");
    }).catch((reason: unknown) => {
      setError(reason instanceof ProjectApiError && reason.status === 404 ? "Проект не найден или у вас нет к нему доступа" : reason instanceof Error ? reason.message : "Не удалось загрузить расходы"); setState("error");
    });
  }, [projectId]);
  if (state === "loading") return <div aria-label="Загрузка расходов" className="h-80 animate-pulse border border-border bg-surface" role="status" />;
  if (state === "error" || !project) return <div className="border border-red-700/40 bg-surface p-8" role="alert"><h1 className="font-display text-4xl">Расходы не открылись</h1><p className="mt-3 text-secondary">{error}</p><Button className="mt-6" onClick={() => void load()}>Повторить</Button></div>;
  return <section aria-labelledby="project-expenses-title" data-cy="project-expenses-page">
    <nav aria-label="Хлебные крошки" className="text-xs text-secondary"><Link href={`/account/projects/${encodeURIComponent(project.id)}`}>{project.name}</Link><span aria-hidden="true" className="px-2">/</span><span aria-current="page">Финансы проекта</span></nav>
    <div className="mt-7"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Финансы проекта</p><h1 className="mt-3 font-display text-5xl" id="project-expenses-title">Расходы</h1><p className="mt-3 max-w-2xl text-sm leading-6 text-secondary">Запросы на расходы не уменьшают остаток, пока не подтверждена фактическая оплата.</p></div>
    {expenses.length === 0 ? <div className="mt-8 border border-border bg-surface p-8" data-cy="expenses-empty"><h2 className="text-xl font-medium">Расходов пока нет</h2><p className="mt-2 text-sm text-secondary">Добавьте первый расход из финансовой сводки проекта.</p><Link className="mt-5 inline-flex min-h-11 items-center rounded-full bg-primary px-5 text-sm font-medium text-inverse-text" href={`/account/projects/${encodeURIComponent(project.id)}`}>Вернуться к сводке</Link></div> : <div className="mt-8 overflow-x-auto border border-border bg-surface"><table className="w-full min-w-[780px] border-collapse text-left text-sm"><thead><tr className="border-b border-border text-xs uppercase tracking-[0.08em] text-secondary"><th className="p-4">Расход</th><th className="p-4">Поставщик</th><th className="p-4">Сумма</th><th className="p-4">Статус</th><th className="p-4">Дата</th><th className="p-4 text-right">Действия</th></tr></thead><tbody>{expenses.map((expense) => <tr className="border-b border-border last:border-b-0" key={expense.id}><td className="p-4 font-medium">{expense.description}<span className="mt-1 block text-xs font-normal text-secondary">{categoryLabel(expense.category)}</span></td><td className="p-4 text-secondary">{expense.vendorName || "Не указан"}</td><td className="p-4 font-medium tabular-nums">{formatMoney(expense.amountMinor, expense.currencyCode)}</td><td className="p-4"><span className="inline-flex border border-border px-2.5 py-1 text-xs">{expenseStatusLabel(expense.status)}</span></td><td className="p-4 text-secondary">{new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium" }).format(new Date(expense.createdAt))}</td><td className="p-4 text-right"><ContextChatAction chatId={expense.contextChatId} contextId={expense.id} contextTitle={expense.description} contextType="expense" projectId={expense.projectId} /></td></tr>)}</tbody></table></div>}
  </section>;
}

function categoryLabel(category: ProjectExpense["category"]) { return ({ materials: "Материалы", furniture: "Мебель", contractor: "Работы подрядчика", delivery: "Доставка", installation: "Монтаж", design: "Проектирование", other: "Другое" })[category]; }
function expenseStatusLabel(status: ProjectExpense["status"]) { return ({ draft: "Черновик", pending_approval: "Ожидает согласования", auto_approved: "Автосогласован", approved: "Согласован", rejected: "Отклонён", awaiting_payment: "Ожидает оплаты", paid: "Оплачен", cancelled: "Отменён" })[status]; }
