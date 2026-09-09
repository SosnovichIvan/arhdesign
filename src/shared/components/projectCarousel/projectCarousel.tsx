"use client";

import Image from "next/image";
import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";

import { CarouselControl, IconButton } from "@/shared/ui";
import { cn } from "@/shared/lib";

type ProjectCarouselProps = { className?: string; imageClassName?: string; images: string[]; title: string; };

export function ProjectCarousel({ className, imageClassName, images, title }: ProjectCarouselProps) {
  const [current, setCurrent] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const lastManualAction = useRef(0);
  const pointerStartX = useRef<number | null>(null);
  const didSwipe = useRef(false);
  const closeFullscreenRef = useRef<HTMLButtonElement>(null);
  const show = (index: number, manual = false) => {
    if (manual) lastManualAction.current = Date.now();
    setCurrent((index + images.length) % images.length);
  };

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (Date.now() - lastManualAction.current >= 10_000) setCurrent((index) => (index + 1) % images.length);
    }, 5_000);
    return () => window.clearInterval(timer);
  }, [images.length]);

  useEffect(() => {
    if (!isFullscreen) return;
    closeFullscreenRef.current?.focus();
    function handleEscape(event: globalThis.KeyboardEvent) { if (event.key === "Escape") setIsFullscreen(false); }
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isFullscreen]);

  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key === "ArrowLeft") { event.preventDefault(); show(current - 1, true); }
    if (event.key === "ArrowRight") { event.preventDefault(); show(current + 1, true); }
    if (event.key === "Home") { event.preventDefault(); show(0, true); }
    if (event.key === "End") { event.preventDefault(); show(images.length - 1, true); }
  }

  function handlePointerDown(event: PointerEvent<HTMLButtonElement>) { pointerStartX.current = event.clientX; didSwipe.current = false; }
  function handlePointerUp(event: PointerEvent<HTMLButtonElement>) {
    if (pointerStartX.current === null) return;
    const distance = event.clientX - pointerStartX.current;
    pointerStartX.current = null;
    if (Math.abs(distance) < 40) return;
    didSwipe.current = true;
    show(current + (distance < 0 ? 1 : -1), true);
  }
  function openFullscreen() {
    if (didSwipe.current) { didSwipe.current = false; return; }
    setIsFullscreen(true);
  }

  return <section aria-label={`Галерея: ${title}`} className={className} onKeyDown={handleKeyDown} role="region" tabIndex={0}>
    <button aria-label={`Открыть изображение ${current + 1} на полный экран`} className="block w-full cursor-zoom-in touch-pan-y" onClick={openFullscreen} onPointerDown={handlePointerDown} onPointerUp={handlePointerUp} type="button">
      <Image alt={`${title}, кадр ${current + 1}`} className={cn("animate-[carousel-dissolve_300ms_ease-out] aspect-[16/10] w-full select-none object-cover", imageClassName)} draggable={false} height={900} key={images[current]} loading="eager" sizes="(min-width: 1440px) 1200px, (min-width: 768px) 90vw, 100vw" src={images[current]} width={1440} />
    </button>
    <div className="mt-3 flex items-center gap-3"><span aria-live="polite" className="text-xs text-secondary">{String(current + 1).padStart(2, "0")} / {String(images.length).padStart(2, "0")}</span><CarouselControl direction="previous" onClick={() => show(current - 1, true)} /><CarouselControl direction="next" onClick={() => show(current + 1, true)} /></div>
    <div aria-label="Миниатюры галереи" className="mt-4 flex gap-2 overflow-x-auto" role="list">
      {images.map((image, index) => <div key={image} role="listitem"><button aria-current={index === current} aria-label={`Показать кадр ${index + 1}`} className={cn("relative size-12 shrink-0 cursor-pointer border-2 border-transparent p-0.5 transition-all duration-300 hover:border-action focus-visible:border-action", index === current && "border-action")} onClick={() => show(index, true)} type="button"><Image alt="" className="animate-[carousel-dissolve_300ms_ease-out] object-cover transition-opacity duration-300" fill key={`${image}-${current}`} sizes="48px" src={image} /></button></div>)}
    </div>
    {isFullscreen ? <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/95 p-6" onClick={() => setIsFullscreen(false)} role="presentation"><section aria-label={`Полноэкранный просмотр: ${title}`} className="relative flex h-full w-full items-center justify-center" onClick={(event) => event.stopPropagation()} role="dialog"><IconButton aria-label="Закрыть полноэкранный просмотр" className="absolute right-0 top-0 z-10 border-white/40 bg-black/50 text-white hover:bg-white/15" onClick={() => setIsFullscreen(false)} ref={closeFullscreenRef}>×</IconButton><Image alt={`${title}, кадр ${current + 1}`} className="max-h-full max-w-full object-contain" height={1440} sizes="100vw" src={images[current]} width={2200} /></section></div> : null}
  </section>;
}
