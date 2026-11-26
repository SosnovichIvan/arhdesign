"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button, Dialog, Input, Textarea } from "@/shared/ui";
import { consent, privacy } from "@/shared/config";

const emailPattern = /^[^@<>\s]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i;
const phonePattern = /^\+?[0-9() .-]+$/;

function isContact(value: string) {
  if (!value) return true;
  const digits = value.replace(/\D/g, "").length;
  return emailPattern.test(value) || (phonePattern.test(value) && digits >= 10 && digits <= 15);
}

const contactFormSchema = z.object({
  name: z.string().trim().min(2, "Укажите имя").max(100, "Имя должно быть не длиннее 100 символов"),
  contact: z.string().trim().min(1, "Укажите телефон или почту").max(254, "Контакт слишком длинный").refine(isContact, "Введите номер телефона или адрес почты"),
  projectType: z.string().trim().min(2, "Укажите тип проекта").max(120, "Тип проекта слишком длинный"),
  projectDetails: z.string().trim().max(5000, "Описание проекта слишком длинное"),
  consent: z.boolean().refine((value) => value, "Требуется согласие на обработку данных"),
  website: z.string(),
});

type ContactFormData = z.infer<typeof contactFormSchema>;
type Status = "idle" | "loading" | "success" | "failure" | "cooldown";
type ContactFormProps = {
  triggerClassName?: string;
  variant?: "primary" | "secondary";
  triggerLabel?: string;
  isOpen?: boolean;
  onClose?: () => void;
  showTrigger?: boolean;
};

const ContactDialogContext = createContext<(() => void) | null>(null);

