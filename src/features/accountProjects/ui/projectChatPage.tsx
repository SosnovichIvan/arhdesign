"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent } from "react";

import { Autocomplete, Button, Dialog, IconButton, Input, Textarea, type AutocompleteOption } from "@/shared/ui";

import { addProjectChatMember, createProjectChat, deleteProjectChat, loadProject, loadProjectChatMembers, loadProjectChats, loadProjectContextChat, loadProjectMembers, markProjectChatRead, ProjectApiError, removeProjectChatMember, sendProjectContextChatMessage, updateProjectChat } from "../api/projects";
import type { ProjectChatMemberList, ProjectChatMessage, ProjectChatPage as ChatPageData, ProjectChatSummary, ProjectMember, ProjectView } from "../model/types";

export function ProjectChatPage({ chatId, projectId }: Readonly<{ chatId?: string; projectId: string }>) {
  const [project, setProject] = useState<ProjectView | null>(null);
  const [chats, setChats] = useState<ProjectChatSummary[]>([]);
  const [selectedId, setSelectedId] = useState(chatId ?? "");
  const [chat, setChat] = useState<ChatPageData | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
	const [settingsChatId, setSettingsChatId] = useState("");
	const [deletingChatId, setDeletingChatId] = useState("");
  const [arrivalNotice, setArrivalNotice] = useState("");
  const [mobileDetail, setMobileDetail] = useState(Boolean(chatId));
  const bottomRef = useRef<HTMLDivElement>(null);

  const selected = useMemo(() => chats.find((item) => item.id === selectedId) ?? null, [chats, selectedId]);
	const settingsChat = useMemo(() => chats.find((item) => item.id === settingsChatId) ?? null, [chats, settingsChatId]);
  const loadConversation = useCallback(async (id: string, before?: string, signal?: AbortSignal) => {
    const page = await loadProjectContextChat(projectId, id, before, signal);
    if (!before && page.messages.length > 0) {
      await markProjectChatRead(projectId, id, page.messages.at(-1)!.id).catch(() => undefined);
    }
    return page;
  }, [projectId]);

  const loadWorkspace = useCallback(async (signal?: AbortSignal) => {
    setState("loading"); setError("");
    try {
      const [projectValue, list] = await Promise.all([loadProject(projectId, signal), loadProjectChats(projectId, signal)]);
      const nextID = chatId && list.items.some((item) => item.id === chatId) ? chatId : list.items[0]?.id ?? "";
      const page = nextID ? await loadConversation(nextID, undefined, signal) : null;
      setProject(projectValue); setChats(list.items); setSelectedId(nextID); setChat(page); setState("ready");
      requestAnimationFrame(() => bottomRef.current?.scrollIntoView({ block: "end" }));
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof ProjectApiError && reason.status === 404 ? "Проект или чат не найден" : reason instanceof Error ? reason.message : "Не удалось загрузить чаты проекта"); setState("error");
    }
  }, [chatId, loadConversation, projectId]);

  useEffect(() => { const controller = new AbortController(); queueMicrotask(() => { if (!controller.signal.aborted) void loadWorkspace(controller.signal); }); return () => controller.abort(); }, [loadWorkspace]);

  useEffect(() => {
    if (state !== "ready") return;
    const refresh = async () => {
      try {
        const list = await loadProjectChats(projectId);
        setChats(list.items.map((item) => item.id === selectedId ? { ...item, unreadCount: 0 } : item));
        if (!selectedId) return;
        const page = await loadConversation(selectedId);
        setChat((current) => {
          const known = new Set(current?.messages.map((message) => message.id) ?? []);
          const incoming = page.messages.filter((message) => !known.has(message.id) && message.author.userId !== page.currentUserId);
          if (incoming.length > 0) setArrivalNotice(incoming.length === 1 ? "В чат пришло новое сообщение" : `В чат пришло новых сообщений: ${incoming.length}`);
          return current ? { ...page, messages: mergeMessages(current.messages, page.messages) } : page;
        });
      } catch {
        // Background refresh keeps the last usable conversation visible.
      }
    };
    const interval = window.setInterval(() => void refresh(), 10000);
    const onFocus = () => void refresh();
    window.addEventListener("focus", onFocus);
    return () => { window.clearInterval(interval); window.removeEventListener("focus", onFocus); };
  }, [loadConversation, projectId, selectedId, state]);

  async function selectChat(id: string) {
    setSelectedId(id); setChat(null); setSendError(""); setMobileDetail(true);
    try { const page = await loadConversation(id); setChat(page); setChats((items) => items.map((item) => item.id === id ? { ...item, unreadCount: 0 } : item)); window.history.pushState(window.history.state, "", `/account/projects/${encodeURIComponent(projectId)}/chat/${encodeURIComponent(id)}`); }
    catch (reason) { setSendError(reason instanceof Error ? reason.message : "Не удалось открыть чат"); }
  }

	function openChatSettings(item: ProjectChatSummary) {
		setSettingsChatId(item.id);
		setSettingsOpen(true);
	}

	async function removeChatFromList(item: ProjectChatSummary) {
		if (!window.confirm(`Удалить чат «${item.name}»? История будет скрыта для всех участников.`)) return;
		setDeletingChatId(item.id);
		setError("");
		try {
			await deleteProjectChat(projectId, item.id, item.version);
			const remaining = chats.filter((chatItem) => chatItem.id !== item.id);
			setChats(remaining);
			if (settingsChatId === item.id) { setSettingsOpen(false); setSettingsChatId(""); }
			if (selectedId === item.id) {
				const next = remaining[0];
				if (next) await selectChat(next.id);
				else {
					setSelectedId(""); setChat(null); setMobileDetail(false);
					window.history.pushState(window.history.state, "", `/account/projects/${encodeURIComponent(projectId)}/chat`);
				}
			}
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : "Не удалось удалить чат");
		} finally {
			setDeletingChatId("");
		}
	}

  async function submit(event: FormEvent) {
    event.preventDefault(); const body = draft.trim(); if (!body || sending || !chat || !selectedId) return;
    setSending(true); setSendError("");
    try { const message = await sendProjectContextChatMessage(projectId, selectedId, body, crypto.randomUUID()); setChat((current) => current ? { ...current, messages: mergeMessages(current.messages, [message]) } : current); setDraft(""); requestAnimationFrame(() => bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" })); }
    catch (reason) { setSendError(reason instanceof Error ? reason.message : "Не удалось отправить сообщение"); }
    finally { setSending(false); }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) { if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit(); } }
  async function loadOlder() { if (!chat?.nextCursor || !selectedId) return; const older = await loadConversation(selectedId, chat.nextCursor); setChat((current) => current ? { ...current, messages: mergeMessages(older.messages, current.messages), hasMore: older.hasMore, nextCursor: older.nextCursor } : older); }

  if (state === "loading") return <div aria-label="Загрузка чатов проекта" className="h-[640px] animate-pulse border border-border bg-surface" role="status" />;
  if (state === "error" || !project) return <div className="border border-red-700/40 bg-surface p-8" role="alert"><h1 className="font-display text-4xl">Чаты не открылись</h1><p className="mt-3 text-secondary">{error}</p><Button className="mt-6" onClick={() => void loadWorkspace()}>Повторить</Button></div>;

  return <section aria-label="Чаты проекта" data-cy="project-chat-page">
    <nav aria-label="Хлебные крошки" className="flex flex-wrap items-center gap-2 text-xs text-secondary"><Link className="hover:text-primary" href="/account">Проекты</Link><span>/</span><Link className="truncate hover:text-primary" href={`/account/projects/${encodeURIComponent(projectId)}`}>{project.name}</Link><span>/</span><span aria-current="page">Чаты</span></nav>
    <p aria-atomic="true" className="sr-only" role="status">{arrivalNotice}</p>
		{error ? <p className="mt-5 border border-red-700/40 bg-surface p-3 text-sm text-red-700" role="alert">{error}</p> : null}
		{sendError && !chat ? <p className="mt-5 border border-red-700/40 bg-surface p-3 text-sm text-red-700" role="alert">{sendError}</p> : null}
    <div className="mt-7 grid min-h-[680px] overflow-hidden border border-border bg-surface tablet:grid-cols-[280px_minmax(0,1fr)] desktop:grid-cols-[300px_minmax(0,1fr)]">
      <aside className={`${mobileDetail ? "hidden" : "block"} border-b border-border tablet:block tablet:border-b-0 tablet:border-r`} aria-label="Список чатов">
        <div className="flex items-center justify-between gap-3 border-b border-border p-4"><div><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">Проект</p><h1 className="mt-1 text-xl font-medium">Чаты</h1></div><IconButton aria-label="Создать чат" className="size-9 border-icon-button-border bg-icon-button text-icon-button-text" data-cy="create-project-chat" onClick={() => setCreateOpen(true)} tooltip="Создать чат"><PlusIcon /></IconButton></div>
		<div className="max-h-[610px] overflow-y-auto p-2">{chats.length === 0 ? <p className="p-4 text-sm text-secondary">Создайте первый чат проекта.</p> : chats.map((item) => <div className={`flex items-center border-b border-border transition-colors ${selectedId === item.id ? "bg-action/10" : "hover:bg-page"}`} data-cy={`chat-list-item-${item.id}`} key={item.id}><Link aria-current={selectedId === item.id ? "page" : undefined} className="flex min-w-0 flex-1 items-center gap-3 px-3 py-4 text-left" href={`/account/projects/${encodeURIComponent(projectId)}/chat/${encodeURIComponent(item.id)}`} onClick={(event) => { event.preventDefault(); void selectChat(item.id); }} scroll={false}><span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{item.name}</span><span className="mt-1 block truncate text-xs text-secondary">{item.contextType ? `${contextTypeLabel(item.contextType)}${item.contextTitle ? ` · ${item.contextTitle}` : ""}` : "Проект"} · {item.memberCount} участн.</span></span>{item.unreadCount > 0 ? <span aria-label={`Непрочитанных сообщений: ${item.unreadCount}`} className="inline-flex min-w-6 shrink-0 items-center justify-center rounded-full bg-action px-2 py-0.5 text-xs font-semibold text-inverse-text">{item.unreadCount > 99 ? "99+" : item.unreadCount}</span> : null}</Link>{item.canManage ? <span className="flex shrink-0 items-center gap-1 pr-2"><IconButton aria-label={`Настроить чат «${item.name}»`} className="size-7 border-icon-button-border bg-icon-button text-icon-button-text hover:bg-icon-button-hover" onClick={() => openChatSettings(item)} tooltip="Настроить чат"><SettingsIcon /></IconButton><IconButton aria-label={`Удалить чат «${item.name}»`} className="size-7 border-danger/40 bg-icon-button text-danger hover:bg-icon-button-hover" disabled={deletingChatId === item.id} onClick={() => void removeChatFromList(item)} tooltip="Удалить чат"><TrashIcon /></IconButton></span> : null}</div>)}</div>
      </aside>
	  <div className={`${mobileDetail ? "flex" : "hidden"} min-w-0 flex-col tablet:flex`} data-cy="project-chat-detail">
        {selected && chat ? <>
		  <div className="flex items-center border-b border-border p-2 tablet:hidden"><Link aria-label="Вернуться к списку чатов" className="inline-flex size-11 shrink-0 items-center justify-center border border-border hover:bg-page" href={`/account/projects/${encodeURIComponent(projectId)}/chat`}><BackIcon /></Link></div>
          <div className="min-h-[410px] flex-1 overflow-y-auto px-5 py-6" data-cy="project-chat-history" role="log" aria-live="polite">{chat.hasMore ? <div className="mb-5 text-center"><Button onClick={() => void loadOlder()} variant="secondary">Показать предыдущие</Button></div> : null}{chat.messages.length === 0 ? <div className="grid min-h-[330px] place-items-center text-center"><div><ChatIcon /><h3 className="mt-4 text-lg font-medium">Сообщений пока нет</h3><p className="mt-2 text-sm text-secondary">Напишите первое сообщение в этом чате.</p></div></div> : <ol className="grid gap-6">{chat.messages.map((message) => <MessageItem currentUserId={chat.currentUserId} key={message.id} message={message} />)}</ol>}<div ref={bottomRef} /></div>
          <form className="border-t border-border p-4" onSubmit={submit}><div className="flex items-end gap-3 border border-border bg-page px-3 py-2 focus-within:border-field-focus"><Textarea aria-label="Сообщение" className="min-h-20 flex-1 resize-none border-0 px-1 py-2" data-cy="project-chat-message" disabled={sending} maxLength={5000} onChange={(event) => setDraft(event.target.value)} onKeyDown={handleKeyDown} placeholder="Напишите сообщение…" rows={3} value={draft} /><IconButton aria-label="Отправить сообщение" className="mb-1 size-10 border-icon-button-border bg-icon-button text-icon-button-text" data-cy="send-project-chat" disabled={!draft.trim() || sending} tooltip="Отправить" type="submit"><SendIcon /></IconButton></div>{sendError ? <p className="mt-3 text-sm text-red-700" role="alert">{sendError}</p> : null}</form>
        </> : <div className="grid flex-1 place-items-center p-8 text-center"><div><ChatIcon /><h2 className="mt-4 text-xl font-medium">Выберите или создайте чат</h2><p className="mt-2 text-sm text-secondary">Обсуждения задач, материалов и расходов собраны здесь.</p></div></div>}
      </div>
    </div>
    <CreateChatDialog isOpen={createOpen} onClose={() => setCreateOpen(false)} onCreated={(created) => { setChats((items) => [...items, created]); setCreateOpen(false); void selectChat(created.id); }} projectId={projectId} />
	{settingsChat ? <ChatSettingsDialog chat={settingsChat} isOpen={settingsOpen} onChanged={(changed) => setChats((items) => items.map((item) => item.id === changed.id ? changed : item))} onClose={() => { setSettingsOpen(false); setSettingsChatId(""); }} projectId={projectId} /> : null}
  </section>;
}

