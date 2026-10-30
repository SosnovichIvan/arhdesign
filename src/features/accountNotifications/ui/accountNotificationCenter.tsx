"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import { loadAccountNotifications, markAccountNotificationRead, markAllAccountNotificationsRead } from "../api/notifications";
import type { AccountNotificationList } from "../model/types";

export function AccountNotificationCenter() {
  const [data, setData] = useState<AccountNotificationList>({ items: [], unreadCount: 0 });
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState("");
  const previousUnread = useRef(0);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    try {
      const next = await loadAccountNotifications(signal);
      if (next.unreadCount > previousUnread.current && previousUnread.current > 0) setStatus("Появилось новое уведомление по проекту");
      previousUnread.current = next.unreadCount;
      setData(next);
    } catch (reason) {
      if (!(reason instanceof DOMException && reason.name === "AbortError")) setStatus("Не удалось обновить уведомления");
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => { if (!controller.signal.aborted) void refresh(controller.signal); });
    const interval = window.setInterval(() => void refresh(), 20000);
    const onFocus = () => void refresh();
    window.addEventListener("focus", onFocus);
    return () => { controller.abort(); window.clearInterval(interval); window.removeEventListener("focus", onFocus); };
  }, [refresh]);

  async function readOne(id: string) {
    setData((current) => ({ ...current, items: current.items.map((item) => item.id === id ? { ...item, readAt: item.readAt ?? new Date().toISOString() } : item), unreadCount: Math.max(0, current.unreadCount - (current.items.find((item) => item.id === id)?.readAt ? 0 : 1)) }));
    await markAccountNotificationRead(id).catch(() => void refresh());
  }

  async function readAll() {
    const now = new Date().toISOString();
    setData((current) => ({ items: current.items.map((item) => ({ ...item, readAt: item.readAt ?? now })), unreadCount: 0 }));
    await markAllAccountNotificationsRead().catch(() => void refresh());
  }

  return <div className="relative">
    <button aria-expanded={open} aria-haspopup="dialog" aria-label={data.unreadCount > 0 ? `Уведомления, непрочитанных: ${data.unreadCount}` : "Уведомления"} className="relative inline-flex size-11 items-center justify-center rounded-full border border-border hover:bg-surface focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action" onClick={() => setOpen((value) => !value)} type="button"><BellIcon />{data.unreadCount > 0 ? <span aria-hidden="true" className="absolute -right-1 -top-1 inline-flex min-w-5 items-center justify-center rounded-full bg-action px-1 text-[10px] font-semibold leading-5 text-inverse-text">{data.unreadCount > 99 ? "99+" : data.unreadCount}</span> : null}</button>
    <span aria-atomic="true" className="sr-only" role="status">{status}</span>
    {open ? <section aria-label="Уведомления" className="absolute right-0 top-[calc(100%+0.75rem)] z-50 w-[min(24rem,calc(100vw-2rem))] border border-border bg-surface shadow-2xl" role="dialog">
      <header className="flex items-center justify-between gap-4 border-b border-border px-4 py-3"><h2 className="text-sm font-semibold">Уведомления</h2>{data.unreadCount > 0 ? <button className="min-h-11 text-xs text-action underline" onClick={() => void readAll()} type="button">Прочитать все</button> : null}</header>
      <div className="max-h-[min(70vh,32rem)] overflow-y-auto">{data.items.length === 0 ? <p className="p-6 text-sm text-secondary">Новых событий пока нет.</p> : <ul className="divide-y divide-border">{data.items.map((item) => <li className={item.readAt ? "bg-surface" : "bg-action/5"} key={item.id}><Link className="block px-4 py-4 hover:bg-page" href={item.href} onClick={() => { setOpen(false); void readOne(item.id); }}><span className="flex items-start gap-3"><span aria-hidden="true" className={`mt-1.5 size-2 shrink-0 rounded-full ${item.readAt ? "bg-border" : "bg-action"}`} /><span className="min-w-0"><strong className="block text-sm font-medium">{item.title}</strong><span className="mt-1 line-clamp-2 block text-xs leading-5 text-secondary">{item.body}</span><time className="mt-2 block text-[10px] text-secondary" dateTime={item.createdAt}>{formatNotificationTime(item.createdAt)}</time></span></span></Link></li>)}</ul>}</div>
    </section> : null}
  </div>;
}

function formatNotificationTime(value: string) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", hour: "2-digit", minute: "2-digit", month: "short" }).format(new Date(value));
}

function BellIcon() { return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><path d="M5.5 8a4.5 4.5 0 0 1 9 0v3.2l1.5 2.3H4l1.5-2.3V8Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.4"/><path d="M8.2 15.5a2 2 0 0 0 3.6 0" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4"/></svg>; }
