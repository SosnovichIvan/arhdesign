"use client";

import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";

import { cn } from "@/shared/lib";
import { IconButton } from "@/shared/ui/iconButton";

type DialogProps = {
  children: ReactNode;
  isOpen: boolean;
  label: string;
  onClose: () => void;
  className?: string;
};

export function Dialog({ children, className, isOpen, label, onClose }: DialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const onCloseRef = useRef(onClose);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!isOpen) return;
    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    (dialogRef.current?.querySelector<HTMLElement>("[data-dialog-initial-focus]") ?? closeButtonRef.current)?.focus();
    function handleEscape(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") onCloseRef.current();
    }
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("keydown", handleEscape);
      document.body.style.overflow = previousOverflow;
      previousFocusRef.current?.focus();
    };
  }, [isOpen]);

  function handleTabKey(event: KeyboardEvent<HTMLElement>) {
    if (event.key !== "Tab") return;
    const focusable = dialogRef.current?.querySelectorAll<HTMLElement>("button:not([disabled]):not([tabindex='-1']), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href]");
    if (!focusable?.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-overlay p-4 backdrop-blur-[2px] tablet:p-7" data-dialog-overlay onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }} role="presentation">
      <section aria-label={label} aria-modal="true" className={cn("relative max-h-full w-full max-w-[640px] overflow-y-auto border border-border bg-surface p-6 shadow-surface tablet:p-12", className)} onKeyDown={handleTabKey} onMouseDown={(event) => event.stopPropagation()} ref={dialogRef} role="dialog">
        <IconButton aria-label="Закрыть форму" className="absolute right-4 top-4 text-secondary tablet:right-6 tablet:top-6" onClick={onClose} ref={closeButtonRef}>×</IconButton>
        {children}
      </section>
    </div>
  );
}
