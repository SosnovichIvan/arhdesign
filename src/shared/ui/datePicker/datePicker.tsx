"use client";

import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";

import { cn } from "@/shared/lib";

type DatePickerProps = {
  className?: string;
  dataCy?: string;
  disabled?: boolean;
  error?: string;
  label: string;
  min?: string;
  onValueChange: (value: string) => void;
  required?: boolean;
  value: string;
};

const months = ["Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"];
const weekdays = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"];
const calendarGap = 8;
const viewportMargin = 16;

type CalendarPosition = {
  left: number;
  maxHeight: number;
  placement: "bottom" | "top";
  top: number;
};

export function DatePicker({ className, dataCy, disabled = false, error, label, min, onValueChange, required = false, value }: Readonly<DatePickerProps>) {
  const labelId = useId();
  const dialogId = useId();
  const errorId = useId();
  const calendarRef = useRef<HTMLDivElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selectedDate = parseDate(value);
  const [isOpen, setIsOpen] = useState(false);
  const [calendarPosition, setCalendarPosition] = useState<CalendarPosition | null>(null);
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(selectedDate ?? new Date()));

  useLayoutEffect(() => {
    if (!isOpen) return;

    function updateCalendarPosition() {
      const trigger = triggerRef.current;
      const calendar = calendarRef.current;
      if (!trigger || !calendar) return;

      const triggerRect = trigger.getBoundingClientRect();
      const calendarRect = calendar.getBoundingClientRect();
      const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
      const viewportHeight = document.documentElement.clientHeight || window.innerHeight;
      const calendarWidth = Math.min(calendarRect.width, Math.max(0, viewportWidth - viewportMargin * 2));
      const maxLeft = Math.max(viewportMargin, viewportWidth - calendarWidth - viewportMargin);
      const left = clamp(triggerRect.left, viewportMargin, maxLeft);
      const spaceBelow = Math.max(0, viewportHeight - triggerRect.bottom - calendarGap - viewportMargin);
      const spaceAbove = Math.max(0, triggerRect.top - calendarGap - viewportMargin);
      const desiredHeight = Math.min(calendarRect.height, Math.max(0, viewportHeight - viewportMargin * 2));
      const placement = spaceBelow >= desiredHeight || spaceBelow >= spaceAbove ? "bottom" : "top";
      const maxHeight = placement === "bottom" ? spaceBelow : spaceAbove;
      const top = placement === "bottom"
        ? triggerRect.bottom + calendarGap
        : triggerRect.top - calendarGap - Math.min(calendarRect.height, maxHeight);

      setCalendarPosition({ left, maxHeight, placement, top: Math.max(viewportMargin, top) });
    }

    updateCalendarPosition();
    window.addEventListener("resize", updateCalendarPosition);
    window.addEventListener("scroll", updateCalendarPosition, true);
    return () => {
      window.removeEventListener("resize", updateCalendarPosition);
      window.removeEventListener("scroll", updateCalendarPosition, true);
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    function closeOnOutsidePointer(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setIsOpen(false);
    }
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      setIsOpen(false);
      triggerRef.current?.focus();
    }
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  const days = useMemo(() => calendarDays(visibleMonth), [visibleMonth]);
  const minDate = parseDate(min ?? "");

  function openCalendar() {
    setVisibleMonth(startOfMonth(selectedDate ?? minDate ?? new Date()));
    setCalendarPosition(null);
    setIsOpen(true);
  }

  function chooseDate(date: Date) {
    onValueChange(toDateValue(date));
    setIsOpen(false);
    triggerRef.current?.focus();
  }

  return (
    <div className={cn("relative grid gap-2 text-sm font-medium", className)} ref={rootRef}>
      <span id={labelId}>{label}{required ? <span aria-hidden="true"> *</span> : null}</span>
      <div className={cn("flex min-h-12 items-center border-b border-border transition-colors hover:border-action focus-within:border-field-focus", disabled && "opacity-50", error && "border-red-700 hover:border-red-700 focus-within:border-red-700")}>
        <button
          aria-controls={dialogId}
          aria-describedby={error ? errorId : undefined}
          aria-expanded={isOpen}
          aria-haspopup="dialog"
          aria-invalid={Boolean(error)}
          aria-labelledby={labelId}
          className="flex min-h-11 min-w-0 flex-1 items-center pl-3 text-left text-base text-primary focus:outline-none focus-visible:outline-none disabled:cursor-not-allowed"
          data-cy={dataCy}
          disabled={disabled}
          onClick={() => isOpen ? setIsOpen(false) : openCalendar()}
          ref={triggerRef}
          role="combobox"
          type="button"
        >
          <span className={cn("truncate", !selectedDate && "text-secondary")}>{selectedDate ? formatDate(selectedDate) : "Выберите дату"}</span>
        </button>
        {value && !disabled ? <button aria-label={`Очистить поле «${label}»`} className="inline-flex size-11 shrink-0 items-center justify-center text-secondary transition-colors hover:text-primary" onClick={() => onValueChange("")} type="button"><span aria-hidden="true">×</span></button> : null}
        <button aria-label={`Открыть календарь: ${label}`} className="inline-flex size-11 shrink-0 items-center justify-center text-action transition-colors hover:text-primary disabled:cursor-not-allowed" disabled={disabled} onClick={() => isOpen ? setIsOpen(false) : openCalendar()} tabIndex={-1} type="button">
          <CalendarIcon />
        </button>
      </div>
      {error ? <p aria-live="polite" className="text-sm font-normal text-red-700" data-cy={dataCy ? `${dataCy}-error` : undefined} id={errorId} role="alert">{error}</p> : null}

      {isOpen ? (
        <div
          aria-label={`Календарь: ${label}`}
          className="fixed z-30 w-[min(20rem,calc(100vw-2rem))] overflow-y-auto border border-border bg-surface p-4 text-primary shadow-surface"
          data-placement={calendarPosition?.placement}
          id={dialogId}
          ref={calendarRef}
          role="dialog"
          style={calendarPosition ? { left: calendarPosition.left, maxHeight: calendarPosition.maxHeight, top: calendarPosition.top } : { visibility: "hidden" }}
        >
          <div className="flex items-center justify-between gap-3 border-b border-border pb-3">
            <button aria-label="Предыдущий месяц" className="inline-flex size-11 items-center justify-center rounded-full text-secondary transition-colors hover:bg-page hover:text-primary" onClick={() => setVisibleMonth(addMonths(visibleMonth, -1))} type="button"><ChevronIcon direction="left" /></button>
            <p aria-live="polite" className="font-display text-xl capitalize">{months[visibleMonth.getMonth()]} {visibleMonth.getFullYear()}</p>
            <button aria-label="Следующий месяц" className="inline-flex size-11 items-center justify-center rounded-full text-secondary transition-colors hover:bg-page hover:text-primary" onClick={() => setVisibleMonth(addMonths(visibleMonth, 1))} type="button"><ChevronIcon direction="right" /></button>
          </div>
          <div aria-hidden="true" className="mt-3 grid grid-cols-7 text-center text-[11px] font-semibold uppercase tracking-[0.08em] text-secondary">{weekdays.map((day) => <span className="py-2" key={day}>{day}</span>)}</div>
          <div className="grid grid-cols-7 gap-1" role="grid">
            {days.map((date) => {
              const dateValue = toDateValue(date);
              const isSelected = value === dateValue;
              const isOutside = date.getMonth() !== visibleMonth.getMonth();
              const isDisabled = Boolean(minDate && date < minDate);
              return (
                <button
                  aria-label={formatLongDate(date)}
                  aria-pressed={isSelected}
                  className={cn("inline-flex aspect-square min-h-9 items-center justify-center rounded-full text-sm transition-colors hover:bg-page focus-visible:outline-offset-0 disabled:cursor-not-allowed disabled:opacity-25", isOutside && "text-secondary", isSelected && "bg-action text-inverse-text hover:bg-action-hover")}
                  disabled={isDisabled}
                  key={dateValue}
                  onClick={() => chooseDate(date)}
                  type="button"
                >
                  {date.getDate()}
                </button>
              );
            })}
          </div>
          {value ? <button className="mt-3 min-h-11 w-full border-t border-border pt-3 text-sm text-secondary transition-colors hover:text-primary" onClick={() => { onValueChange(""); setIsOpen(false); triggerRef.current?.focus(); }} type="button">Очистить дату</button> : null}
        </div>
      ) : null}
    </div>
  );
}

function CalendarIcon() {
  return <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20"><rect height="13" rx="1.5" stroke="currentColor" strokeWidth="1.4" width="14" x="3" y="4.5"/><path d="M3 8.5h14M7 2.5v4M13 2.5v4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.4"/></svg>;
}

function ChevronIcon({ direction }: Readonly<{ direction: "left" | "right" }>) {
  return <svg aria-hidden="true" className="size-4" fill="none" viewBox="0 0 16 16"><path d={direction === "left" ? "m10 3-5 5 5 5" : "m6 3 5 5-5 5"} stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5"/></svg>;
}

function parseDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number) {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function calendarDays(month: Date) {
  const mondayOffset = (month.getDay() + 6) % 7;
  const first = new Date(month.getFullYear(), month.getMonth(), 1 - mondayOffset);
  return Array.from({ length: 42 }, (_, index) => new Date(first.getFullYear(), first.getMonth(), first.getDate() + index));
}

function toDateValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", year: "numeric" }).format(date);
}

function formatLongDate(date: Date) {
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", weekday: "long", year: "numeric" }).format(date);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}
