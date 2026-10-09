"use client";

import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/shared/lib";

export type SelectOption<T extends string = string> = {
  label: string;
  value: T;
};

type SelectProps<T extends string> = {
  className?: string;
  dataCy?: string;
  disabled?: boolean;
  label: string;
  onValueChange: (value: T) => void;
  options: readonly [SelectOption<T>, ...SelectOption<T>[]];
  required?: boolean;
  value: T;
};

export function Select<T extends string>({
  className,
  dataCy,
  disabled = false,
  label,
  onValueChange,
  options,
  required = false,
  value,
}: Readonly<SelectProps<T>>) {
  const labelId = useId();
  const listboxId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const selectedIndex = Math.max(options.findIndex((option) => option.value === value), 0);
  const [activeIndex, setActiveIndex] = useState(selectedIndex);
  const [isOpen, setIsOpen] = useState(false);
  const [popupStyle, setPopupStyle] = useState<CSSProperties>({});

  useEffect(() => {
    if (!isOpen) return;
    function closeOnOutsidePointer(event: PointerEvent) {
      const target = event.target as Node;
      if (!rootRef.current?.contains(target) && !listRef.current?.contains(target)) setIsOpen(false);
    }
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    return () => document.removeEventListener("pointerdown", closeOnOutsidePointer);
  }, [isOpen]);

  useLayoutEffect(() => {
    if (!isOpen) return;
    function updatePosition() {
      const rect = triggerRef.current?.getBoundingClientRect();
      if (!rect) return;
      const gap = 8;
      const viewportGap = 8;
      const desiredHeight = Math.min(options.length * 44 + 8, 256);
      const availableBelow = window.innerHeight - rect.bottom - gap - viewportGap;
      const availableAbove = rect.top - gap - viewportGap;
      const openBelow = availableBelow >= Math.min(desiredHeight, 160) || availableBelow >= availableAbove;
      const maxHeight = Math.max(96, Math.min(desiredHeight, openBelow ? availableBelow : availableAbove));
      const width = rect.width;
      const left = Math.min(Math.max(viewportGap, rect.left), Math.max(viewportGap, window.innerWidth - width - viewportGap));
      setPopupStyle({ bottom: openBelow ? undefined : window.innerHeight - rect.top + gap, left, maxHeight, top: openBelow ? rect.bottom + gap : undefined, width });
    }
    updatePosition();
    document.getElementById(`${listboxId}-option-${activeIndex}`)?.scrollIntoView?.({ block: "nearest" });
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [activeIndex, isOpen, listboxId, options.length]);

  function openSelect() {
    setActiveIndex(selectedIndex);
    setIsOpen(true);
  }

  function choose(index: number) {
    onValueChange(options[index].value);
    setActiveIndex(index);
    setIsOpen(false);
    triggerRef.current?.focus();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!isOpen) {
        const direction = event.key === "ArrowDown" ? 1 : -1;
        setActiveIndex((selectedIndex + direction + options.length) % options.length);
        setIsOpen(true);
        return;
      }
      const direction = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex((current) => (current + direction + options.length) % options.length);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      setActiveIndex(event.key === "Home" ? 0 : options.length - 1);
      setIsOpen(true);
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (isOpen) choose(activeIndex);
      else openSelect();
      return;
    }
    if (event.key === "Escape" && isOpen) {
      event.preventDefault();
      setIsOpen(false);
    }
  }

  return (
    <div className={cn("relative grid gap-2 text-sm font-medium", className)} ref={rootRef}>
      <span id={labelId}>{label}{required ? <span aria-hidden="true"> *</span> : null}</span>
      <button
        aria-activedescendant={isOpen ? `${listboxId}-option-${activeIndex}` : undefined}
        aria-controls={listboxId}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-labelledby={labelId}
        aria-required={required}
        className="flex min-h-12 w-full items-center justify-between gap-4 border-b border-border bg-transparent px-3 py-3 text-left text-base text-primary transition-colors hover:border-action focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        data-cy={dataCy}
        disabled={disabled}
        onClick={() => isOpen ? setIsOpen(false) : openSelect()}
        onKeyDown={handleKeyDown}
        ref={triggerRef}
        role="combobox"
        type="button"
      >
        <span>{options[selectedIndex].label}</span>
        <svg aria-hidden="true" className={cn("size-4 shrink-0 transition-transform", isOpen && "rotate-180")} fill="none" viewBox="0 0 16 16">
          <path d="m3.5 6 4.5 4 4.5-4" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" />
        </svg>
      </button>
      {isOpen ? createPortal(
        <div aria-labelledby={labelId} className="fixed z-[70] overflow-y-auto overscroll-contain border border-border bg-surface p-1 shadow-surface" id={listboxId} ref={listRef} role="listbox" style={popupStyle}>
          {options.map((option, index) => {
            const isActive = index === activeIndex;
            const isSelected = index === selectedIndex;
            return (
              <button
                aria-selected={isSelected}
                className={cn("flex min-h-11 w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm text-secondary transition-colors hover:bg-page hover:text-primary", isActive && "bg-page text-primary")}
                data-cy={dataCy ? `${dataCy}-option-${option.value}` : undefined}
                id={`${listboxId}-option-${index}`}
                key={option.value}
                onClick={() => choose(index)}
                onMouseEnter={() => setActiveIndex(index)}
                role="option"
                tabIndex={-1}
                type="button"
              >
                <span>{option.label}</span>
                {isSelected ? <svg aria-hidden="true" className="size-4 text-action" fill="none" viewBox="0 0 16 16"><path d="m3 8.5 3 3 7-7" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" /></svg> : null}
              </button>
            );
          })}
        </div>, document.body,
      ) : null}
    </div>
  );
}
