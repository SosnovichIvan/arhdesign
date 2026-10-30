"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";

import { Autocomplete, Button, Dialog, IconButton, Input, Textarea, type AutocompleteOption } from "@/shared/ui";

import { createGlobalChat, loadGlobalChat, loadGlobalChats, markGlobalChatRead, searchGlobalChatCandidates, sendGlobalChatMessage } from "../api/chats";
import type { GlobalChatCandidate, GlobalChatConversation, GlobalChatKind, GlobalChatMessage, GlobalChatSummary } from "../model/types";

type LoadState = "error" | "loading" | "ready";

export function GlobalChatsPage() {
  const [chats, setChats] = useState<GlobalChatSummary[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [conversation, setConversation] = useState<GlobalChatConversation | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [conversationState, setConversationState] = useState<LoadState>("ready");
  const [error, setError] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const refreshChats = useCallback(async (signal?: AbortSignal) => {
    const page = await loadGlobalChats(signal);
    setChats(page.items);
    setSelectedId((current) => current ?? page.items[0]?.id ?? null);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    queueMicrotask(() => {
      setState("loading"); setError("");
      void refreshChats(controller.signal).then(() => setState("ready")).catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "Не удалось загрузить чаты"); setState("error");
      });
    });
    return () => controller.abort();
  }, [refreshChats, reloadKey]);

  const refreshConversation = useCallback(async (chatId: string, signal?: AbortSignal, quiet = false) => {
    if (!quiet) setConversationState("loading");
    try {
      const value = await loadGlobalChat(chatId, undefined, signal);
      setConversation((current) => quiet && current?.chat.id === chatId ? { ...value, messages: mergeMessages(current.messages, value.messages) } : value);
      setChats((current) => current.map((chat) => chat.id === chatId ? { ...value.chat, unreadCount: 0 } : chat));
      setConversationState("ready");
      void markGlobalChatRead(chatId).catch(() => undefined);
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      if (!quiet) { setError(reason instanceof Error ? reason.message : "Не удалось открыть чат"); setConversationState("error"); }
    }
  }, []);

  useEffect(() => {
    if (!selectedId) return;
    const controller = new AbortController();
    queueMicrotask(() => void refreshConversation(selectedId, controller.signal));
    return () => controller.abort();
  }, [refreshConversation, selectedId]);

  useEffect(() => {
    if (!selectedId || conversationState !== "ready") return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void Promise.all([refreshConversation(selectedId, undefined, true), refreshChats()]).catch(() => undefined);
    }, 10_000);
    return () => window.clearInterval(timer);
  }, [conversationState, refreshChats, refreshConversation, selectedId]);

  function onCreated(chat: GlobalChatSummary) {
    setChats((current) => [chat, ...current.filter((item) => item.id !== chat.id)]);
    setSelectedId(chat.id); setCreateOpen(false);
  }

  return <section aria-labelledby="global-chats-title" data-cy="global-chats-page">
    <header className="flex flex-wrap items-end justify-between gap-5">
      <div><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Рабочее пространство</p><h1 className="mt-2 font-display text-4xl leading-none tablet:text-5xl" id="global-chats-title">Чаты</h1><p className="mt-3 max-w-2xl text-sm leading-6 text-secondary">Личные и групповые обсуждения с участниками платформы.</p></div>
      <IconButton aria-label="Создать чат" className="border-icon-button-border bg-icon-button text-icon-button-text hover:border-action hover:bg-icon-button-hover" data-cy="create-global-chat" onClick={() => setCreateOpen(true)} tooltip="Создать чат"><PlusIcon /></IconButton>
    </header>

    {state === "loading" ? <div aria-label="Загрузка чатов" className="mt-8 h-[620px] animate-pulse border border-border bg-surface" role="status" /> : null}
    {state === "error" ? <div className="mt-8 border border-red-700/40 bg-surface p-7" role="alert"><h2 className="font-display text-3xl">Чаты не загрузились</h2><p className="mt-3 text-sm text-secondary">{error}</p><Button className="mt-5" onClick={() => setReloadKey((value) => value + 1)} variant="secondary">Повторить</Button></div> : null}
    {state === "ready" ? <div className="mt-8 grid min-h-[620px] overflow-hidden border border-border bg-surface desktop:grid-cols-[340px_minmax(0,1fr)]">
      <aside className="border-b border-border desktop:border-b-0 desktop:border-r" aria-label="Список чатов">
        {chats.length === 0 ? <div className="px-6 py-14 text-center" data-cy="global-chats-empty"><ChatsIcon /><h2 className="mt-4 font-display text-3xl">Начните общение</h2><p className="mt-3 text-sm leading-6 text-secondary">Создайте личный или групповой чат по логину или электронной почте.</p><Button className="mt-6" onClick={() => setCreateOpen(true)}>Создать чат</Button></div> : <ul className="max-h-[260px] overflow-y-auto desktop:max-h-[720px]">{chats.map((chat) => <li className="border-b border-border" key={chat.id}><button aria-current={selectedId === chat.id ? "true" : undefined} className="grid w-full grid-cols-[minmax(0,1fr)_auto] gap-2 px-5 py-4 text-left hover:bg-page aria-[current=true]:bg-action/15" data-cy={`global-chat-${chat.id}`} onClick={() => setSelectedId(chat.id)} type="button"><span className="truncate text-sm font-medium">{chat.displayName}</span><time className="text-[11px] text-secondary" dateTime={chat.lastActivityAt}>{formatListTime(chat.lastActivityAt)}</time><span className="truncate text-xs text-secondary">{chat.lastMessage?.body ?? (chat.kind === "group" ? `${chat.members.length} участников` : "Нет сообщений")}</span>{chat.unreadCount > 0 ? <span aria-label={`Непрочитанных сообщений: ${chat.unreadCount}`} className="min-w-5 rounded-full bg-action px-1.5 text-center text-[11px] font-semibold leading-5 text-inverse-text">{chat.unreadCount}</span> : null}</button></li>)}</ul>}
      </aside>
      <div className="min-w-0">{selectedId ? <ConversationPanel conversation={conversation} error={error} key={selectedId} onChange={setConversation} onRetry={() => void refreshConversation(selectedId)} state={conversationState} /> : chats.length > 0 ? <div className="grid min-h-[420px] place-items-center p-8 text-center text-sm text-secondary">Выберите чат слева</div> : null}</div>
    </div> : null}
    <Dialog isOpen={createOpen} label="Создание чата" onClose={() => setCreateOpen(false)}><CreateChatForm onCancel={() => setCreateOpen(false)} onCreated={onCreated} /></Dialog>
  </section>;
}

