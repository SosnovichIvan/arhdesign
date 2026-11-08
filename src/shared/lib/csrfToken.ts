export function csrfToken(cookieHeader = document.cookie, protocol = window.location.protocol) {
  const cookieName = protocol === "https:" ? "__Host-arhdesign_csrf" : "arhdesign_csrf";
  const prefix = `${cookieName}=`;
  const cookie = cookieHeader.split("; ").find((value) => value.startsWith(prefix));
  if (!cookie) return "";
  try {
    return decodeURIComponent(cookie.slice(prefix.length));
  } catch {
    return "";
  }
}
