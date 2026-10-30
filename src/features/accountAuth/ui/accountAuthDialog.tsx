"use client";

import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useState, type FormEvent, type ReactNode } from "react";

import { RegistrationForm, VerificationPrompt } from "@/features/accountRegistration";
import { readApiErrorDetails } from "@/shared/lib";
import { Button, Dialog, Input, PasswordInput } from "@/shared/ui";

type AuthMode = "login" | "register" | "forgot" | "reset" | "verify";
type AccountAuthContextValue = {
  openAuth: (mode?: AuthMode) => void;
};

const AccountAuthContext = createContext<AccountAuthContextValue | null>(null);

export function AccountAuthProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [mode, setMode] = useState<AuthMode | null>(null);

  useEffect(() => {
    const timeoutId = window.setTimeout(() => {
      const requestedMode = new URLSearchParams(window.location.search).get("auth");
      if (requestedMode === "login" || requestedMode === "register" || requestedMode === "forgot" || requestedMode === "reset" || requestedMode === "verify") setMode(requestedMode);
    }, 0);
    return () => window.clearTimeout(timeoutId);
  }, []);

  const syncUrl = useCallback((nextMode: AuthMode | null) => {
    const url = new URL(window.location.href);
    if (nextMode) url.searchParams.set("auth", nextMode);
    else url.searchParams.delete("auth");
    if (nextMode !== "reset" && nextMode !== "verify" && url.hash.startsWith("#token=")) url.hash = "";
    window.history.replaceState(window.history.state, "", url);
  }, []);

  const openAuth = useCallback((nextMode: AuthMode = "login") => {
    setMode(nextMode);
    syncUrl(nextMode);
  }, [syncUrl]);

  const closeAuth = useCallback(() => {
    setMode(null);
    syncUrl(null);
  }, [syncUrl]);

  return (
    <AccountAuthContext.Provider value={{ openAuth }}>
      {children}
      <Dialog className={mode === "register" ? "max-w-[820px]" : "max-w-[560px]"} isOpen={mode !== null} key={mode ?? "closed"} label={authDialogLabel(mode)} onClose={closeAuth}>
        {mode === "login" ? <LoginForm onClose={closeAuth} onForgot={() => openAuth("forgot")} onSwitchToRegister={() => openAuth("register")} /> : null}
        {mode === "register" ? <RegistrationForm onSwitchToLogin={() => openAuth("login")} /> : null}
        {mode === "forgot" ? <ForgotPasswordForm onBack={() => openAuth("login")} /> : null}
        {mode === "reset" ? <ResetPasswordForm onComplete={() => openAuth("login")} /> : null}
        {mode === "verify" ? <VerifyEmailForm onComplete={() => openAuth("login")} /> : null}
      </Dialog>
    </AccountAuthContext.Provider>
  );
}

export function useAccountAuth() {
  const context = useContext(AccountAuthContext);
  if (!context) throw new Error("useAccountAuth must be used inside AccountAuthProvider");
  return context;
}