export function ContactForm({ triggerClassName, triggerLabel = "Обсудить проект", isOpen: controlledIsOpen, onClose, showTrigger = true }: ContactFormProps) {
  const [uncontrolledIsOpen, setUncontrolledIsOpen] = useState(false);
  const [status, setStatus] = useState<Status>("idle");
  const [retryAfter, setRetryAfter] = useState(0);
  const isControlled = controlledIsOpen !== undefined;
  const isOpen = controlledIsOpen ?? uncontrolledIsOpen;
  const form = useForm<ContactFormData>({
    defaultValues: { name: "", contact: "", projectType: "", projectDetails: "", consent: false, website: "" },
    resolver: zodResolver(contactFormSchema),
  });

  useEffect(() => {
    if (status !== "cooldown" || retryAfter <= 0) return;
    const timeoutId = window.setTimeout(() => {
      if (retryAfter <= 1) {
        setRetryAfter(0);
        setStatus("idle");
        return;
      }
      setRetryAfter((value) => value - 1);
    }, 1000);
    return () => window.clearTimeout(timeoutId);
  }, [retryAfter, status]);

  function closeDialog() {
    if (!isControlled) setUncontrolledIsOpen(false);
    setStatus("idle");
    setRetryAfter(0);
    onClose?.();
  }

  async function submit(data: ContactFormData) {
    setStatus("loading");
    try {
      const response = await fetch("/api/v1/contact-submissions", { body: JSON.stringify(data), headers: { "Content-Type": "application/json" }, method: "POST" });
      if (response.status === 429) {
        setRetryAfter(Number(response.headers.get("Retry-After") ?? 3600));
        setStatus("cooldown");
        return;
      }
      if (!response.ok) {
        setStatus("failure");
        return;
      }
      form.reset();
      setStatus("success");
    } catch {
      setStatus("failure");
    }
  }

  const errors = form.formState.errors;
  const statusMessage = status === "cooldown" ? `Повторная отправка будет доступна через ${formatRemainingTime(retryAfter)}.` : status === "success" ? "Спасибо, заявка принята. Мы свяжемся с вами в ближайшее время." : null;

  return <>
    {showTrigger ? <Button className={triggerClassName} onClick={() => setUncontrolledIsOpen(true)}>{triggerLabel}</Button> : null}
    <Dialog isOpen={isOpen} label="Обсудить проект" onClose={closeDialog}>
      <div className="min-w-0">
        <h2 className="font-display wrap-break-word text-3xl leading-[1.08] tablet:max-w-md tablet:text-5xl">Расскажите о будущем проекте</h2>
        <p className="mt-4 max-w-xl leading-7 text-secondary">Оставьте контакты и несколько деталей — отвечу с ориентиром по формату работы и следующему шагу.</p>
        {statusMessage ? <p className="mt-8 border-l-2 border-action pl-4 text-primary" role="status">{statusMessage}</p> : <form className="mt-6 space-y-4" noValidate onSubmit={form.handleSubmit(submit)}>
          <FieldError error={errors.name?.message} id="contact-name-error"><FieldLabel label="Имя" required><Input aria-describedby={errors.name ? "contact-name-error" : undefined} aria-invalid={Boolean(errors.name)} aria-required="true" autoComplete="name" data-dialog-initial-focus className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Как к вам обращаться" {...form.register("name")} /></FieldLabel></FieldError>
          <FieldError error={errors.contact?.message} id="contact-value-error"><FieldLabel label="Телефон или почта" required><Input aria-describedby={errors.contact ? "contact-value-error" : undefined} aria-invalid={Boolean(errors.contact)} aria-required="true" autoComplete="on" className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Удобный способ связи" {...form.register("contact")} /></FieldLabel></FieldError>
          <FieldError error={errors.projectType?.message} id="contact-type-error"><FieldLabel label="Тип проекта" required><Input aria-describedby={errors.projectType ? "contact-type-error" : undefined} aria-invalid={Boolean(errors.projectType)} className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" aria-required="true" placeholder="Например, квартира" {...form.register("projectType")} /></FieldLabel></FieldError>
          <FieldError error={errors.projectDetails?.message} id="contact-details-error"><FieldLabel label="О проекте"><Textarea aria-describedby={errors.projectDetails ? "contact-details-error" : undefined} aria-invalid={Boolean(errors.projectDetails)} className="mt-2 min-h-[76px] border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Площадь, сроки, пожелания" {...form.register("projectDetails")} /></FieldLabel></FieldError>
          <div className="flex items-start gap-3 text-xs leading-[18px] text-secondary"><input aria-describedby={errors.consent ? "personal-data-consent-details contact-consent-error" : "personal-data-consent-details"} aria-invalid={Boolean(errors.consent)} aria-required="true" className="mt-1 size-4 shrink-0 accent-action" id="personal-data-consent" type="checkbox" {...form.register("consent")} /><div id="personal-data-consent-details"><label className="cursor-pointer" htmlFor="personal-data-consent">Я даю {consent.operatorName} согласие на обработку персональных данных для рассмотрения обращения и связи со мной.</label> <a className="text-primary underline decoration-action underline-offset-4 transition-colors hover:text-action focus-visible:rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-action" href={consent.documentHref} rel="noopener noreferrer" target="_blank">Полный текст согласия (PDF)</a>. <Link className="text-primary underline decoration-action underline-offset-4 transition-colors hover:text-action focus-visible:rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-action" href={privacy.pageHref}>Политика обработки персональных данных</Link>.</div></div>
          {form.formState.errors.consent?.message ? <p className="text-sm text-danger" id="contact-consent-error" role="alert">{errors.consent?.message}</p> : null}
          <Input aria-hidden="true" autoComplete="off" className="hidden" tabIndex={-1} {...form.register("website")} />
          {status === "failure" ? <p className="text-sm text-danger" role="alert">Не удалось отправить форму. Попробуйте ещё раз.</p> : null}
          <Button className="px-8 text-xs tracking-[0.1em] uppercase" disabled={status === "loading"} type="submit">{status === "loading" ? "Отправляем…" : "Отправить запрос"}</Button>
        </form>}
      </div>
    </Dialog>
  </>;
}

export function ContactFormProvider({ children }: { children: ReactNode }) {
  const [isOpen, setIsOpen] = useState(false);

  return <ContactDialogContext.Provider value={() => setIsOpen(true)}>
    {children}
    <ContactForm isOpen={isOpen} onClose={() => setIsOpen(false)} showTrigger={false} />
  </ContactDialogContext.Provider>;
}

export function ContactFormTrigger({ triggerClassName, triggerLabel = "Обсудить проект", variant }: Pick<ContactFormProps, "triggerClassName" | "triggerLabel" | "variant">) {
  const openDialog = useContext(ContactDialogContext);

  if (!openDialog) throw new Error("ContactFormTrigger must be rendered inside ContactFormProvider");

  return <Button className={triggerClassName} onClick={openDialog} variant={variant}>{triggerLabel}</Button>;
}

function FieldError({ children, error, id }: { children: ReactNode; error?: string; id: string }) {
  return <div>{children}{error ? <p className="mt-2 text-sm text-danger" id={id} role="alert">{error}</p> : null}</div>;
}

function FieldLabel({ children, label, required = false }: { children: ReactNode; label: string; required?: boolean }) {
  return <label className="block text-xs font-semibold tracking-[0.08em] text-secondary uppercase"><span>{label}{required ? <span aria-hidden="true" className="ml-1 text-action">*</span> : null}</span>{children}</label>;
}

function formatRemainingTime(seconds: number) {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
