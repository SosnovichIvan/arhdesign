import { readApiError } from "@/shared/lib";

import type { AccountStatus, AdminUser, AdminUserPage } from "../model/types";

export class AdminUsersApiError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

export async function loadAdminUsers(filters: { cursor?: string; identifier?: string; status?: AccountStatus }, signal?: AbortSignal): Promise<AdminUserPage> {
  const query = new URLSearchParams({ pageSize: "25" });
  if (filters.cursor) query.set("cursor", filters.cursor);
  if (filters.identifier?.trim()) query.set("identifier", filters.identifier.trim());
  if (filters.status) query.set("status", filters.status);
  const response = await fetch(`/api/v1/admin/users?${query}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new AdminUsersApiError(await readApiError(response, "Не удалось загрузить пользователей"), response.status);
  return response.json() as Promise<AdminUserPage>;
}

export async function changeAdminUserStatus(user: AdminUser, action: "disable" | "restore"): Promise<AdminUser> {
  const response = await fetch(`/api/v1/admin/users/${encodeURIComponent(user.id)}/${action}`, {
    credentials: "same-origin",
    headers: { "If-Match": `"${user.version}"`, "X-CSRF-Token": csrfToken() },
    method: "POST",
  });
  if (!response.ok) throw new AdminUsersApiError(await readApiError(response, action === "disable" ? "Не удалось отключить пользователя" : "Не удалось восстановить пользователя"), response.status);
  return response.json() as Promise<AdminUser>;
}

function csrfToken() {
  const cookie = document.cookie.split("; ").find((value) => value.startsWith("arhdesign_csrf=") || value.startsWith("__Host-arhdesign_csrf="));
  return cookie ? decodeURIComponent(cookie.slice(cookie.indexOf("=") + 1)) : "";
}