function CreateChatDialog({ isOpen, onClose, onCreated, projectId }: Readonly<{ isOpen: boolean; onClose: () => void; onCreated: (chat: ProjectChatSummary) => void; projectId: string }>) {
	const [name, setName] = useState("");
	const [members, setMembers] = useState<ProjectMember[]>([]);
	const [selected, setSelected] = useState<ProjectMember[]>([]);
	const [query, setQuery] = useState("");
	const [error, setError] = useState("");
	const [loadingMembers, setLoadingMembers] = useState(false);
	const [pending, setPending] = useState(false);

	useEffect(() => {
		if (!isOpen) return;
		let cancelled = false;
		queueMicrotask(() => {
			if (cancelled) return;
			setLoadingMembers(true); setError(""); setQuery("");
			void loadProjectMembers(projectId)
				.then((value) => { if (!cancelled) setMembers(value.items); })
				.catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : "Не удалось загрузить участников проекта"); })
				.finally(() => { if (!cancelled) setLoadingMembers(false); });
		});
		return () => { cancelled = true; };
	}, [isOpen, projectId]);

	const options: (AutocompleteOption & { member: ProjectMember })[] = members
		.filter((member) => !selected.some((item) => item.userId === member.userId))
		.filter((member) => `${member.firstName} ${member.lastName ?? ""} ${member.login}`.toLocaleLowerCase("ru").includes(query.trim().toLocaleLowerCase("ru")))
		.map((member) => ({ id: member.userId, label: `${member.firstName} ${member.lastName ?? ""}`.trim(), description: `@${member.login}`, member }));

	async function submit(event: FormEvent) {
		event.preventDefault();
		if (name.trim().length < 2) { setError("Название должно содержать не менее двух символов"); return; }
		setPending(true); setError("");
		try {
			onCreated(await createProjectChat(projectId, name.trim(), selected.map((member) => member.userId)));
			setName(""); setSelected([]); setQuery("");
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : "Не удалось создать чат");
		} finally { setPending(false); }
	}

	return <Dialog isOpen={isOpen} label="Создать чат" onClose={onClose}><form onSubmit={submit}><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">Новый чат</p><h2 className="mt-3 font-display text-4xl">Создать обсуждение</h2><label className="mt-7 block text-sm" htmlFor="chat-name">Название *</label><Input data-cy="chat-name" id="chat-name" maxLength={200} onChange={(event) => setName(event.target.value)} placeholder="Например, Согласование кухни" value={name}/><div className="mt-6"><Autocomplete emptyText="Доступные участники проекта не найдены" label="Добавить участников" loading={loadingMembers} onSelect={(option) => { setSelected((items) => [...items, option.member]); setQuery(""); }} onValueChange={setQuery} options={options} placeholder="Имя или логин" value={query}/>{!loadingMembers && members.length === 0 ? <p className="mt-3 text-sm text-secondary">В проекте пока нет участников. Сначала <Link className="text-action underline" href={`/account/projects/${encodeURIComponent(projectId)}`}>добавьте их в проект</Link>, затем они станут доступны для выбора в чате.</p> : null}{selected.length > 0 ? <ul aria-label="Выбранные участники" className="mt-4 grid gap-2">{selected.map((member) => <li className="flex items-center gap-3 border border-border px-3 py-2" key={member.userId}><span className="min-w-0 flex-1 truncate text-sm">{member.firstName} {member.lastName ?? ""} <span className="text-secondary">@{member.login}</span></span><button aria-label={`Убрать ${member.firstName} из создаваемого чата`} className="inline-flex min-h-11 items-center text-xs text-red-700 underline" onClick={() => setSelected((items) => items.filter((item) => item.userId !== member.userId))} type="button">Убрать</button></li>)}</ul> : null}<p className="mt-2 text-xs text-secondary">Вы автоматически будете добавлены в чат.</p></div>{error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 flex gap-3"><Button disabled={pending || loadingMembers} type="submit">{pending ? "Создаём…" : "Создать чат"}</Button><Button onClick={onClose} type="button" variant="secondary">Отмена</Button></div></form></Dialog>;
}