function LoginForm({ onClose, onForgot, onSwitchToRegister }: Readonly<{ onClose: () => void; onForgot: () => void; onSwitchToRegister: () => void }>) {
  const router = useRouter();
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(false);
  const [isPending, setIsPending] = useState(false);
  const [requiresVerification, setRequiresVerification] = useState(false);
  const [error, setError] = useState("");

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identifier, password, rememberMe }),
      });
      if (!response.ok) {
        const details = await readApiErrorDetails(response, "Не удалось выполнить вход");
        if (details.code === "account_unverified") {
          setRequiresVerification(true);
          return;
        }
        setError(details.message);
        return;
      }
      window.dispatchEvent(new CustomEvent("arhdesign:session-changed", { detail: { authenticated: true } }));
      onClose();
      router.replace("/account");
    } catch {
      setError("Сервис авторизации временно недоступен");
    } finally {
      setIsPending(false);
    }
  }

  if (requiresVerification) return <VerificationPrompt identifier={identifier.trim()} onBack={() => setRequiresVerification(false)} />;

  return (
    <form className="pr-10 tablet:pr-12" noValidate onSubmit={login}>
      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Личный кабинет</p>
      <h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Войти</h2>
      <p className="mt-4 leading-7 text-secondary">Используйте логин или электронную почту, указанные при регистрации.</p>
      <div className="mt-8 space-y-6">
        <label className="grid gap-2 text-sm font-medium">Логин или почта *<Input autoComplete="username" data-cy="login-identifier" data-dialog-initial-focus onChange={(event) => setIdentifier(event.target.value)} required value={identifier} /></label>
        <PasswordInput autoComplete="current-password" dataCy="login-password" label="Пароль" onChange={(event) => setPassword(event.target.value)} required value={password} />
        <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-secondary"><input checked={rememberMe} className="size-4 accent-action" onChange={(event) => setRememberMe(event.target.checked)} type="checkbox" />Запомнить меня</label>
      </div>
      {error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}
      <Button className="mt-7 w-full" data-cy="login-submit" disabled={isPending} type="submit">{isPending ? "Входим…" : "Войти"}</Button>
      <button className="mt-4 min-h-11 text-sm text-secondary underline decoration-action underline-offset-4 hover:text-primary" onClick={onForgot} type="button">Забыли пароль?</button>
      <p className="mt-6 text-sm text-secondary">Нет профиля? <button className="font-medium text-primary underline decoration-action underline-offset-4 hover:text-action" data-cy="auth-switch-register" onClick={onSwitchToRegister} type="button">Зарегистрироваться</button></p>
    </form>
  );
}

function ForgotPasswordForm({ onBack }: Readonly<{ onBack: () => void }>) {
  const [identifier, setIdentifier] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [acceptedChannel, setAcceptedChannel] = useState<"email" | "telegram" | null>(null);
  const [error, setError] = useState("");

  async function requestRecovery(channel: "email" | "telegram") {
    setError("");
    if (identifier.trim().length < 3) { setError("Укажите логин или электронную почту"); return; }
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/forgot-password", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ identifier: identifier.trim(), channel }) });
      if (!response.ok) { setError((await readApiErrorDetails(response, "Не удалось выполнить запрос")).message); return; }
      setAcceptedChannel(channel);
    } catch { setError("Сервис восстановления временно недоступен"); }
    finally { setIsPending(false); }
  }

  if (acceptedChannel) return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Восстановление доступа</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Проверьте выбранный канал</h2><p className="mt-5 leading-7 text-secondary">Если профиль существует и {acceptedChannel === "email" ? "электронная почта была подтверждена" : "Telegram был подключён заранее"}, мы отправили одноразовую ссылку для восстановления доступа. Ссылка действует 30 минут.</p><div className="mt-7 flex flex-wrap gap-3"><Button onClick={() => setAcceptedChannel(null)} type="button">Выбрать другой способ</Button><Button onClick={onBack} type="button" variant="secondary">Вернуться ко входу</Button></div></section>;

  return <form className="pr-10 tablet:pr-12" onSubmit={(event) => { event.preventDefault(); void requestRecovery("email"); }}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Восстановление доступа</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Восстановить пароль</h2><p className="mt-4 leading-7 text-secondary">Укажите логин или почту и выберите способ восстановления. Ответ не раскрывает, существует ли профиль.</p><label className="mt-8 grid gap-2 text-sm font-medium">Логин или почта *<Input autoComplete="username" data-dialog-initial-focus onChange={(event) => setIdentifier(event.target.value)} required value={identifier} /></label>{error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}<div className="mt-7 grid gap-3 tablet:grid-cols-2"><Button disabled={isPending} type="submit">Получить по email</Button><Button disabled={isPending} onClick={() => void requestRecovery("telegram")} type="button" variant="secondary">Получить в Telegram</Button></div><button className="mt-5 min-h-11 text-sm text-secondary underline decoration-action underline-offset-4 hover:text-primary" onClick={onBack} type="button">Вернуться ко входу</button></form>;
}