function ConversationPanel({ conversation, error, onChange, onRetry, state }: Readonly<{ conversation: GlobalChatConversation | null; error: string; onChange: (value: GlobalChatConversation) => void; onRetry: () => void; state: LoadState }>) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState("");
  const [failedId, setFailedId] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  useEffect(() => { if (conversation) requestAnimationFrame(() => bottomRef.current?.scrollIntoView({ block: "end" })); }, [conversation?.messages.length, conversation]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const body = draft.trim();
    if (!conversation || !body || sending || !conversation.canSend) return;
    if (body.length > 5000) { setSendError("Сообщение не должно превышать 5000 символов"); return; }
    const clientMessageId = failedId ?? crypto.randomUUID();
    setSending(true); setSendError(""); setFailedId(clientMessageId);
    try {
      const message = await sendGlobalChatMessage(conversation.chat.id, body, clientMessageId);
      onChange({ ...conversation, messages: mergeMessages(conversation.messages, [message]) });
      setDraft(""); setFailedId(null);
      requestAnimationFrame(() => bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" }));
    } catch (reason) { setSendError(reason instanceof Error ? reason.message : "Не удалось отправить сообщение"); }
    finally { setSending(false); }
  }

  if (state === "loading") return <div aria-label="Загрузка переписки" className="h-full min-h-[520px] animate-pulse bg-page/40" role="status" />;
  if (state === "error" || !conversation) return <div className="grid min-h-[520px] place-items-center p-8 text-center" role="alert"><div><h2 className="font-display text-3xl">Чат не открылся</h2><p className="mt-3 text-sm text-secondary">{error}</p><Button className="mt-5" onClick={onRetry} variant="secondary">Повторить</Button></div></div>;
  return <div className="flex min-h-[620px] flex-col" data-cy="global-chat-conversation">
    <header className="border-b border-border px-5 py-5 tablet:px-7"><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">{conversation.chat.kind === "group" ? "Групповой чат" : "Личный чат"}</p><h2 className="mt-1 text-xl font-medium">{conversation.chat.displayName}</h2><p className="mt-1 text-xs text-secondary">{conversation.chat.members.length} {pluralMembers(conversation.chat.members.length)}</p></header>
    <div aria-live="polite" aria-relevant="additions" className="max-h-[540px] min-h-[350px] flex-1 overflow-y-auto px-5 py-6 tablet:px-7" data-cy="global-chat-history" role="log">{conversation.messages.length === 0 ? <div className="grid min-h-[300px] place-items-center text-center"><div><ChatsIcon /><h3 className="mt-4 text-lg font-medium">Сообщений пока нет</h3><p className="mt-2 text-sm text-secondary">Отправьте первое сообщение.</p></div></div> : <ol className="grid gap-5">{conversation.messages.map((message) => <MessageItem currentUserId={conversation.currentUserId} key={message.id} message={message} />)}</ol>}<div ref={bottomRef} /></div>
    <form className="border-t border-border p-4 tablet:p-5" onSubmit={submit}><label className="sr-only" htmlFor="global-chat-message">Сообщение</label><div className="flex items-end gap-3 border border-border bg-page px-3 py-2 focus-within:border-field-focus"><Textarea className="min-h-16 flex-1 resize-none border-0 px-0 py-2" data-cy="global-chat-message" disabled={!conversation.canSend || sending} id="global-chat-message" maxLength={5000} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event: KeyboardEvent<HTMLTextAreaElement>) => { if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit(); } }} placeholder="Напишите сообщение" value={draft} /><IconButton aria-label="Отправить сообщение" className="shrink-0 border-icon-button-border bg-icon-button text-icon-button-text hover:border-action" data-cy="send-global-chat-message" disabled={!draft.trim() || sending || !conversation.canSend} tooltip="Отправить" type="submit"><SendIcon /></IconButton></div>{sendError ? <p className="mt-2 text-sm text-red-700" role="alert">{sendError}</p> : null}</form>
  </div>;
}