function ChatSettingsDialog({chat,isOpen,onChanged,onClose,projectId}:Readonly<{chat:ProjectChatSummary;isOpen:boolean;onChanged:(chat:ProjectChatSummary)=>void;onClose:()=>void;projectId:string}>){
  const [name,setName]=useState(chat.name);const [data,setData]=useState<ProjectChatMemberList|null>(null);const [projectMembers,setProjectMembers]=useState<ProjectMember[]>([]);const [query,setQuery]=useState("");const [error,setError]=useState("");const [pendingMemberId,setPendingMemberId]=useState("");
  useEffect(()=>{let cancelled=false;queueMicrotask(()=>{if(cancelled)return;setName(chat.name);setData(null);setProjectMembers([]);setQuery("");setError("");if(isOpen)void Promise.all([loadProjectChatMembers(projectId,chat.id),loadProjectMembers(projectId)]).then(([members,projectList])=>{if(!cancelled){setData(members);setProjectMembers(projectList.items)}}).catch((reason)=>{if(!cancelled)setError(reason instanceof Error?reason.message:"Не удалось загрузить настройки")})});return()=>{cancelled=true}},[chat.id,chat.name,chat.version,isOpen,projectId]);
  const available=projectMembers.filter((candidate)=>!data?.items.some((member)=>member.userId===candidate.userId));
  const options:(AutocompleteOption&{member:ProjectMember})[]=available.filter((member)=>`${member.firstName} ${member.lastName??""} ${member.login}`.toLocaleLowerCase("ru").includes(query.trim().toLocaleLowerCase("ru"))).map((member)=>({id:member.userId,label:`${member.firstName} ${member.lastName??""}`.trim(),description:`@${member.login}`,member}));
  async function rename(){try{const changed=await updateProjectChat(projectId,chat.id,name.trim(),chat.version);onChanged(changed);setError("")}catch(reason){setError(reason instanceof Error?reason.message:"Не удалось сохранить название")}}
  async function add(userId:string){setPendingMemberId(userId);try{const member=await addProjectChatMember(projectId,chat.id,userId);setData((current)=>current?{...current,items:[...current.items,member]}:current);onChanged({...chat,memberCount:(data?.items.length??chat.memberCount)+1});setQuery("");setError("")}catch(reason){setError(reason instanceof Error?reason.message:"Не удалось добавить участника")}finally{setPendingMemberId("")}}
  async function remove(userId:string){setPendingMemberId(userId);try{await removeProjectChatMember(projectId,chat.id,userId);setData((current)=>current?{...current,items:current.items.filter((item)=>item.userId!==userId)}:current);onChanged({...chat,memberCount:Math.max(1,(data?.items.length??chat.memberCount)-1)});setError("")}catch(reason){setError(reason instanceof Error?reason.message:"Не удалось удалить участника")}finally{setPendingMemberId("")}}
	return <Dialog isOpen={isOpen} label="Настройки чата" onClose={onClose}><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">Настройки</p><h2 className="mt-3 font-display text-4xl">Чат и участники</h2><label className="mt-7 block text-sm" htmlFor="rename-chat">Название</label><div className="flex items-end gap-3"><Input id="rename-chat" maxLength={200} onChange={(event)=>setName(event.target.value)} value={name}/><Button disabled={name.trim().length<2||name.trim()===chat.name} onClick={()=>void rename()} type="button">Сохранить</Button></div><h3 className="mt-8 text-sm font-medium">Участники чата</h3>{data ? <div className="mt-3 border border-border">{data.items.map((member)=><div className="flex items-center gap-3 border-b border-border px-4 py-3 last:border-0" key={member.userId}><span className="min-w-0 flex-1 truncate text-sm">{member.firstName} {member.lastName??""} <span className="text-secondary">@{member.login}</span></span>{data.canManage&&member.removable?<button aria-label={`Удалить ${member.firstName} из чата`} className="min-h-11 text-xs text-red-700 underline" disabled={pendingMemberId===member.userId} onClick={()=>void remove(member.userId)} type="button">{pendingMemberId===member.userId?"Удаляем…":"Удалить"}</button>:null}</div>)}</div> : <p className="mt-3 text-sm text-secondary" role="status">Загружаем участников…</p>}{data?.canManage?<div className="mt-5">{available.length>0?<Autocomplete disabled={Boolean(pendingMemberId)} emptyText="Участники проекта не найдены" label="Добавить участника проекта" onSelect={(option)=>void add(option.member.userId)} onValueChange={setQuery} options={options} placeholder="Имя или логин" value={query}/>:<p className="text-sm text-secondary">{projectMembers.length===0?"В проекте пока нет участников.":"Все участники проекта уже добавлены в этот чат."} <Link className="text-action underline" href={`/account/projects/${encodeURIComponent(projectId)}`}>Управлять участниками проекта</Link>.</p>}</div>:null}{error?<p className="mt-4 text-sm text-red-700" role="alert">{error}</p>:null}</Dialog>
}