function VerifyEmailForm({ onComplete }: Readonly<{ onComplete: () => void }>) {
  const [token] = useState(() => typeof window === "undefined" ? "" : new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "");
  const [status, setStatus] = useState<"pending" | "success" | "expired" | "error">("pending");
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function verify() {
      if (token.length < 32) { setStatus("expired"); return; }
      try {
        const response = await fetch("/api/v1/auth/verify-email", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token }), signal: controller.signal });
        if (!response.ok) {
          const details = await readApiErrorDetails(response, "Не удалось подтвердить электронную почту");
          if (details.code === "invalid_or_expired_token") setStatus("expired");
          else { setError(details.message); setStatus("error"); }
          return;
        }
        setStatus("success");
      } catch (requestError) {
        if ((requestError as Error).name !== "AbortError") { setError("Сервис подтверждения временно недоступен"); setStatus("error"); }
      }
    }
    void verify();
    return () => controller.abort();
  }, [token]);

  if (status === "pending") return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Подтверждение профиля</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Проверяем ссылку…</h2><p className="mt-5 leading-7 text-secondary" role="status">Это займёт несколько секунд.</p></section>;
  if (status === "success") return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Подтверждение профиля</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Почта подтверждена</h2><p className="mt-5 leading-7 text-secondary">Профиль активирован. Теперь вы можете войти в личный кабинет.</p><Button className="mt-7" onClick={onComplete} type="button">Войти</Button></section>;
  if (status === "expired") return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Подтверждение профиля</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Ссылка устарела</h2><p className="mt-5 leading-7 text-secondary">Ссылка недействительна, уже использована или истекла. Войдите с логином и паролем — мы предложим запросить новое письмо или подтвердить профиль через Telegram.</p><Button className="mt-7" onClick={onComplete} type="button">Вернуться ко входу</Button></section>;
  return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Подтверждение профиля</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Не удалось проверить ссылку</h2><p className="mt-5 leading-7 text-secondary" role="alert">{error}</p><Button className="mt-7" onClick={onComplete} type="button" variant="secondary">Вернуться ко входу</Button></section>;
}

function ResetPasswordForm({ onComplete }: Readonly<{ onComplete: () => void }>) {
  const [token] = useState(() => typeof window === "undefined" ? "" : new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [error, setError] = useState("");

  async function reset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError("");
    if (token.length < 32) { setError("Ссылка недействительна или устарела"); return; }
    if (Array.from(password).length < 15 || password !== confirmation) { setError(password !== confirmation ? "Пароли не совпадают" : "Пароль должен содержать не менее 15 символов"); return; }
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/reset-password", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token, password, passwordConfirmation: confirmation }) });
      if (!response.ok) { setError((await readApiErrorDetails(response, "Не удалось изменить пароль")).message); return; }
      setCompleted(true);
    } catch { setError("Сервис восстановления временно недоступен"); }
    finally { setIsPending(false); }
  }

  if (completed) return <section aria-live="polite" className="pr-10 tablet:pr-12"><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Восстановление доступа</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Пароль изменён</h2><p className="mt-5 leading-7 text-secondary">Все ранее открытые сеансы завершены. Войдите с новым паролем.</p><Button className="mt-7" onClick={onComplete} type="button">Войти</Button></section>;
  return <form className="pr-10 tablet:pr-12" onSubmit={reset}><p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Восстановление доступа</p><h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Новый пароль</h2><div className="mt-8 space-y-6"><PasswordInput autoComplete="new-password" dataCy="reset-password" data-dialog-initial-focus label="Новый пароль" minLength={15} onChange={(event) => setPassword(event.target.value)} required value={password} /><PasswordInput autoComplete="new-password" dataCy="reset-password-confirmation" label="Подтверждение пароля" minLength={15} onChange={(event) => setConfirmation(event.target.value)} required value={confirmation} /></div>{error ? <p className="mt-4 text-sm text-red-700" role="alert">{error}</p> : null}<Button className="mt-7 w-full" disabled={isPending} type="submit">{isPending ? "Сохраняем…" : "Сохранить пароль"}</Button></form>;
}

function authDialogLabel(mode: AuthMode | null) {
  if (mode === "register") return "Регистрация";
  if (mode === "forgot" || mode === "reset") return "Восстановление доступа";
  if (mode === "verify") return "Подтверждение профиля";
  return "Авторизация";
}
