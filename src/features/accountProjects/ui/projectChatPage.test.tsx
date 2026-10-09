// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ProjectChatPage } from "./projectChatPage";

vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

const project = { address:null,autoApproveExpenses:true,createdAt:"2026-09-24T10:00:00Z",createdByUserId:"user-1",currencyCode:"RUB",customerUserId:"user-1",description:null,id:"project-1",name:"Полянка",plannedFinishOn:null,plannedStartOn:null,status:"active",type:"interior_design",version:1 };
const summary = { canManage:true,contextId:null,contextTitle:null,createdAt:"2026-09-24T10:00:00Z",id:"chat-1",kind:"project",memberCount:2,name:"Общий чат проекта",projectId:"project-1",unreadCount:0,version:1 };
const firstMessage = { author:{firstName:"Анна",lastName:"Иванова",login:"anna",userId:"user-2"},body:"Проверила планировку",chatId:"chat-1",createdAt:"2026-09-26T10:00:00Z",deletedAt:null,editedAt:null,id:"message-1",projectId:"project-1",version:1 };
const ownMessage = {...firstMessage,author:{firstName:"Светлана",lastName:null,login:"svetlana",userId:"user-1"},body:"Отправлено",id:"message-2"};
const json=(body:unknown,status=200)=>Promise.resolve(new Response(JSON.stringify(body),{headers:{"Content-Type":"application/json"},status}));
const page=(messages:unknown[])=>({canSend:true,chatId:"chat-1",currentUserId:"user-1",hasMore:false,messages,nextCursor:null,projectId:"project-1"});

beforeEach(()=>{Object.defineProperty(document,"cookie",{configurable:true,value:"arhdesign_csrf=csrf-token"});vi.stubGlobal("requestAnimationFrame",(callback:FrameRequestCallback)=>{callback(0);return 1});vi.stubGlobal("crypto",{randomUUID:()=>"00000000-0000-0000-0000-000000000031"});Element.prototype.scrollIntoView=vi.fn()});
afterEach(()=>{cleanup();vi.clearAllMocks();vi.unstubAllGlobals()});

