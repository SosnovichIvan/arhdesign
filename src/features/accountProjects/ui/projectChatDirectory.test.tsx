// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { ProjectChatDirectory } from "./projectChatDirectory";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("retries a failed load and links to each visible project's chats with pagination", async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response(null, { status: 503 }))
    .mockResolvedValueOnce(Response.json({ items: [{ id: "first", name: "Дом" }], nextCursor: "next" }))
    .mockResolvedValueOnce(Response.json({ items: [{ id: "first", name: "Дом" }, { id: "second", name: "Студия" }], nextCursor: null }));
  vi.stubGlobal("fetch", fetchMock);
  render(<ProjectChatDirectory />);
  expect(screen.getByRole("status")).toBeTruthy();
  expect(await screen.findByRole("alert")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect((await screen.findByRole("link", { name: "Дом" })).getAttribute("href")).toBe("/account/projects/first/chat");
  fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
  expect((await screen.findByRole("link", { name: "Студия" })).getAttribute("href")).toBe("/account/projects/second/chat");
  expect(screen.getAllByRole("link", { name: "Дом" })).toHaveLength(1);
  expect(fetchMock.mock.calls[2][0]).toBe("/api/v1/projects?pageSize=50&cursor=next");
});

it("explains when there are no visible projects", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ items: [], nextCursor: null })));
  render(<ProjectChatDirectory />);
  expect(await screen.findByText("У вас пока нет доступных проектов.")).toBeTruthy();
});
