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
  closeLabel?: string;
  closeButtonClassName?: string;
  contentClassName?: string;
  placement?: "center" | "right";
};

export function Dialog({ children, className, closeButtonClassName, contentClassName, closeLabel = "Закрыть форму", isOpen, label, onClose, placement = "center" }: DialogProps) {
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
      const targetDialog = document.activeElement?.closest('[role="dialog"]');
      if (event.key === "Escape" && !event.defaultPrevented && !dialogRef.current?.querySelector('[role="dialog"]') && (!targetDialog || targetDialog === dialogRef.current)) onCloseRef.current();
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
    const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>("button:not([disabled]), input:not([disabled]):not([type=hidden]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]") ?? [])
      .filter((element) => element.tabIndex >= 0 && !element.closest("[hidden], .hidden, [inert]"));
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
    <div className={cn("fixed inset-0 z-50 grid bg-overlay backdrop-blur-[2px]", placement === "right" ? "justify-items-end" : "place-items-center p-4 tablet:p-7")} data-dialog-overlay onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }} role="presentation">
      <section aria-label={label} aria-modal="true" className={cn("relative flex max-h-full w-full flex-col overflow-hidden border border-border bg-surface shadow-surface", placement === "right" ? "h-dvh max-w-[540px]" : "max-w-[640px]", className)} onKeyDown={handleTabKey} onMouseDown={(event) => event.stopPropagation()} ref={dialogRef} role="dialog">
        <div className="flex shrink-0 justify-end px-4 pt-3 pb-1 tablet:px-6">
          <IconButton aria-label={closeLabel} className={cn("shrink-0 text-secondary", closeButtonClassName)} onClick={onClose} ref={closeButtonRef}>×</IconButton>
        </div>
        <div aria-label={placement === "right" ? "Содержимое панели" : undefined} className={cn("min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 pb-6 tablet:px-12 tablet:pb-12", placement === "right" && "focus-visible:outline-none focus-visible:[box-shadow:inset_3px_0_var(--color-focus)] forced-colors:focus-visible:outline", contentClassName)} role={placement === "right" ? "region" : undefined} tabIndex={placement === "right" ? 0 : undefined}>
          {children}
        </div>
      </section>
    </div>
  );
}
