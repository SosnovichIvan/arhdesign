"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";

import { Dialog } from "@/shared/ui";

import { loadProjectDocumentBlob } from "../api/projects";

type PreviewKind = "image" | "pdf" | "unsupported";
type DocumentPreviewState = { status: "loading" } | { message: string; status: "error" } | { status: "ready"; url: string };
type StoredDocumentPreviewState = ({ message: string; status: "error" } | { status: "ready"; url: string }) & { requestKey: string };

export function ProjectDocumentDiscussionPreview({ documentId, name, projectId }: Readonly<{ documentId: string; name: string; projectId: string }>) {
  const [isOpen, setIsOpen] = useState(false);
  const [retryKey, setRetryKey] = useState(0);
  const kind = useMemo(() => previewKind(name), [name]);
  const previewState = useAuthorizedDocumentPreview(projectId, documentId, retryKey);
  const canOpenPreview = previewState.status === "ready";
  const viewerUrl = previewState.status === "ready" ? previewState.url : "";

  return <>
    <section aria-label={`Документ обсуждения «${name}»`} className="mb-6 border border-border bg-page p-3 tablet:p-4">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0"><p className="text-[10px] font-semibold uppercase tracking-[0.12em] text-action">Документ обсуждения</p><p className="mt-1 truncate text-sm font-medium">{name}</p></div>
        <span className="shrink-0 text-xs text-secondary">{kind === "pdf" ? "PDF" : kind === "image" ? "Изображение" : "Файл"}</span>
      </div>
      <div className="relative mt-3 h-48 overflow-hidden border border-border bg-surface tablet:h-60">
        <DocumentSurface kind={kind} name={name} onRetry={() => setRetryKey((value) => value + 1)} preview state={previewState} url={viewerUrl} />
        {canOpenPreview ? <button aria-label={`Открыть документ «${name}» на весь экран`} className="absolute inset-0 cursor-zoom-in ring-inset transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-action" data-cy="open-document-preview" onClick={() => setIsOpen(true)} type="button"><span className="sr-only">Открыть на весь экран</span></button> : null}
      </div>
    </section>
    <Dialog className="h-[calc(100dvh-2rem)] max-w-none overflow-hidden p-0 tablet:h-[calc(100dvh-3.5rem)] tablet:p-0" closeButtonClassName="right-3 top-3 size-8 text-lg leading-none tablet:right-4 tablet:top-4" closeLabel="Закрыть полноэкранный просмотр" isOpen={isOpen} label={`Полноэкранный просмотр «${name}»`} onClose={() => setIsOpen(false)}>
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex min-h-16 items-center gap-4 border-b border-border px-4 pr-16 tablet:px-6 tablet:pr-20"><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{name}</p><p className="mt-0.5 text-xs text-secondary">Документ проекта</p></div><a className="shrink-0 text-xs font-medium text-action underline underline-offset-4" href={viewerUrl} rel="noreferrer" target="_blank">Открыть отдельно</a></div>
        <div className="min-h-0 flex-1 bg-black/90 p-2 tablet:p-4"><DocumentSurface kind={kind} name={name} onRetry={() => setRetryKey((value) => value + 1)} state={previewState} url={viewerUrl} /></div>
      </div>
    </Dialog>
  </>;
}

function DocumentSurface({ kind, name, onRetry, preview = false, state, url }: Readonly<{ kind: PreviewKind; name: string; onRetry: () => void; preview?: boolean; state: DocumentPreviewState; url: string }>) {
  if (state.status === "error") return <div className="grid size-full place-items-center bg-surface p-6 text-center" role="alert"><div><p className="text-sm font-medium text-primary">Не удалось загрузить документ</p><p className="mt-2 text-xs leading-5 text-secondary">{state.message}</p><button className="mt-4 min-h-9 border border-border px-4 text-xs font-medium text-primary hover:bg-page focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-action" onClick={onRetry} type="button">Повторить</button></div></div>;
  if (state.status === "loading") return <div aria-label={`Загрузка документа «${name}»`} className="grid size-full place-items-center bg-surface"><span className="text-xs text-secondary">Загрузка документа…</span></div>;
  if (kind === "pdf") return <iframe className={`size-full border-0 bg-white ${preview ? "pointer-events-none" : ""}`} src={`${url}#toolbar=${preview ? "0" : "1"}&navpanes=0&view=FitH`} tabIndex={preview ? -1 : 0} title={`${preview ? "Предпросмотр" : "Полноэкранный просмотр"} документа «${name}»`} />;
  if (kind === "image") return <Image alt={`${preview ? "Предпросмотр" : "Полноэкранный просмотр"} изображения «${name}»`} className={`object-contain ${preview ? "pointer-events-none" : ""}`} fill sizes={preview ? "(min-width: 768px) 720px, calc(100vw - 3rem)" : "100vw"} src={url} unoptimized />;
  return <FileFallback name={name} />;
}

function useAuthorizedDocumentPreview(projectId: string, documentId: string, retryKey: number): DocumentPreviewState {
  const requestKey = `${projectId}:${documentId}:${retryKey}`;
  const [result, setResult] = useState<StoredDocumentPreviewState | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    let objectUrl: string | undefined;
    void loadProjectDocumentBlob(projectId, documentId, controller.signal).then((blob) => {
      if (controller.signal.aborted) return;
      objectUrl = URL.createObjectURL(blob);
      setResult({ requestKey, status: "ready", url: objectUrl });
    }).catch((error: unknown) => {
      if (controller.signal.aborted) return;
      setResult({ message: error instanceof Error ? error.message : "Повторите попытку.", requestKey, status: "error" });
    });
    return () => {
      controller.abort();
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [documentId, projectId, requestKey]);

  return result?.requestKey === requestKey ? result : { status: "loading" };
}

function FileFallback({ name }: Readonly<{ name: string }>) {
  return <div className="grid size-full place-items-center bg-surface p-6 text-center"><div><FileIcon /><p className="mt-3 max-w-md break-words text-sm font-medium text-primary">{name}</p><p className="mt-2 text-xs leading-5 text-secondary">Предпросмотр этого формата недоступен. Откройте файл отдельно.</p></div></div>;
}

function previewKind(name: string): PreviewKind {
  const extension = name.toLocaleLowerCase("ru").split(".").at(-1);
  if (extension === "pdf") return "pdf";
  if (extension && ["avif", "gif", "jpeg", "jpg", "png", "svg", "webp"].includes(extension)) return "image";
  return "unsupported";
}

function FileIcon() {
  return <svg aria-hidden="true" className="mx-auto size-10 text-secondary" fill="none" viewBox="0 0 40 40"><path d="M10 4h13l7 7v25H10V4Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.5"/><path d="M23 4v8h7M15 20h10M15 25h10M15 30h7" stroke="currentColor" strokeLinecap="round" strokeWidth="1.5"/></svg>;
}
