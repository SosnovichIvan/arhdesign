"use client";

import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

import { cn } from "@/shared/lib";

export type AutocompleteOption = {
  description?: string;
  id: string;
  label: string;
};

type AutocompleteProps<T extends AutocompleteOption> = {
  autoFocus?: boolean;
  className?: string;
  dataCy?: string;
  disabled?: boolean;
  emptyText?: string;
  error?: string;
  label: string;
  loading?: boolean;
  onSelect: (option: T) => void;
  onValueChange: (value: string) => void;
  options: readonly T[];
  placeholder?: string;
  required?: boolean;
  value: string;
};

export function Autocomplete<T extends AutocompleteOption>({
  autoFocus = false,
  className,
  dataCy,
  disabled = false,
  emptyText = "Ничего не найдено. Попробуйте изменить запрос.",
  error,
  label,
  loading = false,
  onSelect,
  onValueChange,
  options,
  placeholder,
  required = false,
  value,
}: Readonly<AutocompleteProps<T>>) {
  const labelId = useId();
  const listboxId = useId();
  const errorId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const suppressOpenOnFocusRef = useRef(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [isOpen, setIsOpen] = useState(false);
  const effectiveActiveIndex = Math.min(activeIndex, Math.max(0, options.length - 1));

  useEffect(() => {
    if (!isOpen) return;
    function closeOnOutsidePointer(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setIsOpen(false);
    }
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    return () => document.removeEventListener("pointerdown", closeOnOutsidePointer);
  }, [isOpen]);

  function choose(option: T) {
    onSelect(option);
    setIsOpen(false);
    if (document.activeElement !== inputRef.current) {
      suppressOpenOnFocusRef.current = true;
      inputRef.current?.focus();
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      setIsOpen(true);
      if (options.length === 0) return;
      const direction = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex((current) => (Math.min(current, options.length - 1) + direction + options.length) % options.length);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      if (!isOpen || options.length === 0) return;
      event.preventDefault();
      setActiveIndex(event.key === "Home" ? 0 : options.length - 1);
      return;
    }
    if (event.key === "Enter" && isOpen && options[effectiveActiveIndex]) {
      event.preventDefault();
      choose(options[effectiveActiveIndex]);
      return;
    }
    if (event.key === "Escape" && isOpen) {
      event.preventDefault();
      setIsOpen(false);
    }
  }

  return (
    <div className={cn("relative grid gap-2 text-sm font-medium", className)} ref={rootRef}>
      <label htmlFor={`${listboxId}-input`} id={labelId}>{label}{required ? <span aria-hidden="true"> *</span> : null}</label>
      <div className={cn("flex min-h-12 items-center border-b border-border transition-colors hover:border-action focus-within:border-field-focus", error && "border-red-700 hover:border-red-700 focus-within:border-red-700", disabled && "opacity-50")}>
        <input
          aria-activedescendant={isOpen && options[effectiveActiveIndex] ? `${listboxId}-option-${effectiveActiveIndex}` : undefined}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-describedby={error ? errorId : undefined}
          aria-expanded={isOpen}
          aria-invalid={Boolean(error)}
          aria-labelledby={labelId}
          aria-required={required}
          autoComplete="off"
          autoFocus={autoFocus}
          className="min-h-11 min-w-0 flex-1 bg-transparent px-3 py-3 text-base text-primary placeholder:text-secondary placeholder:opacity-100 focus:outline-none disabled:cursor-not-allowed"
          data-cy={dataCy}
          disabled={disabled}
          id={`${listboxId}-input`}
          onChange={(event) => { onValueChange(event.target.value); setActiveIndex(0); setIsOpen(true); }}
          onFocus={() => {
            if (suppressOpenOnFocusRef.current) {
              suppressOpenOnFocusRef.current = false;
              return;
            }
            setIsOpen(true);
          }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          ref={inputRef}
          role="combobox"
          type="text"
          value={value}
        />
        {value && !disabled ? <button aria-label={`Очистить поле «${label}»`} className="inline-flex size-11 shrink-0 items-center justify-center text-secondary transition-colors hover:text-primary" onClick={() => { onValueChange(""); setActiveIndex(0); setIsOpen(true); inputRef.current?.focus(); }} type="button">×</button> : null}
      </div>
      {error ? <p aria-live="polite" className="text-sm font-normal text-red-700" id={errorId} role="alert">{error}</p> : null}
      {isOpen ? (
        <div aria-labelledby={labelId} className="absolute inset-x-0 top-full z-30 mt-2 max-h-72 overflow-y-auto border border-border bg-surface p-1 shadow-surface" id={listboxId} role="listbox">
          {loading ? <p aria-live="polite" className="px-3 py-4 text-sm font-normal text-secondary" role="status">Ищем пользователей…</p> : null}
          {!loading && options.length === 0 ? <p className="px-3 py-4 text-sm font-normal text-secondary">{emptyText}</p> : null}
          {!loading ? options.map((option, index) => (
            <button
              aria-selected={index === effectiveActiveIndex}
              className={cn("flex min-h-12 w-full flex-col items-start justify-center px-3 py-2 text-left font-normal transition-colors hover:bg-page", index === effectiveActiveIndex && "bg-page")}
              id={`${listboxId}-option-${index}`}
              key={option.id}
              onClick={() => choose(option)}
              onMouseEnter={() => setActiveIndex(index)}
              role="option"
              tabIndex={-1}
              type="button"
            >
              <span className="text-sm font-medium text-primary">{option.label}</span>
              {option.description ? <span className="mt-1 text-xs text-secondary">{option.description}</span> : null}
            </button>
          )) : null}
        </div>
      ) : null}
    </div>
  );
}
