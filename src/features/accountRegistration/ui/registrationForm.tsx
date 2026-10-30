"use client";

import Link from "next/link";
import { type FormEvent, type ReactNode, useState } from "react";

import { readApiErrorDetails } from "@/shared/lib";
import { Button, Input, PasswordInput } from "@/shared/ui";

import { RoleSelect } from "./roleSelect";
import { VerificationPrompt } from "./verificationPrompt";

type RegistrationValues = {
  login: string;
  email: string;
  firstName: string;
  lastName: string;
  middleName: string;
  professionalRoleCode: string;
  password: string;
  passwordConfirmation: string;
};

type RegistrationFieldErrors = Partial<Record<keyof RegistrationValues, string>>;

const initialValues: RegistrationValues = {
  login: "",
  email: "",
  firstName: "",
  lastName: "",
  middleName: "",
  professionalRoleCode: "customer",
  password: "",
  passwordConfirmation: "",
};

export function RegistrationForm({ onSwitchToLogin }: Readonly<{ onSwitchToLogin?: () => void }>) {
  const [values, setValues] = useState(initialValues);
  const [isRegistered, setIsRegistered] = useState(false);
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<RegistrationFieldErrors>({});

  const update = (field: keyof RegistrationValues, value: string) => {
    setValues((current) => ({ ...current, [field]: value }));
    setFieldErrors((current) => ({ ...current, [field]: undefined }));
  };

  async function register(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const validationErrors = validateRegistration(values);
    setFieldErrors(validationErrors);
    if (Object.keys(validationErrors).length > 0) {
      setError("Исправьте ошибки в выделенных полях");
      return;
    }
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(values),
      });
      if (!response.ok) {
        const details = await readApiErrorDetails(response, "Не удалось выполнить запрос");
        const serverFields = toRegistrationFieldErrors(details.fields);
        if (Object.keys(serverFields).length > 0) {
          setFieldErrors(serverFields);
        } else if (response.status === 409) {
          setFieldErrors({ email: "Почта или логин уже используются", login: "Логин или почта уже используются" });
        }
        setError(details.message);
        return;
      }
      setIsRegistered(true);
    } catch {
      setError("Сервис регистрации временно недоступен");
    } finally {
      setIsPending(false);
    }
  }

  if (isRegistered) return <VerificationPrompt email={values.email.trim()} identifier={values.login.trim()} />;

  return (
    <form className="pr-10 tablet:pr-12" noValidate onSubmit={register}>
      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Личный кабинет</p>
      <h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Создать профиль</h2>
      <p className="mt-4 text-secondary">Поля со звёздочкой обязательны.</p>
      <div className="mt-8 grid gap-6 tablet:grid-cols-2">
        <Field error={fieldErrors.login} hint="3–64 латинских символа, цифры, точка, дефис или _." id="register-login" label="Логин *"><Input aria-describedby="register-login-details" aria-invalid={fieldErrors.login ? true : undefined} autoComplete="username" data-cy="register-login" data-dialog-initial-focus id="register-login" maxLength={64} minLength={3} onChange={(event) => update("login", event.target.value)} required value={values.login} /></Field>
        <Field error={fieldErrors.email} id="register-email" label="Электронная почта *"><Input aria-describedby={fieldErrors.email ? "register-email-details" : undefined} aria-invalid={fieldErrors.email ? true : undefined} autoComplete="email" data-cy="register-email" id="register-email" maxLength={254} onChange={(event) => update("email", event.target.value)} required type="email" value={values.email} /></Field>
        <Field error={fieldErrors.firstName} id="register-first-name" label="Имя *"><Input aria-describedby={fieldErrors.firstName ? "register-first-name-details" : undefined} aria-invalid={fieldErrors.firstName ? true : undefined} autoComplete="given-name" data-cy="register-first-name" id="register-first-name" maxLength={100} onChange={(event) => update("firstName", event.target.value)} required value={values.firstName} /></Field>
        <Field error={fieldErrors.lastName} id="register-last-name" label="Фамилия"><Input aria-describedby={fieldErrors.lastName ? "register-last-name-details" : undefined} aria-invalid={fieldErrors.lastName ? true : undefined} autoComplete="family-name" id="register-last-name" maxLength={100} onChange={(event) => update("lastName", event.target.value)} value={values.lastName} /></Field>
        <Field error={fieldErrors.middleName} id="register-middle-name" label="Отчество"><Input aria-describedby={fieldErrors.middleName ? "register-middle-name-details" : undefined} aria-invalid={fieldErrors.middleName ? true : undefined} autoComplete="additional-name" id="register-middle-name" maxLength={100} onChange={(event) => update("middleName", event.target.value)} value={values.middleName} /></Field>
        <RoleSelect onChange={(value) => update("professionalRoleCode", value)} value={values.professionalRoleCode} />
        <PasswordInput autoComplete="new-password" dataCy="register-password" description="От 15 до 128 символов." error={fieldErrors.password} id="register-password" label="Пароль" maxLength={128} minLength={15} onChange={(event) => update("password", event.target.value)} required value={values.password} />
        <PasswordInput autoComplete="new-password" dataCy="register-password-confirmation" error={fieldErrors.passwordConfirmation} id="register-password-confirmation" label="Подтверждение пароля" maxLength={128} minLength={15} onChange={(event) => update("passwordConfirmation", event.target.value)} required value={values.passwordConfirmation} />
      </div>
      <p className="mt-5 text-sm leading-6 text-secondary">Создавая профиль, вы соглашаетесь с <Link className="underline underline-offset-4" href="/privacy" target="_blank">политикой обработки персональных данных</Link>.</p>
      {error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}
      <Button className="mt-7 w-full tablet:w-auto" data-cy="register-submit" disabled={isPending} type="submit">{isPending ? "Создаём профиль…" : "Зарегистрироваться"}</Button>
      <p className="mt-6 text-sm text-secondary">Уже есть профиль? {onSwitchToLogin ? <button className="font-medium text-primary underline decoration-action underline-offset-4 hover:text-action" data-cy="auth-switch-login" onClick={onSwitchToLogin} type="button">Войти</button> : <Link className="font-medium text-primary underline underline-offset-4" href="/?auth=login">Войти</Link>}</p>
    </form>
  );
}

