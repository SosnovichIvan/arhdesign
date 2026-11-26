import { csrfToken } from "./csrfToken";

const accountURL = "/api/v1/me";
let pendingRefresh: Promise<boolean> | undefined;

async function refreshSession() {
  const token = csrfToken();
  if (!token) return false;
  const response = await fetch("/api/v1/auth/refresh", {
    credentials: "same-origin",
    headers: { "X-CSRF-Token": token },
    method: "POST",
    signal: AbortSignal.timeout(10_000),
  });
  return response.ok;
}

function restoreSession() {
  pendingRefresh ??= (navigator.locks
    ? navigator.locks.request("arhdesign:refresh-session", async () => {
      // Another tab may have rotated the shared cookies while we waited.
      const account = await fetch(accountURL, { credentials: "same-origin", signal: AbortSignal.timeout(10_000) });
      return account.status === 401 ? refreshSession() : account.ok;
    })
    : refreshSession()).finally(() => { pendingRefresh = undefined; });
  return pendingRefresh;
}

export async function getWithAccountSession(url: string, signal?: AbortSignal) {
  const options = { credentials: "same-origin", signal } as const;
  const response = await fetch(url, options);
  if (response.status !== 401 || !csrfToken()) return response;
  signal?.throwIfAborted();
  return await restoreSession() ? fetch(url, options) : response;
}
