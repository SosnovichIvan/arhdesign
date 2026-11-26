"use client";

import Image from "next/image";
import { useEffect, useState, type ChangeEvent } from "react";

import { Button, Dialog } from "@/shared/ui";

import { chatImageURL, deleteChatImage, loadChatImages, setChatImagePin, uploadChatImage, type ChatImage, type ChatImagePage, type ChatLocation } from "../api/chatImages";

type PendingUpload = { file: File; key: string };
const supportedImages = ["image/jpeg", "image/png", "image/gif"];

export function ChatImages({ chatId, projectId }: ChatLocation) {
  const [open, setOpen] = useState(false);
  const [page, setPage] = useState<ChatImagePage | null>(null);
  const [revision, setRevision] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const [busy, setBusy] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState<PendingUpload | null>(null);
  const [preview, setPreview] = useState<ChatImage | null>(null);
  const [previewError, setPreviewError] = useState(false);
  const [deleting, setDeleting] = useState<ChatImage | null>(null);
  const location = { chatId, projectId };

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    void loadChatImages({ chatId, projectId }, undefined, controller.signal).then((result) => {
      if (!controller.signal.aborted) setPage(result);
    }, (reason: unknown) => {
      if (!controller.signal.aborted) setLoadError(errorMessage(reason, "Не удалось загрузить изображения"));
    });
    return () => controller.abort();
  }, [chatId, open, projectId, revision]);

  function reload() {
    setLoadError("");
    setRevision((value) => value + 1);
  }

  async function mutate(action: () => Promise<void>, success: string) {
    if (busy) return;
    setBusy(true);
    setActionError("");
    setStatus("");
    try {
      await action();
      setStatus(success);
      reload();
    } catch (reason) {
      setActionError(errorMessage(reason, "Не удалось выполнить действие"));
    } finally {
      setBusy(false);
    }
  }

  function pickFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = "";
    if (!file) return;
    setActionError("");
    if ((file.type && !supportedImages.includes(file.type)) || !/\.(jpe?g|png|gif)$/i.test(file.name) || file.size < 1 || file.size > 10 * 1024 * 1024) {
      setUpload(null);
      setActionError("Выберите изображение JPEG, PNG или GIF размером до 10 МБ");
      return;
    }
    const selection = { file, key: crypto.randomUUID() };
    setUpload(selection);
    void attach(selection);
  }

  async function attach(selection: PendingUpload) {
    await mutate(async () => {
      await uploadChatImage(location, selection.file, selection.key);
      setUpload(null);
    }, "Изображение прикреплено");
  }

  async function showMore() {
    if (!page?.nextCursor || loadingMore) return;
    setLoadingMore(true);
    setLoadError("");
    try {
      const next = await loadChatImages(location, page.nextCursor);
      setPage({ ...next, items: [...page.items, ...next.items.filter((item) => !page.items.some((previous) => previous.id === item.id))] });
    } catch (reason) {
      setLoadError(errorMessage(reason, "Не удалось загрузить ещё изображения"));
    } finally {
      setLoadingMore(false);
    }
  }

  return (
    <details className="border-b border-border px-4 py-3" data-cy="chat-images" onToggle={(event) => setOpen(event.currentTarget.open)}>
      <summary className="min-h-8 cursor-pointer py-1 text-sm font-medium">Изображения чата</summary>
      {open ? (
        <div className="pt-2">
          <label className="inline-flex min-h-11 cursor-pointer items-center rounded-full border border-border px-4 text-sm hover:bg-page focus-within:outline-2 focus-within:outline-focus">
            <input accept="image/jpeg,image/png,image/gif" aria-label="Прикрепить изображение" className="sr-only" disabled={busy || loadingMore} onChange={pickFile} type="file" />
            {busy && upload ? "Загружаем…" : "Прикрепить изображение"}
          </label>
          <p className="mt-2 text-xs text-secondary">JPEG, PNG, GIF · до 10 МБ</p>
          {status ? <p className="mt-3 text-sm" role="status">{status}</p> : null}
          {actionError ? <p className="mt-3 text-sm text-danger" role="alert">{actionError}</p> : null}
          {upload && !busy ? <Button className="mt-2" onClick={() => void attach(upload)} type="button" variant="secondary">Повторить загрузку</Button> : null}
          {loadError ? <div className="mt-3"><p className="text-sm text-danger" role="alert">{loadError}</p><Button onClick={reload} type="button" variant="text">Обновить список</Button></div> : null}
          {!page && !loadError ? <p className="mt-4 text-sm text-secondary" role="status">Загружаем изображения…</p> : null}
          {page?.items.length === 0 ? <p className="mt-4 text-sm text-secondary">Изображений пока нет</p> : null}
          {page?.items.length ? (
            <ul className="mt-4 grid gap-3 sm:grid-cols-2 desktop:grid-cols-3">
              {page.items.map((image) => (
                <li className="min-w-0 border border-border bg-page p-2" key={image.id}>
                  <button aria-label={`Просмотреть ${image.name}`} className="relative block aspect-square w-full overflow-hidden bg-surface" onClick={() => { setPreviewError(false); setPreview(image); }} type="button">
                    <Image alt="" className="object-cover" fill sizes="(min-width: 1024px) 240px, 80vw" src={chatImageURL(location, image.id)} unoptimized />
                  </button>
                  <p className="mt-2 wrap-anywhere text-xs font-medium">{image.name}</p>
                  {image.pinned ? <p className="mt-1 text-xs text-action">Закреплено</p> : null}
                  <div className="flex flex-wrap gap-x-3">
                    <Button aria-label={`${image.pinned ? "Открепить" : "Закрепить"} ${image.name}`} disabled={busy || loadingMore} onClick={() => void mutate(() => setChatImagePin(location, image), image.pinned ? "Изображение откреплено" : "Изображение закреплено")} type="button" variant="text">{image.pinned ? "Открепить" : "Закрепить"}</Button>
                    {image.canDelete ? <Button aria-label={`Удалить ${image.name}`} className="text-danger" disabled={busy || loadingMore} onClick={() => { setActionError(""); setDeleting(image); }} type="button" variant="text">Удалить</Button> : null}
                  </div>
                </li>
              ))}
            </ul>
          ) : null}
          {page?.nextCursor ? <Button className="mt-3" disabled={busy || loadingMore} onClick={() => void showMore()} type="button" variant="secondary">Показать ещё</Button> : null}
          <Dialog className="max-w-5xl" closeLabel="Закрыть просмотр" isOpen={preview !== null} label={preview ? `Просмотр ${preview.name}` : "Просмотр изображения"} onClose={() => setPreview(null)}>
            {preview ? (
              previewError ? <p className="text-danger" role="alert">Изображение не загрузилось или было удалено.</p> :
                <Image alt={preview.name} className="mx-auto max-h-[70dvh] w-auto max-w-full object-contain" height={preview.height} onError={() => setPreviewError(true)} src={chatImageURL(location, preview.id)} unoptimized width={preview.width} />
            ) : null}
          </Dialog>
          <Dialog isOpen={deleting !== null} label="Удаление изображения" onClose={() => { if (!busy) setDeleting(null); }}>
            {deleting ? (
              <div>
                <h2 className="font-display text-3xl">Удалить изображение?</h2>
                <p className="mt-3 wrap-anywhere text-sm text-secondary">{deleting.name} исчезнет из чата у всех участников.</p>
                {actionError ? <p className="mt-3 text-sm text-danger" role="alert">{actionError}</p> : null}
                <div className="mt-5 flex flex-wrap gap-3">
                  <Button data-dialog-initial-focus disabled={busy} onClick={() => setDeleting(null)} type="button" variant="secondary">Отмена</Button>
                  <Button disabled={busy} onClick={() => void mutate(async () => { await deleteChatImage(location, deleting.id); setDeleting(null); }, "Изображение удалено")} type="button">Удалить</Button>
                </div>
              </div>
            ) : null}
          </Dialog>
        </div>
      ) : null}
    </details>
  );
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof Error ? reason.message : fallback;
}