function CreateChatForm({ onCancel, onCreated }: Readonly<{ onCancel: () => void; onCreated: (chat: GlobalChatSummary) => void }>) {
  const [kind, setKind] = useState<GlobalChatKind>("direct");
  const [name, setName] = useState("");
  const [query, setQuery] = useState("");
  const [options, setOptions] = useState<GlobalChatCandidate[]>([]);
  const [selected, setSelected] = useState<GlobalChatCandidate[]>([]);
  const [searching, setSearching] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const value = query.trim();
    if (value.length < 2) { queueMicrotask(() => setOptions([])); return; }
    const controller = new AbortController();
    const timer = window.setTimeout(() => { setSearching(true); void searchGlobalChatCandidates(value, controller.signal).then((items) => setOptions(items.filter((item) => !selected.some((value) => value.userId === item.userId)))).catch(() => setOptions([])).finally(() => setSearching(false)); }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [query, selected]);

  const minimum = kind === "direct" ? 1 : 2;
  async function submit(event: FormEvent) {
    event.preventDefault(); setError("");
    if (selected.length !== 1 && kind === "direct") { setError("Для личного чата выберите одного пользователя"); return; }
    if (kind === "group" && selected.length < 2) { setError("Для группового чата выберите не менее двух пользователей"); return; }
    if (kind === "group" && name.trim().length < 2) { setError("Введите название группового чата"); return; }
    setSubmitting(true);
    try { onCreated(await createGlobalChat({ kind, memberUserIds: selected.map((item) => item.userId), ...(kind === "group" ? { name: name.trim() } : {}) })); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Не удалось создать чат"); }
    finally { setSubmitting(false); }
  }
  const autocompleteOptions: (AutocompleteOption & { candidate: GlobalChatCandidate })[] = options.map((item) => ({ candidate: item, id: item.userId, label: displayPerson(item), description: `@${item.login} · ${item.email}` }));
  return <form data-cy="create-global-chat-form" onSubmit={submit}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Новый разговор</p><h2 className="mt-3 pr-10 font-display text-4xl">Создать чат</h2><div aria-label="Тип чата" className="mt-7 flex gap-2">{([['direct', 'Личный'], ['group', 'Групповой']] as const).map(([value, label]) => <button aria-pressed={kind === value} className="min-h-11 border border-border px-5 text-sm aria-pressed:border-action aria-pressed:bg-action/15" key={value} onClick={() => { setKind(value); setSelected((items) => value === "direct" ? items.slice(0, 1) : items); setError(""); }} type="button">{label}</button>)}</div>{kind === "group" ? <label className="mt-6 grid gap-2 text-sm font-medium">Название чата *<Input data-cy="global-chat-name" maxLength={200} onChange={(event) => setName(event.target.value)} placeholder="Например, Команда проекта" value={name} /></label> : null}<div className="mt-6"><Autocomplete dataCy="global-chat-user-search" emptyText={query.trim().length < 2 ? "Введите минимум два символа" : "Пользователи не найдены"} label={kind === "direct" ? "Собеседник" : "Участники"} loading={searching} onSelect={(option) => { setSelected((items) => kind === "direct" ? [option.candidate] : [...items, option.candidate]); setQuery(""); }} onValueChange={setQuery} options={autocompleteOptions} placeholder="Логин или электронная почта" required value={query} /></div>{selected.length > 0 ? <ul aria-label="Выбранные пользователи" className="mt-4 flex flex-wrap gap-2">{selected.map((item) => <li className="inline-flex items-center gap-2 rounded-full border border-border bg-page py-1 pl-3 pr-1 text-sm" key={item.userId}><span>{displayPerson(item)}</span><button aria-label={`Удалить ${displayPerson(item)}`} className="inline-flex size-8 items-center justify-center rounded-full hover:bg-surface" onClick={() => setSelected((items) => items.filter((value) => value.userId !== item.userId))} type="button">×</button></li>)}</ul> : <p className="mt-3 text-xs text-secondary">Выберите {minimum === 1 ? "одного пользователя" : "минимум двух пользователей"}.</p>}{error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-8 flex flex-wrap gap-3"><Button data-cy="submit-global-chat" disabled={submitting} type="submit">{submitting ? "Создаём…" : "Создать"}</Button><Button onClick={onCancel} type="button" variant="secondary">Отмена</Button></div></form>;
}

function MessageItem({ currentUserId, message }: Readonly<{ currentUserId: string; message: GlobalChatMessage }>) { const mine = message.author.userId === currentUserId; return <li className={`flex ${mine ? "justify-end" : "justify-start"}`}><article className={`max-w-[82%] px-4 py-3 ${mine ? "bg-primary text-inverse-text" : "border border-border bg-page"}`}><p className={`text-xs font-semibold ${mine ? "text-inverse-text/70" : "text-action"}`}>{mine ? "Вы" : [message.author.firstName, message.author.lastName].filter(Boolean).join(" ") || `@${message.author.login}`}</p><p className="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{message.body}</p><time className={`mt-2 block text-[10px] ${mine ? "text-inverse-text/60" : "text-secondary"}`} dateTime={message.createdAt}>{formatMessageTime(message.createdAt)}</time></article></li>; }
function mergeMessages(left: GlobalChatMessage[], right: GlobalChatMessage[]) { return [...new Map([...left, ...right].map((item) => [item.id, item])).values()].sort((a, b) => a.createdAt.localeCompare(b.createdAt)); }
function displayPerson(person: Pick<GlobalChatCandidate, "firstName" | "lastName" | "login">) { return [person.firstName, person.lastName].filter(Boolean).join(" ") || `@${person.login}`; }
function formatListTime(value: string) { const date = new Date(value); const sameDay = date.toDateString() === new Date().toDateString(); return new Intl.DateTimeFormat("ru-RU", sameDay ? { hour: "2-digit", minute: "2-digit" } : { day: "2-digit", month: "2-digit" }).format(date); }
function formatMessageTime(value: string) { return new Intl.DateTimeFormat("ru-RU", { day: "2-digit", hour: "2-digit", minute: "2-digit", month: "short" }).format(new Date(value)); }
function pluralMembers(value: number) { const mod10 = value % 10; const mod100 = value % 100; return mod10 === 1 && mod100 !== 11 ? "участник" : mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) ? "участника" : "участников"; }
function PlusIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="M8 3v10M3 8h10" stroke="currentColor" strokeLinecap="round" strokeWidth="1.5" /></svg>; }
function SendIcon() { return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d="m2 3 12 5-12 5 2-5-2-5Zm2 5h10" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.3" /></svg>; }
function ChatsIcon() { return <svg aria-hidden="true" className="mx-auto size-10 text-action" fill="none" viewBox="0 0 40 40"><path d="M7 9h26v18H17l-8 6v-6H7z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.6" /></svg>; }
