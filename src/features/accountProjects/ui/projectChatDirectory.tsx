"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/shared/ui";

import { loadProjects } from "../api/projects";
import type { ProjectView } from "../model/types";

export function ProjectChatDirectory() {
  const [projects, setProjects] = useState<ProjectView[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [cursor, setCursor] = useState<string | undefined>();
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

  useEffect(() => {
    const controller = new AbortController();
    void loadProjects(controller.signal, cursor).then((page) => {
      if (controller.signal.aborted) return;
      setProjects((current) => [...current, ...page.items.filter((item) => !current.some((project) => project.id === item.id))]);
      setNextCursor(page.nextCursor);
      setState("ready");
    }).catch(() => {
      if (!controller.signal.aborted) setState("error");
    });
    return () => controller.abort();
  }, [attempt, cursor]);

  return <section aria-labelledby="project-chats-title">
    <Link className="inline-flex min-h-11 items-center text-sm text-action underline" href="/account/chats">← Личные и групповые чаты</Link>
    <h1 className="mt-2 font-display text-4xl tablet:text-5xl" id="project-chats-title">Чаты проектов</h1>
    <p className="mt-3 text-sm text-secondary">Выберите проект, чтобы открыть доступные вам обсуждения.</p>
    {projects.length > 0 ? <ul className="mt-6 divide-y divide-border border border-border bg-surface">
      {projects.map((project) => <li key={project.id}><Link className="flex min-h-11 items-center justify-between gap-4 px-4 py-3 text-sm font-medium hover:bg-page" href={`/account/projects/${encodeURIComponent(project.id)}/chat`}><span className="wrap-anywhere">{project.name}</span><span aria-hidden="true">→</span></Link></li>)}
    </ul> : null}
    {state === "loading" ? <p className="mt-5 text-sm text-secondary" role="status">Загружаем проекты…</p> : null}
    {state === "error" ? <div className="mt-5 space-y-3" role="alert"><p>Не удалось загрузить проекты.</p><Button onClick={() => { setState("loading"); setAttempt((value) => value + 1); }} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" && projects.length === 0 ? <p className="mt-5 text-sm text-secondary">У вас пока нет доступных проектов.</p> : null}
    {state === "ready" && nextCursor ? <Button className="mt-5" onClick={() => { setState("loading"); setCursor(nextCursor); }} variant="secondary">Показать ещё</Button> : null}
  </section>;
}