function MessageItem({currentUserId,message}:Readonly<{currentUserId:string;message:ProjectChatMessage}>){const own=message.author.userId===currentUserId;const author=[message.author.firstName,message.author.lastName].filter(Boolean).join(" ")||`@${message.author.login}`;return <li className={`max-w-[82%] ${own?"ml-auto text-right":"mr-auto"}`} data-cy={`chat-message-${message.id}`}><div className={`text-xs ${own?"text-right":"text-left"}`}><strong>{own?"Вы":author}</strong></div><p className={`mt-2 whitespace-pre-wrap break-words border px-4 py-3 text-left text-sm leading-6 ${own?"border-action/40 bg-action/10":"border-border bg-page"}`}>{message.body}</p><time className="mt-1.5 block text-[10px] text-secondary" dateTime={message.createdAt}>{new Intl.DateTimeFormat("ru-RU",{day:"numeric",hour:"2-digit",minute:"2-digit",month:"short"}).format(new Date(message.createdAt))}</time></li>}
function mergeMessages(...groups:ProjectChatMessage[][]){const values=new Map<string,ProjectChatMessage>();for(const message of groups.flat())values.set(message.id,message);return [...values.values()].sort((a,b)=>new Date(a.createdAt).getTime()-new Date(b.createdAt).getTime())}
function contextTypeLabel(type?:"task"|"material"|"expense"){return type?({expense:"Расход",material:"Материал",task:"Задача"})[type]:"Проект"}
function PlusIcon(){return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 20 20"><path d="M10 4v12M4 10h12" stroke="currentColor" strokeLinecap="round" strokeWidth="1.5"/></svg>}
function SettingsIcon(){return <svg aria-hidden="true" className="size-3.5" fill="none" viewBox="0 0 20 20"><path d="M8.6 3.2h2.8l.5 1.7 1.4.8 1.8-.4 1.4 2.4-1.2 1.3v1.9l1.2 1.3-1.4 2.4-1.8-.4-1.4.8-.5 1.7H8.6L8.1 15l-1.4-.8-1.8.4-1.4-2.4 1.2-1.3V9L3.5 7.7l1.4-2.4 1.8.4 1.4-.8.5-1.7Z" stroke="currentColor"/><circle cx="10" cy="10" r="2.3" stroke="currentColor"/></svg>}
function TrashIcon(){return <svg aria-hidden="true" className="size-3.5" fill="none" viewBox="0 0 20 20"><path d="M4.5 6h11M8 3.5h4M6.5 6l.6 10h5.8l.6-10M8.5 8.5v5M11.5 8.5v5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.4"/></svg>}
function ChatIcon(){return <svg aria-hidden="true" className="mx-auto size-8 text-secondary" fill="none" viewBox="0 0 32 32"><path d="M5 6.5h22v15H14l-7 5v-5H5z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.5"/></svg>}
function SendIcon(){return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 24 24"><path d="m4 5 16 7-16 7 3-7-3-7Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.6"/><path d="M7 12h13" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6"/></svg>}
function BackIcon(){return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><path d="m12.5 4.5-5.5 5.5 5.5 5.5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5"/></svg>}
