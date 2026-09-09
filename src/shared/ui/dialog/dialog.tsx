"use client";

import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";

import { IconButton } from "@/shared/ui/iconButton";

type DialogProps = {
  children: ReactNode;
  isOpen: boolean;
  label: string;
  onClose: () => void;
};

export function Dialog({ children, isOpen, label, onClose }: DialogProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    closeButtonRef.current?.focus();
    function handleEscape(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isOpen, onClose]);

  function handleTabKey(event: KeyboardEvent<HTMLElement>) {
    if (event.key !== "Tab") return;
    const focusable = dialogRef.current?.querySelectorAll<HTMLElement>("button:not([disabled]), input:not([disabled]), textarea:not([disabled]), a[href]");
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
    <div className="fixed inset-0 z-50 grid place-items-center bg-inverse/95 p-6 tablet:p-7" role="presentation">
      <section aria-label={label} aria-modal="true" className="relative max-h-full w-full max-w-[640px] overflow-y-auto border border-border bg-surface p-6 shadow-surface tablet:p-12" onKeyDown={handleTabKey} ref={dialogRef} role="dialog">
        <IconButton aria-label="Закрыть форму" className="absolute right-4 top-4 text-secondary tablet:right-6 tablet:top-6" onClick={onClose} ref={closeButtonRef}>×</IconButton>
        {children}
      </section>
    </div>
  );
}
