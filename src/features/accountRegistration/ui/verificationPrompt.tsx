"use client";

import { useEffect, useState } from "react";

import { readApiError } from "@/shared/lib";
import { Button } from "@/shared/ui";

type VerificationPhase = "channel" | "email-sent" | "telegram-handoff" | "telegram-error";
const resendCooldownSeconds = 60;

export function VerificationPrompt({ email, identifier, onBack }: Readonly<{ email?: string; identifier: string; onBack?: () => void }>) {
  const [phase, setPhase] = useState<VerificationPhase>("channel");
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState("");
  const [resendRemaining, setResendRemaining] = useState(0);
  const [telegramUrl, setTelegramUrl] = useState("");

  useEffect(() => {
    if (resendRemaining <= 0) return;
    const interval = window.setInterval(() => setResendRemaining((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearInterval(interval);
  }, [resendRemaining]);

  async function selectChannel(channel: "email" | "telegram") {
    setError("");
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/verification-channel", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identifier, channel }),
      });
      if (!response.ok) {
        setError(await readApiError(response, "Не удалось выполнить запрос"));
        return;
      }
      const handoff = (await response.json()) as { telegramBotUrl?: string | null };
      if (channel === "email") {
        setPhase("email-sent");
        setResendRemaining(resendCooldownSeconds);
        return;
      }
      if (!handoff.telegramBotUrl) {
        setPhase("telegram-error");
        return;
      }
      setTelegramUrl(handoff.telegramBotUrl);
      setPhase("telegram-handoff");
      window.location.assign(handoff.telegramBotUrl);
    } catch {
      setPhase(channel === "telegram" ? "telegram-error" : "channel");
      setError("Сервис подтверждения временно недоступен");
    } finally {
      setIsPending(false);
    }
  }

  async function resendEmail() {
    setError("");
    setIsPending(true);
    try {
      const response = await fetch("/api/v1/auth/resend-verification", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identifier }),
      });
      if (!response.ok) {
        setError(await readApiError(response, "Не удалось отправить письмо повторно"));
        return;
      }
      setResendRemaining(resendCooldownSeconds);
    } catch {
      setError("Сервис подтверждения временно недоступен");
    } finally {
      setIsPending(false);
    }
  }

  return (
    <section aria-live="polite" className="pr-10 tablet:pr-12">
      <div className="mb-7 border-l-2 border-action bg-surface px-5 py-4" data-cy="verification-required" role="status">
        <p className="font-medium text-primary">Профиль ещё не подтверждён</p>
        <p className="mt-2 text-sm leading-6 text-secondary">Подтвердите данные по электронной почте или через Telegram, чтобы войти в личный кабинет.</p>
      </div>
      {phase === "channel" ? (
        <>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-action">Подтверждение профиля</p>
          <h2 className="mt-4 font-display text-4xl leading-none tablet:text-5xl">Как подтвердить профиль?</h2>
          <p className="mt-5 max-w-xl leading-7 text-secondary">Выберите удобный способ. При выборе Telegram сайт откроет приложение и личный чат с ботом. Логин и пароль вводятся только внутри Telegram — сайт их повторно не запрашивает.</p>
          <div className="mt-8 grid gap-3 tablet:grid-cols-2">
            <Button data-cy="verification-email" disabled={isPending} onClick={() => void selectChannel("email")} type="button">Получить письмо</Button>
            <Button data-cy="verification-telegram" disabled={isPending} onClick={() => void selectChannel("telegram")} type="button" variant="secondary">Открыть Telegram</Button>
          </div>
          {onBack ? <button className="mt-6 min-h-11 text-sm text-secondary underline decoration-action underline-offset-4 hover:text-primary" onClick={onBack} type="button">Вернуться ко входу</button> : null}
        </>
      ) : null}
      {phase === "email-sent" ? <><h2 className="font-display text-4xl">Проверьте почту</h2><p className="mt-4 leading-7 text-secondary">Мы отправили ссылку подтверждения{email ? <> на <strong className="font-medium text-primary">{maskEmail(email)}</strong></> : " на указанный при регистрации адрес"}. Перейдите по ней в течение 24 часов. Если письма нет, проверьте папку «Спам» или выберите Telegram.</p><div className="mt-6 flex flex-wrap gap-3"><Button disabled={isPending || resendRemaining > 0} onClick={() => void resendEmail()} type="button">{resendRemaining > 0 ? `Отправить ещё раз (${resendRemaining} сек.)` : "Отправить ещё раз"}</Button><Button disabled={isPending} onClick={() => void selectChannel("telegram")} type="button" variant="secondary">Получить в Telegram</Button></div></> : null}
      {phase === "telegram-handoff" ? <><h2 className="font-display text-4xl">Открываем Telegram</h2><p className="mt-4 leading-7 text-secondary">Продолжите подтверждение в личном чате с ботом arhDesign. Логин и пароль вводятся только в Telegram — на сайте этих полей нет.</p><div className="mt-6 flex flex-wrap gap-3"><Button onClick={() => window.location.assign(telegramUrl)} type="button">Открыть приложение Telegram</Button><Button onClick={() => void selectChannel("email")} type="button" variant="secondary">Выбрать почту</Button></div></> : null}
      {phase === "telegram-error" ? <><h2 className="font-display text-4xl">Не удалось открыть Telegram</h2><p className="mt-4 leading-7 text-secondary">Повторите запуск или вернитесь к подтверждению по электронной почте. Логин и пароль вводятся только в личном чате с ботом.</p><div className="mt-6 flex flex-wrap gap-3"><Button disabled={isPending} onClick={() => void selectChannel("telegram")} type="button">Открыть приложение Telegram</Button><Button disabled={isPending} onClick={() => void selectChannel("email")} type="button" variant="secondary">Выбрать почту</Button></div></> : null}
      {error ? <p className="mt-5 text-sm text-red-700" role="alert">{error}</p> : null}
    </section>
  );
}

function maskEmail(value: string) {
  const [local, domain] = value.trim().split("@");
  if (!local || !domain) return value;
  return `${local.slice(0, 1)}${"•".repeat(Math.min(Math.max(local.length - 1, 3), 8))}@${domain}`;
}
