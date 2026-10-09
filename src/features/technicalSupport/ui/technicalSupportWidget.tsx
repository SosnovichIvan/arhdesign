"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type FormEvent } from "react";

import { Button, Dialog, IconButton, Textarea } from "@/shared/ui";

import { sendTechnicalFeedback, type FeedbackCategory } from "../api/technicalSupport";

export function TechnicalSupportWidget() {
  const pathname = usePathname() ?? "/account";
  const [open, setOpen] = useState(false);
  const [category, setCategory] = useState<FeedbackCategory>("complaint");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const [sending, setSending] = useState(false);

  function close() { setOpen(false); setError(""); setSent(false); }
  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    const value = message.trim();
    if (value.length < 5) { setError("Опишите обращение хотя бы пятью символами"); return; }
    setSending(true);
    try { await sendTechnicalFeedback(category, value, pathname); setSent(true); setMessage(""); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось отправить обращение"); }
    finally { setSending(false); }
  }

  return <>
    <div className="fixed bottom-5 right-5 z-40 tablet:bottom-7 tablet:right-7"><IconButton aria-label="Сообщить о проблеме или предложить улучшение" className="size-12 border-action bg-primary text-inverse-text shadow-surface hover:bg-action" data-cy="technical-feedback-open" onClick={() => setOpen(true)} tooltip="Обратная связь" tooltipAlign="end"><SupportIcon /></IconButton></div>
    <Dialog isOpen={open} label="Обратная связь разработчику" onClose={close}>
      {sent ? <div data-cy="technical-feedback-success" role="status"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Сообщение отправлено</p><h2 className="mt-3 pr-10 font-display text-4xl">Спасибо за обратную связь</h2><p className="mt-4 text-sm leading-6 text-secondary">Технический администратор получил обращение в Telegram.</p><Button className="mt-7" onClick={close}>Закрыть</Button></div> : <form data-cy="technical-feedback-form" onSubmit={submit}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Связь с разработчиком</p><h2 className="mt-3 pr-10 font-display text-4xl">Жалоба или предложение</h2><p className="mt-4 text-sm leading-6 text-secondary">Сообщение и адрес текущего раздела будут переданы техническому администратору в Telegram.</p><div aria-label="Тип обращения" className="mt-6 flex gap-2">{([['complaint', 'Сообщить о проблеме'], ['suggestion', 'Предложить улучшение']] as const).map(([value, label]) => <button aria-pressed={category === value} className="min-h-11 border border-border px-4 text-sm aria-pressed:border-action aria-pressed:bg-action/15" key={value} onClick={() => setCategory(value)} type="button">{label}</button>)}</div><label className="mt-6 grid gap-2 text-sm font-medium" htmlFor="technical-feedback-message">Сообщение *<Textarea data-cy="technical-feedback-message" id="technical-feedback-message" maxLength={2000} onChange={(event) => setMessage(event.target.value)} placeholder="Опишите, что произошло или что можно улучшить" value={message} /></label><p className="mt-3 text-xs leading-5 text-secondary">Не указывайте пароли, банковские данные и другие секреты. Подробнее — в <Link className="underline decoration-action underline-offset-4" href="/privacy" target="_blank">политике обработки данных</Link>.</p>{error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex flex-wrap gap-3"><Button data-cy="technical-feedback-submit" disabled={sending} type="submit">{sending ? "Отправляем…" : "Отправить"}</Button><Button onClick={close} type="button" variant="secondary">Отмена</Button></div></form>}
    </Dialog>
  </>;
}

function SupportIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><path d="M3 4.5h14v10H8l-4 3v-3H3z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.4"/><path d="M7 8h6M7 11h4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4"/></svg>; }