function Field({ children, error, hint, id, label }: Readonly<{ children: ReactNode; error?: string; hint?: string; id: string; label: string }>) {
  return <div className="grid gap-2 text-sm font-medium"><label htmlFor={id}>{label}</label>{children}{error ? <p className="text-xs font-normal leading-5 text-red-700" id={`${id}-details`}>{error}</p> : hint ? <p className="text-xs font-normal leading-5 text-secondary" id={`${id}-details`}>{hint}</p> : null}</div>;
}

function validateRegistration(values: RegistrationValues): RegistrationFieldErrors {
  const errors: RegistrationFieldErrors = {};
  const login = values.login.trim();
  const email = values.email.trim();
  const firstName = values.firstName.trim();
  if (!/^[A-Za-z0-9._-]{3,64}$/.test(login)) errors.login = "Используйте 3–64 латинских символа, цифры, точку, дефис или подчёркивание";
  if (email.length > 254 || !/^[^\s@]+@[^\s@]+$/.test(email)) errors.email = "Введите корректный адрес электронной почты";
  if (firstName.length < 1 || Array.from(firstName).length > 100) errors.firstName = "Укажите имя длиной до 100 символов";
  if (Array.from(values.lastName.trim()).length > 100) errors.lastName = "Фамилия не должна быть длиннее 100 символов";
  if (Array.from(values.middleName.trim()).length > 100) errors.middleName = "Отчество не должно быть длиннее 100 символов";
  const passwordLength = Array.from(values.password).length;
  if (passwordLength < 15 || passwordLength > 128 || /[\u0000-\u001F\u007F]/.test(values.password)) errors.password = "Пароль должен содержать от 15 до 128 символов";
  if (values.password !== values.passwordConfirmation) errors.passwordConfirmation = "Пароли не совпадают";
  else if (!values.passwordConfirmation) errors.passwordConfirmation = "Повторите пароль";
  return errors;
}

function toRegistrationFieldErrors(fields: Record<string, string>): RegistrationFieldErrors {
  const allowed = new Set<keyof RegistrationValues>(["login", "email", "firstName", "lastName", "middleName", "professionalRoleCode", "password", "passwordConfirmation"]);
  return Object.fromEntries(Object.entries(fields).filter(([field]) => allowed.has(field as keyof RegistrationValues))) as RegistrationFieldErrors;
}
