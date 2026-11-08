import { csrfToken, readApiError } from "@/shared/lib";

import type { AccountNotificationList } from "../model/types";

export async function loadAccountNotifications(signal?: AbortSignal): Promise<AccountNotificationList> {
  const response = await fetch("/api/v1/notifications?pageSize=20", { credentials: "same-origin", signal });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось загрузить уведомления"));
  return response.json() as Promise<AccountNotificationList>;
}

export async function markAccountNotificationRead(notificationId: string): Promise<void> {
  const response = await fetch(`/api/v1/notifications/${encodeURIComponent(notificationId)}/read`, { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось отметить уведомление прочитанным"));
}

export async function markAllAccountNotificationsRead(): Promise<void> {
  const response = await fetch("/api/v1/notifications/read-all", { credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }, method: "POST" });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось отметить уведомления прочитанными"));
}
