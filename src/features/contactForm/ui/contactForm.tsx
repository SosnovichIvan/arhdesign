"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button, Dialog, Input, Textarea } from "@/shared/ui";

const contactFormSchema = z.object({
  name: z.string().trim().min(2, "Укажите имя"),
  contact: z.string().trim().min(5, "Укажите телефон или почту"),
  projectType: z.string().trim().min(1, "Укажите тип проекта"),
  projectDetails: z.string().trim().max(5000, "Описание проекта слишком длинное"),
  consent: z.boolean().refine((value) => value, "Требуется согласие на обработку данных"),
  website: z.string(),
});

type ContactFormData = z.infer<typeof contactFormSchema>;
type Status = "idle" | "loading" | "success" | "failure" | "cooldown";
type ContactFormProps = {
  triggerClassName?: string;
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

  const statusMessage = status === "cooldown" ? `Повторная отправка будет доступна через ${formatRemainingTime(retryAfter)}.` : status === "success" ? "Спасибо, заявка принята. Мы свяжемся с вами в ближайшее время." : null;

  return <>
    {showTrigger ? <Button className={triggerClassName} onClick={() => setUncontrolledIsOpen(true)}>{triggerLabel}</Button> : null}
    <Dialog isOpen={isOpen} label="Обсудить проект" onClose={closeDialog}>
      <div className="pr-10 tablet:pr-12">
        <h2 className="font-display text-4xl leading-[1.08] tablet:max-w-md tablet:text-5xl">Расскажите о будущем проекте</h2>
        <p className="mt-4 max-w-xl leading-7 text-secondary">Оставьте контакты и несколько деталей — отвечу с ориентиром по формату работы и следующему шагу.</p>
        {statusMessage ? <p className="mt-8 border-l-2 border-action pl-4 text-primary" role="status">{statusMessage}</p> : <form className="mt-6 space-y-4" noValidate onSubmit={form.handleSubmit(submit)}>
          <FieldError error={form.formState.errors.name?.message}><FieldLabel label="Имя" required><Input aria-invalid={Boolean(form.formState.errors.name)} autoComplete="name" className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Как к вам обращаться" {...form.register("name")} /></FieldLabel></FieldError>
          <FieldError error={form.formState.errors.contact?.message}><FieldLabel label="Телефон или почта" required><Input aria-invalid={Boolean(form.formState.errors.contact)} autoComplete="email" className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Удобный способ связи" {...form.register("contact")} /></FieldLabel></FieldError>
          <FieldError error={form.formState.errors.projectType?.message}><FieldLabel label="Тип проекта" required><Input aria-invalid={Boolean(form.formState.errors.projectType)} className="mt-2 border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Дом, квартира, коммерческое пространство" {...form.register("projectType")} /></FieldLabel></FieldError>
          <FieldError error={form.formState.errors.projectDetails?.message}><FieldLabel label="О проекте"><Textarea aria-invalid={Boolean(form.formState.errors.projectDetails)} className="mt-2 min-h-[76px] border-x-0 border-t-0 border-b border-border bg-transparent px-0 py-3 text-sm focus:border-field-focus focus:outline-none focus:ring-0 focus-visible:outline-none" placeholder="Площадь, сроки, пожелания" {...form.register("projectDetails")} /></FieldLabel></FieldError>
          <label className="flex cursor-pointer items-start gap-3 text-xs leading-[18px] text-secondary"><input className="mt-1 size-4 accent-action" type="checkbox" {...form.register("consent")} /><span>Отправляя форму, вы соглашаетесь на обработку данных для ответа на запрос.</span></label>
          {form.formState.errors.consent?.message ? <p className="text-sm text-red-700" role="alert">{form.formState.errors.consent.message}</p> : null}
          <Input aria-hidden="true" autoComplete="off" className="hidden" tabIndex={-1} {...form.register("website")} />
          {status === "failure" ? <p className="text-sm text-red-700" role="alert">Не удалось отправить форму. Попробуйте ещё раз.</p> : null}
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

export function ContactFormTrigger({ triggerClassName, triggerLabel = "Обсудить проект" }: Pick<ContactFormProps, "triggerClassName" | "triggerLabel">) {
  const openDialog = useContext(ContactDialogContext);

  if (!openDialog) throw new Error("ContactFormTrigger must be rendered inside ContactFormProvider");

  return <Button className={triggerClassName} onClick={openDialog}>{triggerLabel}</Button>;
}

function FieldError({ children, error }: { children: ReactNode; error?: string }) {
  return <div>{children}{error ? <p className="mt-2 text-sm text-red-700" role="alert">{error}</p> : null}</div>;
}

function FieldLabel({ children, label, required = false }: { children: ReactNode; label: string; required?: boolean }) {
  return <label className="block text-xs font-semibold tracking-[0.08em] text-secondary uppercase"><span>{label}{required ? <span aria-hidden="true" className="ml-1 text-action">*</span> : null}</span>{children}</label>;
}

function formatRemainingTime(seconds: number) {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
