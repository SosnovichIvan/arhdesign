// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { ChatImages } from "./chatImages";
import type { ChatImage } from "../api/chatImages";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("loads private images with retries, previews, uploads, pins, and deletes", async () => {
  const kitchen: ChatImage = { id: "image-1", name: "Кухня.png", width: 64, height: 32, pinned: false, canDelete: true };
  const second = { ...kitchen, id: "image-2", name: "Схема.png", canDelete: false };
  const newImage = { ...kitchen, id: "image-3", name: "Эскиз.png" };
  let items = [kitchen];
  let failList = true;
  let failUpload = true;
  let failDelete = true;
  let failMore = true;
  const uploadKeys: string[] = [];
  const fetchMock = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    expect(url.searchParams.get("projectId")).toBe("project-1");
    if (init?.method === "POST") {
      uploadKeys.push(new Headers(init.headers).get("Idempotency-Key") ?? "");
      expect(new Headers(init.headers).get("X-File-Name")).toContain("UTF-8''");
      if (failUpload) return json({ error: { message: "Не удалось загрузить файл" } }, 503);
      items = [newImage, ...items];
      return json(newImage, 201);
    }
    if (init?.method === "PUT") {
      const { pinned } = JSON.parse(String(init.body)) as { pinned: boolean };
      items = items.map((image) => image.id === kitchen.id ? { ...image, pinned } : image);
      return new Response(null, { status: 204 });
    }
    if (init?.method === "DELETE") {
      if (failDelete) return json({ error: { message: "Не удалось удалить файл" } }, 503);
      items = items.filter((image) => image.id !== kitchen.id);
      return new Response(null, { status: 204 });
    }
    if (url.searchParams.get("cursor")) {
      return failMore ? json({ error: { message: "Страница недоступна" } }, 503) : json({ items: [kitchen, second] });
    }
    return failList ? json({ error: { message: "Изображения недоступны" } }, 503) : json({ items, nextCursor: "next+" });
  });
  vi.stubGlobal("fetch", fetchMock);

  const { container } = render(<ChatImages chatId="chat-1" projectId="project-1" />);
  const gallery = container.querySelector("details")!;
  expect(fetchMock).not.toHaveBeenCalled();
  gallery.open = true;
  fireEvent(gallery, new Event("toggle"));
  expect(await screen.findByText("Изображения недоступны")).toBeTruthy();
  failList = false;
  fireEvent.click(screen.getByRole("button", { name: "Обновить список" }));
  await screen.findByRole("button", { name: "Просмотреть Кухня.png" });
  fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
  expect(await screen.findByText("Страница недоступна")).toBeTruthy();
  failMore = false;
  fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
  await screen.findByRole("button", { name: "Просмотреть Схема.png" });
  expect(screen.getAllByRole("button", { name: "Просмотреть Кухня.png" })).toHaveLength(1);
  expect(screen.queryByRole("button", { name: "Удалить Схема.png" })).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: "Просмотреть Кухня.png" }));
  const preview = screen.getByRole("dialog", { name: "Просмотр Кухня.png" });
  const image = within(preview).getByRole("img", { name: "Кухня.png" });
  expect(image.getAttribute("src")).toContain("/api/v1/chat-images/chat-1/image-1");
  fireEvent.error(image);
  expect(within(preview).getByRole("alert").textContent).toContain("не загрузилось");
  fireEvent.click(within(preview).getByRole("button", { name: "Закрыть просмотр" }));

  const input = screen.getByLabelText("Прикрепить изображение");
  fireEvent.change(input, { target: { files: [] } });
  fireEvent.change(input, { target: { files: [new File(["exe"], "bad.exe", { type: "application/octet-stream" })] } });
  await screen.findByText(/Выберите изображение JPEG/);
  fireEvent.change(input, { target: { files: [new File(["png"], "Эскиз.png", { type: "image/png" })] } });
  await screen.findByText("Не удалось загрузить файл");
  failUpload = false;
  fireEvent.click(screen.getByRole("button", { name: "Повторить загрузку" }));
  await screen.findByRole("button", { name: "Просмотреть Эскиз.png" });
  expect(uploadKeys).toEqual([uploadKeys[0], uploadKeys[0]]);
  expect(uploadKeys[0]).toMatch(/^[a-f0-9-]{36}$/);

  fireEvent.click(screen.getByRole("button", { name: "Закрепить Кухня.png" }));
  await screen.findByText("Закреплено");
  fireEvent.click(screen.getByRole("button", { name: "Открепить Кухня.png" }));
  await waitFor(() => expect(screen.queryByText("Закреплено")).toBeNull());
  fireEvent.click(screen.getByRole("button", { name: "Удалить Кухня.png" }));
  const deletion = screen.getByRole("dialog", { name: "Удаление изображения" });
  fireEvent.click(within(deletion).getByRole("button", { name: "Удалить" }));
  expect(await within(deletion).findByRole("alert")).toBeTruthy();
  failDelete = false;
  fireEvent.click(within(deletion).getByRole("button", { name: "Удалить" }));
  await screen.findByText("Изображение удалено");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Просмотреть Кухня.png" })).toBeNull());
});

function json(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } });
}
