"use client";

import { useEffect, useState } from "react";

import { getWithAccountSession } from "@/shared/lib/accountSession";
import { Button, Dialog, IconButton } from "@/shared/ui";

type ReleaseNote = {
  version: string;
  added: string[];
  fixed: { description: string; source: string }[];
};

export function ReleaseNotesDrawer() {
  const [open, setOpen] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [notes, setNotes] = useState<ReleaseNote[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    void getWithAccountSession("/api/v1/release-notes", controller.signal).then(async (response) => {
      if (!response.ok) throw new Error("Не удалось загрузить обновления");
      const data = await response.json() as ReleaseNote[];
      if (!controller.signal.aborted) setNotes(data);
    }).catch(() => {
      if (!controller.signal.aborted) setFailed(true);
    });
    return () => controller.abort();
  }, [attempt, open]);

  function openDrawer() {
    setNotes(null);
    setFailed(false);
    setOpen(true);
  }

  function retry() {
    setFailed(false);
    setAttempt((value) => value + 1);
  }

  return <>
    <IconButton aria-expanded={open} aria-haspopup="dialog" aria-label="Что нового" className="min-h-11 w-auto shrink-0 gap-2 rounded-full px-3 text-secondary hover:text-primary" onClick={openDrawer} tooltip="Что нового">
      <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 24 24">
        <path d="M10 20h4m-2-16a5 5 0 0 0-5 5v4l-2 3h14l-2-3V9m3-6v6m-3-3h6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.6" />
      </svg>
      <span className="hidden desktop:inline">Что нового</span>
    </IconButton>
    <Dialog closeLabel="Закрыть список обновлений" contentClassName="tablet:px-8" isOpen={open} label="Что нового" onClose={() => setOpen(false)} placement="right">
      <h2 className="font-display text-4xl text-primary">Что нового</h2>
      {failed ? <div className="mt-6 space-y-3" role="alert"><p>Не удалось загрузить обновления.</p><Button onClick={retry} variant="secondary">Повторить</Button></div> : notes === null ? <p className="mt-6 text-secondary" role="status">Загружаем обновления…</p> : null}
      {notes?.map(({ added, fixed, version }) => <article className="mt-6 border-t border-border py-5" key={version}>
        <h3 className="text-lg font-semibold text-primary">Версия {version}</h3>
        <h4 className="mt-5 text-sm font-semibold text-primary">Добавили</h4>
        <ul className="mt-2 list-disc space-y-2 pl-5 text-sm leading-relaxed text-secondary">{added.map((item) => <li key={item}>{item}</li>)}</ul>
        <h4 className="mt-5 text-sm font-semibold text-primary">Исправили</h4>
        <ul className="mt-2 list-disc space-y-3 pl-5 text-sm leading-relaxed text-secondary">{fixed.map(({ description, source }) => <li key={description}>{description}<span className="mt-1 block text-xs">Источник — {source}</span></li>)}</ul>
      </article>)}
    </Dialog>
  </>;
}
