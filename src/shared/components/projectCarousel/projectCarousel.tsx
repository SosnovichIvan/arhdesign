"use client";

import Image from "next/image";
import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";

import { Button, CarouselControl, Dialog } from "@/shared/ui";
import { cn } from "@/shared/lib";

type ProjectCarouselProps = {
  className?: string;
  imageClassName?: string;
  images: string[];
  title: string;
};

export function ProjectCarousel({ className, imageClassName, images, title }: ProjectCarouselProps) {
  const [current, setCurrent] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [stopped, setStopped] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [hasFocus, setHasFocus] = useState(false);
  const galleryRef = useRef<HTMLElement>(null);
  const thumbnailRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const pointerStartX = useRef<number | null>(null);
  const didSwipe = useRef(false);
  const lastManualAction = useRef(0);
  const playing = !stopped && !reducedMotion;
  const autoAdvancing = playing && !hasFocus && !isFullscreen;

  useEffect(() => {
    if (!window.matchMedia) return;
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    function updateMotion() { setReducedMotion(preference.matches); }
    updateMotion();
    preference.addEventListener("change", updateMotion);
    return () => preference.removeEventListener("change", updateMotion);
  }, []);

  useEffect(() => {
    if (images.length < 2 || !autoAdvancing) return;
    const timer = window.setInterval(() => {
      if (Date.now() - lastManualAction.current >= 10_000) setCurrent((index) => (index + 1) % images.length);
    }, 5_000);
    return () => window.clearInterval(timer);
  }, [autoAdvancing, images.length]);

  function show(index: number) {
    lastManualAction.current = Date.now();
    const next = (index + images.length) % images.length;
    setCurrent(next);
    if (thumbnailRefs.current.includes(document.activeElement as HTMLButtonElement)) {
      thumbnailRefs.current[next]?.focus();
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    const next = { ArrowLeft: current - 1, ArrowRight: current + 1, Home: 0, End: images.length - 1 }[event.key];
    if (next === undefined || !images.length) return;
    event.preventDefault();
    show(next);
  }

  function handlePointerDown(event: PointerEvent<HTMLButtonElement>) {
    pointerStartX.current = event.clientX;
    didSwipe.current = false;
  }

  function handlePointerUp(event: PointerEvent<HTMLButtonElement>) {
    if (pointerStartX.current === null) return;
    const distance = event.clientX - pointerStartX.current;
    pointerStartX.current = null;
    if (Math.abs(distance) < 40) return;
    didSwipe.current = true;
    show(current + (distance < 0 ? 1 : -1));
  }

  function openFullscreen() {
    if (didSwipe.current) {
      didSwipe.current = false;
      return;
    }
    setIsFullscreen(true);
  }

  if (!images.length) return null;

  return (
    <section aria-label={`Галерея: ${title}`} className={className} onBlur={(event) => {
      if (!galleryRef.current?.contains(event.relatedTarget)) setHasFocus(false);
    }} onFocus={() => setHasFocus(true)} onKeyDown={handleKeyDown} ref={galleryRef} role="region" tabIndex={0}>
      <button aria-label={`Открыть изображение ${current + 1} на полный экран`} className="block w-full cursor-zoom-in touch-pan-y" onClick={openFullscreen} onPointerDown={handlePointerDown} onPointerUp={handlePointerUp} type="button">
        <Image alt={`${title}, кадр ${current + 1}`} className={cn("animate-[carousel-dissolve_300ms_ease-out] aspect-[16/10] w-full select-none object-cover", imageClassName)} draggable={false} height={900} key={images[current]} loading="eager" sizes="(min-width: 1440px) 1200px, (min-width: 768px) 90vw, 100vw" src={images[current]} width={1440} />
      </button>
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <span aria-live={autoAdvancing ? "off" : "polite"} className="text-xs text-secondary">{String(current + 1).padStart(2, "0")} / {String(images.length).padStart(2, "0")}</span>
        <CarouselControl direction="previous" onClick={() => show(current - 1)} />
        <CarouselControl direction="next" onClick={() => show(current + 1)} />
        {!reducedMotion && images.length > 1 ? <Button onClick={() => setStopped((value) => !value)} type="button" variant="secondary">{stopped ? "Продолжить показ" : "Пауза"}</Button> : null}
      </div>
      <div aria-label="Миниатюры галереи" className="mt-4 flex gap-2 overflow-x-auto" role="list">
        {images.map((src, index) => (
          <div key={src} role="listitem">
            <button aria-current={index === current} aria-label={`Показать кадр ${index + 1}`} className={cn("relative size-12 shrink-0 cursor-pointer border-2 border-transparent p-0.5 hover:border-action focus-visible:border-action", index === current && "border-action")} onClick={() => show(index)} ref={(button) => { thumbnailRefs.current[index] = button; }} tabIndex={index === current ? 0 : -1} type="button">
              <Image alt="" className="object-cover" fill sizes="48px" src={src} />
            </button>
          </div>
        ))}
      </div>
      <Dialog className="max-w-6xl" closeLabel="Закрыть полноэкранный просмотр" isOpen={isFullscreen} label={`Полноэкранный просмотр: ${title}`} onClose={() => setIsFullscreen(false)}>
        <Image alt={`${title}, кадр ${current + 1}`} className="mx-auto max-h-[70dvh] w-auto max-w-full object-contain" height={1440} sizes="100vw" src={images[current]} width={2200} />
        <div className="mt-3 flex justify-center gap-3">
          <CarouselControl direction="previous" onClick={() => show(current - 1)} />
          <CarouselControl direction="next" onClick={() => show(current + 1)} />
        </div>
      </Dialog>
    </section>
  );
}
