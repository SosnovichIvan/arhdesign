"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { Button, Dialog, IconButton, Input } from "@/shared/ui";

import { openProjectContextChat } from "../api/projects";
import type { ProjectChatContextType } from "../model/types";

type ContextChatActionProps = {
  chatId: string | null;
  contextId: string;
  contextTitle: string;
  contextType: ProjectChatContextType;
  projectId: string;
};

export function ContextChatAction({ chatId, contextId, contextTitle, contextType, projectId }: Readonly<ContextChatActionProps>) {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);

  function open() {
    if (chatId) {
      router.push(contextChatHref(projectId, chatId));
      return;
    }
    setIsOpen(true);
  }

  return <>
    <IconButton aria-label={`Обсудить «${contextTitle}»`} className="size-9 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy={`discuss-${contextType}-${contextId}`} onClick={open} tooltip="Обсудить" tooltipAlign="end"><ChatIcon /></IconButton>
    <Dialog isOpen={isOpen} label={`Создание обсуждения «${contextTitle}»`} onClose={() => setIsOpen(false)}>
      <CreateContextChatForm contextId={contextId} contextTitle={contextTitle} contextType={contextType} onCancel={() => setIsOpen(false)} onCreated={(createdChatId) => router.push(contextChatHref(projectId, createdChatId))} projectId={projectId} />
    </Dialog>
  </>;
}

function CreateContextChatForm({ contextId, contextTitle, contextType, onCancel, onCreated, projectId }: Readonly<Omit<ContextChatActionProps, "chatId"> & { onCancel: () => void; onCreated: (chatId: string) => void }>) {
  const [name, setName] = useState(`Обсуждение: ${contextTitle}`.slice(0, 200));
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    const normalizedName = name.trim();
    if (normalizedName.length < 2) {
      setError("Укажите название обсуждения — минимум 2 символа");
      return;
    }
    setPending(true); setError("");
    try {
      const chat = await openProjectContextChat(projectId, contextType, contextId, normalizedName);
      onCreated(chat.chatId);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Не удалось создать обсуждение");
    } finally {
      setPending(false);
    }
  }

  return <form data-cy="context-chat-form" noValidate onSubmit={submit}>
    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Контекстный чат</p>
    <h2 className="mt-3 font-display text-4xl">Новое обсуждение</h2>
    <p className="mt-3 text-sm leading-6 text-secondary">Чат будет связан с записью «{contextTitle}». Его увидят только участники, у которых есть доступ к этой записи.</p>
    <label className="mt-7 grid gap-2 text-sm font-medium">Название обсуждения *<Input autoFocus data-cy="context-chat-name" data-dialog-initial-focus maxLength={200} onChange={(event) => setName(event.target.value)} value={name} /></label>
    {error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}
    <div className="mt-7 flex flex-wrap gap-3"><Button data-cy="create-context-chat" disabled={pending} type="submit">{pending ? "Создаём…" : "Создать обсуждение"}</Button><Button disabled={pending} onClick={onCancel} type="button" variant="secondary">Отмена</Button></div>
  </form>;
}

function contextChatHref(projectId: string, chatId: string) {
  return `/account/projects/${encodeURIComponent(projectId)}/chat/${encodeURIComponent(chatId)}`;
}

function ChatIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="M4 5h16v11H9l-5 4V5Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.5" /><path d="M8 9h8M8 12h5" stroke="currentColor" strokeLinecap="round" strokeWidth="1.5" /></svg>; }
