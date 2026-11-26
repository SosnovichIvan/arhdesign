"use client";

import { useEffect, useRef, useState } from "react";

import { getWithAccountSession } from "@/shared/lib/accountSession";
import { cn } from "@/shared/lib";
import { Button, Dialog, IconButton } from "@/shared/ui";

type ReleaseNote = {
  version: string;
  added: string[];
  fixed: { description: string; source: string }[];
};

function rememberOpening(key: string, revision: string) {
  try { localStorage.setItem(key, revision); } catch { /* Storage may be disabled. */ }
}

export function ReleaseNotesDrawer({ accountId }: Readonly<{ accountId: string }>) {
  const [open, setOpen] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [catalog, setCatalog] = useState<{ notes: ReleaseNote[]; revision: string | null } | null>(null);
  const [seenRevision, setSeenRevision] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const isOpen = useRef(false);
  const storageKey = `arhdesign:release-notes:seen:${accountId}`;
  const revision = catalog?.revision;
  const unread = Boolean(revision && revision !== seenRevision);

  useEffect(() => {
    let request: AbortController | undefined;
    function loadNotes() {
      request?.abort();
      const controller = new AbortController();
      request = controller;
      void getWithAccountSession("/api/v1/release-notes", controller.signal).then(async (response) => {
        if (!response.ok) throw new Error("Не удалось загрузить обновления");
        const notes: unknown = await response.json();
        if (!Array.isArray(notes)) throw new Error("Не удалось загрузить обновления");
        if (controller.signal.aborted) return;
        const nextRevision = notes.length ? response.headers.get("X-Release-Notes-Revision") : null;
        if (isOpen.current && nextRevision) {
          rememberOpening(storageKey, nextRevision);
          setSeenRevision(nextRevision);
        } else {
          try {
            const stored = localStorage.getItem(storageKey);
            if (stored !== null && stored !== nextRevision) localStorage.removeItem(storageKey);
            setSeenRevision(stored === nextRevision ? stored : null);
          } catch { /* Keep the in-memory state if storage is disabled. */ }
        }
        setCatalog({ notes, revision: nextRevision });
        setFailed(false);
      }).catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    }

    function onStorage(event: StorageEvent) {
      if (event.key === storageKey) setSeenRevision(event.newValue);
    }

    loadNotes();
    window.addEventListener("focus", loadNotes);
    window.addEventListener("storage", onStorage);
    return () => {
      request?.abort();
      window.removeEventListener("focus", loadNotes);
      window.removeEventListener("storage", onStorage);
    };
  }, [attempt, storageKey]);

  function openDrawer() {
    isOpen.current = true;
    setOpen(true);
    if (revision) {
      rememberOpening(storageKey, revision);
      setSeenRevision(revision);
    }
  }

  function closeDrawer() {
    isOpen.current = false;
    setOpen(false);
  }

  function retry() {
    setFailed(false);
    setAttempt((value) => value + 1);
  }

  return <>
    <IconButton aria-expanded={open} aria-haspopup="dialog" aria-label="Что нового" className="min-h-11 w-auto shrink-0 gap-2 rounded-full px-3 text-secondary hover:text-primary" data-unread={unread || undefined} onClick={openDrawer} tooltip={unread ? "Есть непрочитанные обновления" : "Что нового"} tooltipPlacement="bottom">
      <span aria-hidden="true" className="relative">
        <svg className={cn("size-5", unread && "motion-safe:animate-[release-bell-pulse_2.4s_ease-in-out_infinite]")} fill="none" viewBox="0 0 24 24">
          <path d="M10 20h4m-2-16a5 5 0 0 0-5 5v4l-2 3h14l-2-3V9m3-6v6m-3-3h6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" />
        </svg>
        {unread ? <span className="absolute -right-1 -top-1 size-2 rounded-full bg-action ring-2 ring-surface" /> : null}
      </span>
      <span className="hidden desktop:inline">Что нового</span>
    </IconButton>
    <Dialog closeLabel="Закрыть список обновлений" contentClassName="tablet:px-8" isOpen={open} label="Что нового" onClose={closeDrawer} placement="right">
      <h2 className="font-display text-4xl text-primary">Что нового</h2>
      {failed ? <div className="mt-6 space-y-3" role="alert"><p>Не удалось загрузить обновления.</p><Button onClick={retry} variant="secondary">Повторить</Button></div> : catalog === null ? <p className="mt-6 text-secondary" role="status">Загружаем обновления…</p> : null}
      {catalog?.notes.map(({ added, fixed, version }) => <article className="mt-6 border-t border-border py-5" key={version}>
        <h3 className="text-lg font-semibold text-primary">Версия {version}</h3>
        <h4 className="mt-5 text-sm font-semibold text-primary">Добавили</h4>
        <ul className="mt-2 list-disc space-y-2 pl-5 text-sm leading-relaxed text-secondary">{added.map((item) => <li key={item}>{item}</li>)}</ul>
        <h4 className="mt-5 text-sm font-semibold text-primary">Исправили</h4>
        <ul className="mt-2 list-disc space-y-3 pl-5 text-sm leading-relaxed text-secondary">{fixed.map(({ description, source }) => <li key={description}>{description}<span className="mt-1 block text-xs">Источник — {source}</span></li>)}</ul>
      </article>)}
    </Dialog>
  </>;
}