describe("ProjectChatPage",()=>{
  it("shows the chat list, history and sends a message",async()=>{const fetchMock=vi.fn((input:string|URL|Request,init?:RequestInit)=>{const url=String(input);if(init?.method==="POST"&&url.endsWith("/messages"))return json(ownMessage,201);if(url.endsWith("/chats"))return json({canCreate:true,items:[summary]});if(url.includes("/chats/chat-1?"))return json(page([firstMessage]));return json(project)});vi.stubGlobal("fetch",fetchMock);render(<ProjectChatPage projectId="project-1"/>);expect(await screen.findByText("Проверила планировку")).toBeTruthy();expect(screen.getAllByText("Общий чат проекта").length).toBeGreaterThan(0);fireEvent.change(screen.getByLabelText("Сообщение"),{target:{value:"Отправлено"}});fireEvent.click(screen.getByRole("button",{name:"Отправить сообщение"}));expect(await screen.findByText("Отправлено")).toBeTruthy()});
	it("creates a named chat with selected project members",async()=>{const created={...summary,id:"chat-2",name:"Согласование кухни"};const fetchMock=vi.fn((input:string|URL|Request,init?:RequestInit)=>{const url=String(input);if(init?.method==="POST"&&url.endsWith("/chats"))return json(created,201);if(url.endsWith("/members"))return json({canManage:true,items:[{firstName:"Анна",joinedAt:"2026-09-24T10:00:00Z",lastName:"Иванова",login:"anna",middleName:null,professionalRole:{code:"designer",name:"Дизайнер"},projectRoles:["executor"],removable:true,userId:"user-2"}]});if(url.endsWith("/chats"))return json({canCreate:true,items:[summary]});if(url.includes("/chats/chat-"))return json(page([]));return json(project)});vi.stubGlobal("fetch",fetchMock);render(<ProjectChatPage projectId="project-1"/>);await screen.findByText("Сообщений пока нет");fireEvent.click(screen.getByRole("button",{name:"Создать чат"}));fireEvent.change(screen.getByLabelText("Название *"),{target:{value:"Согласование кухни"}});fireEvent.change(screen.getByLabelText("Добавить участников"),{target:{value:"Анна"}});fireEvent.click(await screen.findByRole("option",{name:/Анна Иванова/}));expect(screen.getByRole("list",{name:"Выбранные участники"})).toBeTruthy();fireEvent.click(screen.getAllByRole("button",{name:"Создать чат"}).at(-1)!);await waitFor(()=>expect(fetchMock.mock.calls.some(([url,options])=>String(url).endsWith("/chats")&&options?.method==="POST"&&String(options.body).includes("Согласование кухни")&&String(options.body).includes("user-2"))).toBe(true))});
  it("renders a useful empty state",async()=>{vi.stubGlobal("fetch",vi.fn((input:string|URL|Request)=>{const url=String(input);if(url.endsWith("/chats"))return json({canCreate:true,items:[summary]});if(url.includes("/chats/chat-1?"))return json(page([]));return json(project)}));render(<ProjectChatPage projectId="project-1"/>);expect(await screen.findByText("Сообщений пока нет")).toBeTruthy()});
  it("renders access errors",async()=>{vi.stubGlobal("fetch",vi.fn((input:string|URL|Request)=>String(input).endsWith("/chats")?json({error:{message:"Проект не найден"}},404):json(project)));render(<ProjectChatPage projectId="project-1"/>);expect(await screen.findByRole("heading",{name:"Чаты не открылись"})).toBeTruthy();expect(screen.getByRole("button",{name:"Повторить"})).toBeTruthy()});
	it("adds and removes a project member, then deletes the chat from its list row",async()=>{const projectMember={firstName:"Иван",joinedAt:"2026-09-24T10:00:00Z",lastName:"Петров",login:"ivan",middleName:null,professionalRole:{code:"architect",name:"Архитектор"},projectRoles:["executor"],removable:true,userId:"user-3"};const chatMember={firstName:"Светлана",joinedAt:"2026-09-24T10:00:00Z",lastName:null,login:"svetlana",removable:false,userId:"user-1"};const fetchMock=vi.fn((input:string|URL|Request,init?:RequestInit)=>{const url=String(input);if(init?.method==="POST"&&url.endsWith("/chats/chat-1/members"))return json({...projectMember},201);if(init?.method==="DELETE"&&url.endsWith("/chats/chat-1/members/user-3"))return Promise.resolve(new Response(null,{status:204}));if(init?.method==="DELETE"&&url.includes("/chats/chat-1?version=1"))return Promise.resolve(new Response(null,{status:204}));if(url.endsWith("/chats/chat-1/members"))return json({canManage:true,items:[chatMember]});if(url.endsWith("/projects/project-1/members"))return json({canManage:true,items:[projectMember]});if(url.endsWith("/chats"))return json({canCreate:true,items:[summary]});if(url.includes("/chats/chat-1?"))return json(page([]));return json(project)});vi.stubGlobal("fetch",fetchMock);vi.stubGlobal("confirm",vi.fn(()=>true));render(<ProjectChatPage chatId="chat-1" projectId="project-1"/>);await screen.findByText("Сообщений пока нет");expect(document.querySelector('[data-cy="project-chat-detail"] h2')).toBeNull();fireEvent.click(screen.getByRole("button",{name:"Настроить чат «Общий чат проекта»"}));const search=await screen.findByLabelText("Добавить участника проекта");fireEvent.change(search,{target:{value:"Иван"}});fireEvent.click(await screen.findByRole("option",{name:/Иван Петров/}));await waitFor(()=>expect(fetchMock.mock.calls.some(([url,options])=>String(url).endsWith("/chats/chat-1/members")&&options?.method==="POST")).toBe(true));fireEvent.click(await screen.findByRole("button",{name:"Удалить Иван из чата"}));await waitFor(()=>expect(fetchMock.mock.calls.some(([url,options])=>String(url).endsWith("/chats/chat-1/members/user-3")&&options?.method==="DELETE")).toBe(true));fireEvent.click(screen.getByRole("button",{name:"Закрыть форму"}));fireEvent.click(screen.getByRole("button",{name:"Удалить чат «Общий чат проекта»"}));await waitFor(()=>expect(fetchMock.mock.calls.some(([url,options])=>String(url).includes("/chats/chat-1?version=1")&&options?.method==="DELETE")).toBe(true))});
	it("uses list then detail navigation on mobile layouts",async()=>{vi.stubGlobal("fetch",vi.fn((input:string|URL|Request)=>{const url=String(input);if(url.endsWith("/chats"))return json({canCreate:true,items:[summary]});if(url.includes("/chats/chat-1?"))return json(page([]));return json(project)}));const {container}=render(<ProjectChatPage chatId="chat-1" projectId="project-1"/>);await screen.findByText("Сообщений пока нет");expect(container.querySelector("aside")?.className).toContain("hidden");expect(screen.getByRole("link",{name:"Вернуться к списку чатов"}).getAttribute("href")).toBe("/account/projects/project-1/chat")});

  it("renders contextual unread chats, loads older messages and reports selection/send errors", async () => {
    const contextual = [
      { ...summary, contextTitle: "Рабочие чертежи", contextType: "task", id: "task-chat", name: "Задача", unreadCount: 100 },
      { ...summary, contextType: "material", id: "material-chat", name: "Материал", unreadCount: 2 },
      { ...summary, canManage: false, contextType: "expense", id: "expense-chat", name: "Расход", unreadCount: 0 },
    ];
    let materialFails = true;
    const older = { ...firstMessage, createdAt: "2026-09-25T10:00:00Z", id: "older", body: "Предыдущее" };
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "POST" && url.endsWith("/messages")) return Promise.reject(new Error("Сообщение не отправлено"));
      if (url.endsWith("/chats")) return json({ canCreate: true, items: contextual });
      if (url.includes("/chats/material-chat?")) {
        if (materialFails) { materialFails = false; return Promise.reject(new Error("Чат временно недоступен")); }
        return json({ ...page([]), chatId: "material-chat" });
      }
      if (url.includes("before=older")) return json({ ...page([older]), hasMore: false, nextCursor: null });
      if (url.includes("/chats/task-chat?")) return json({ ...page([firstMessage]), chatId: "task-chat", hasMore: true, nextCursor: "older" });
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    expect(await screen.findByText("Проверила планировку")).toBeTruthy();
    expect(screen.getByText("Задача · Рабочие чертежи · 2 участн.")).toBeTruthy();
    expect(screen.getByText("Материал · 2 участн.")).toBeTruthy();
    expect(screen.getByText("Расход · 2 участн.")).toBeTruthy();
    expect(screen.getByLabelText("Непрочитанных сообщений: 100").textContent).toBe("99+");
    fireEvent.click(screen.getByRole("button", { name: "Показать предыдущие" }));
    expect(await screen.findByText("Предыдущее")).toBeTruthy();
    fireEvent.click(screen.getByRole("link", { name: /Материал/ }));
    expect((await screen.findByRole("alert")).textContent).toContain("Чат временно недоступен");
    fireEvent.click(screen.getByRole("link", { name: /Материал/ }));
    await screen.findByText("Сообщений пока нет");
    fireEvent.change(screen.getByLabelText("Сообщение"), { target: { value: "Ошибка" } });
    fireEvent.keyDown(screen.getByLabelText("Сообщение"), { key: "Enter", shiftKey: false });
    expect((await screen.findByRole("alert")).textContent).toContain("Сообщение не отправлено");
  });

  it("validates chat creation, removes a selected member and shows member-load and create errors", async () => {
    let loadMembersFails = false;
    const projectMember={firstName:"Иван",joinedAt:"2026-09-24T10:00:00Z",lastName:null,login:"ivan",middleName:null,professionalRole:{code:"architect",name:"Архитектор"},projectRoles:["executor"],removable:true,userId:"user-3"};
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "POST" && url.endsWith("/chats")) return Promise.reject(new Error("Чат не создан"));
      if (url.endsWith("/members")) return loadMembersFails ? Promise.reject(new Error("Участники недоступны")) : json({ canManage: true, items: [projectMember] });
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Создать чат" }));
    fireEvent.click(screen.getAllByRole("button", { name: "Создать чат" }).at(-1)!);
    expect(screen.getByRole("alert").textContent).toContain("не менее двух");
    fireEvent.change(screen.getByLabelText("Название *"), { target: { value: "Новый чат" } });
    fireEvent.change(screen.getByLabelText("Добавить участников"), { target: { value: "Иван" } });
    fireEvent.click(await screen.findByRole("option", { name: /Иван/ }));
    fireEvent.click(screen.getByRole("button", { name: "Убрать Иван из создаваемого чата" }));
    expect(screen.queryByRole("list", { name: "Выбранные участники" })).toBeNull();
    fireEvent.click(screen.getAllByRole("button", { name: "Создать чат" }).at(-1)!);
    expect((await screen.findByRole("alert")).textContent).toContain("Чат не создан");
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    loadMembersFails = true;
    fireEvent.click(screen.getByRole("button", { name: "Создать чат" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Участники недоступны");
  });

  it("renames a chat, reports settings failures and covers all-members state", async () => {
    const chatMember={firstName:"Светлана",joinedAt:"2026-09-24T10:00:00Z",lastName:null,login:"svetlana",removable:false,userId:"user-1"};
    let renameFails = true;
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "PATCH") return renameFails ? Promise.reject(new Error("Название не сохранено")) : json({ ...summary, name: "Новое название", version: 2 });
      if (url.endsWith("/chats/chat-1/members")) return json({ canManage: true, items: [chatMember] });
      if (url.endsWith("/projects/project-1/members")) return json({ canManage: true, items: [] });
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Настроить чат «Общий чат проекта»" }));
    expect(await screen.findByText(/В проекте пока нет участников/)).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Название"), { target: { value: "Новое название" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Название не сохранено");
    renameFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));
    expect(await screen.findByText("Новое название")).toBeTruthy();
  });

  it("cancels deletion, reports delete failure and removes the last selected chat", async () => {
    let confirmDelete = false;
    let deleteFails = true;
    vi.stubGlobal("confirm", vi.fn(() => confirmDelete));
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "DELETE") return deleteFails ? Promise.reject(new Error("Чат не удалён")) : Promise.resolve(new Response(null, { status: 204 }));
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Удалить чат «Общий чат проекта»" }));
    expect(fetchMock.mock.calls.some(([, init]) => init?.method === "DELETE")).toBe(false);
    confirmDelete = true;
    fireEvent.click(screen.getByRole("button", { name: "Удалить чат «Общий чат проекта»" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Чат не удалён");
    deleteFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Удалить чат «Общий чат проекта»" }));
    expect(await screen.findByText("Выберите или создайте чат")).toBeTruthy();
  });

  it("announces incoming background messages on window focus and keeps authored messages deduplicated", async () => {
    let conversationLoads = 0;
    const second = { ...firstMessage, author: { firstName: "Иван", lastName: null, login: "ivan", userId: "user-3" }, body: "Новое сообщение", id: "message-new", createdAt: "2026-09-26T11:00:00Z" };
    const third = { ...second, author: { firstName: "Мария", lastName: null, login: "maria", userId: "user-4" }, body: "Ещё сообщение", id: "message-third", createdAt: "2026-09-26T11:01:00Z" };
    const fetchMock = vi.fn((input: string | URL | Request) => {
      const url = String(input);
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) { conversationLoads += 1; return json(page(conversationLoads === 1 ? [firstMessage] : [firstMessage, ownMessage, second, third])); }
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Проверила планировку");
    fireEvent(window, new Event("focus"));
    expect(await screen.findByText("Новое сообщение")).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain("В чат пришло новых сообщений: 2");
    expect(screen.getAllByText("Проверила планировку")).toHaveLength(1);
  });

  it("reports settings loading, add and remove failures", async () => {
    const available={firstName:"Иван",joinedAt:"2026-09-24T10:00:00Z",lastName:"Петров",login:"ivan",middleName:null,professionalRole:{code:"architect",name:"Архитектор"},projectRoles:["executor"],removable:true,userId:"user-3"};
    const existing={firstName:"Анна",joinedAt:"2026-09-24T10:00:00Z",lastName:"Иванова",login:"anna",removable:true,userId:"user-2"};
    let membersLoadFails = true;
    const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "POST" && url.endsWith("/members")) return Promise.reject(new Error("Добавление запрещено"));
      if (init?.method === "DELETE" && url.includes("/members/")) return Promise.reject(new Error("Удаление запрещено"));
      if (url.endsWith("/chats/chat-1/members")) { if (membersLoadFails) { membersLoadFails = false; return Promise.reject(new Error("Настройки недоступны")); } return json({ canManage: true, items: [existing] }); }
      if (url.endsWith("/projects/project-1/members")) return json({ canManage: true, items: [available] });
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Настроить чат «Общий чат проекта»" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Настройки недоступны");
    fireEvent.click(screen.getByRole("button", { name: "Закрыть форму" }));
    fireEvent.click(screen.getByRole("button", { name: "Настроить чат «Общий чат проекта»" }));
    const search = await screen.findByLabelText("Добавить участника проекта");
    fireEvent.change(search, { target: { value: "Иван" } });
    fireEvent.click(await screen.findByRole("option", { name: /Иван Петров/ }));
    expect((await screen.findByRole("alert")).textContent).toContain("Добавление запрещено");
    fireEvent.click(screen.getByRole("button", { name: "Удалить Анна из чата" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Удаление запрещено");
  });

  it("deletes the active chat and selects the next available conversation", async () => {
    const second = { ...summary, id: "chat-2", name: "Второй чат" };
    vi.stubGlobal("confirm", vi.fn(() => true));
    vi.stubGlobal("fetch", vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "DELETE") return Promise.resolve(new Response(null, { status: 204 }));
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary, second] });
      if (url.includes("/chats/chat-2?")) return json({ ...page([]), chatId: "chat-2" });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    }));
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Удалить чат «Общий чат проекта»" }));
    await waitFor(() => expect(screen.queryByText("Общий чат проекта")).toBeNull());
    expect(screen.getByText("Второй чат")).toBeTruthy();
  });

  it("falls back from an unknown route chat and supports an empty workspace", async () => {
    const fetchMock = vi.fn((input: string | URL | Request) => { const url = String(input); if (url.endsWith("/chats")) return json({ canCreate: true, items: [] }); return json(project); });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage chatId="missing-chat" projectId="project-1" />);
    expect(await screen.findByText("Создайте первый чат проекта.")).toBeTruthy();
    expect(screen.getByText("Выберите или создайте чат")).toBeTruthy();
    fireEvent(window, new Event("focus"));
    await waitFor(() => expect(fetchMock.mock.calls.filter(([url]) => String(url).endsWith("/chats")).length).toBeGreaterThan(1));
  });

  it("shows the no-members creation hint and the all-project-members settings hint", async () => {
    const member={firstName:"Светлана",joinedAt:"2026-09-24T10:00:00Z",lastName:null,login:"svetlana",middleName:null,professionalRole:{code:"designer",name:"Дизайнер"},projectRoles:["executor"],removable:false,userId:"user-1"};
    const fetchMock = vi.fn((input: string | URL | Request) => {
      const url = String(input);
      if (url.endsWith("/chats/chat-1/members")) return json({ canManage: true, items: [member] });
      if (url.endsWith("/projects/project-1/members")) return json({ canManage: true, items: [member] });
      if (url.endsWith("/chats")) return json({ canCreate: true, items: [summary] });
      if (url.includes("/chats/chat-1?")) return json(page([]));
      return json(project);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<ProjectChatPage projectId="project-1" />);
    await screen.findByText("Сообщений пока нет");
    fireEvent.click(screen.getByRole("button", { name: "Настроить чат «Общий чат проекта»" }));
    expect(await screen.findByText(/Все участники проекта уже добавлены/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Закрыть форму" }));
  });
});
