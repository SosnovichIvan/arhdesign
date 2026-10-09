"use client";

import { forwardRef, useId, useState, type InputHTMLAttributes } from "react";

import { cn } from "@/shared/lib";
import { Input } from "@/shared/ui/input";

type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  dataCy?: string;
  description?: string;
  error?: string;
  label: string;
  rootClassName?: string;
};

export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(function PasswordInput(
  { "aria-describedby": ariaDescribedBy, className, dataCy, description, disabled, error, id, label, required, rootClassName, ...props },
  ref,
) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const detailsId = `${inputId}-details`;
  const [isVisible, setIsVisible] = useState(false);
  const actionLabel = isVisible ? "Скрыть пароль" : "Показать пароль";

  return (
    <div className={cn("grid gap-2 text-sm font-medium", rootClassName)}>
      <label htmlFor={inputId}>{label}{required ? " *" : null}</label>
      <div className="relative">
        <Input
          {...props}
          aria-describedby={description || error ? [ariaDescribedBy, detailsId].filter(Boolean).join(" ") : ariaDescribedBy}
          aria-invalid={error ? true : undefined}
          className={cn("pr-12", className)}
          data-cy={dataCy}
          disabled={disabled}
          id={inputId}
          ref={ref}
          required={required}
          type={isVisible ? "text" : "password"}
        />
        <button
          aria-controls={inputId}
          aria-label={actionLabel}
          aria-pressed={isVisible}
          className="absolute inset-y-0 right-0 inline-flex w-11 items-center justify-center text-secondary transition-colors hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-field-focus disabled:pointer-events-none disabled:opacity-50"
          data-cy={dataCy ? `${dataCy}-visibility` : undefined}
          disabled={disabled}
          onClick={() => setIsVisible((current) => !current)}
          type="button"
        >
          {isVisible ? <EyeOffIcon /> : <EyeIcon />}
        </button>
      </div>
      {error ? <p className="text-xs font-normal leading-5 text-red-700" id={detailsId}>{error}</p> : description ? <p className="text-xs font-normal leading-5 text-secondary" id={detailsId}>{description}</p> : null}
    </div>
  );
});

function EyeIcon() {
  return (
    <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20">
      <path d="M2.5 10s2.7-5 7.5-5 7.5 5 7.5 5-2.7 5-7.5 5-7.5-5-7.5-5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.4" />
      <circle cx="10" cy="10" r="2.2" stroke="currentColor" strokeWidth="1.4" />
    </svg>
  );
}

function EyeOffIcon() {
  return (
    <svg aria-hidden="true" className="size-5" fill="none" viewBox="0 0 20 20">
      <path d="M3 3 17 17M8.6 5.15A7.4 7.4 0 0 1 10 5c4.8 0 7.5 5 7.5 5a12.3 12.3 0 0 1-2.05 2.65M6.4 6.35C3.9 7.75 2.5 10 2.5 10s2.7 5 7.5 5c.7 0 1.35-.1 1.95-.28M8.45 8.45a2.2 2.2 0 0 0 3.1 3.1" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.4" />
    </svg>
  );
}
