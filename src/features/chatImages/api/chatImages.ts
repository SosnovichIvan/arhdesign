import { csrfToken, readApiError } from "@/shared/lib";

export type ChatLocation = { chatId: string; projectId?: string };
export type ChatImage = {
  id: string;
  name: string;
  width: number;
  height: number;
  pinned: boolean;
  canDelete: boolean;
};
export type ChatImagePage = { items: ChatImage[]; nextCursor?: string };

export function chatImageURL({ chatId, projectId }: ChatLocation, imageId?: string, cursor?: string) {
  const params = new URLSearchParams();
  if (projectId) params.set("projectId", projectId);
  if (cursor) params.set("cursor", cursor);
  const query = params.toString();
  const image = imageId ? `/${encodeURIComponent(imageId)}` : "";
  return `/api/v1/chat-images/${encodeURIComponent(chatId)}${image}${query ? `?${query}` : ""}`;
}

export async function loadChatImages(location: ChatLocation, cursor?: string, signal?: AbortSignal) {
  const response = await fetch(chatImageURL(location, undefined, cursor), { credentials: "same-origin", signal });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось загрузить изображения"));
  return response.json() as Promise<ChatImagePage>;
}

export async function uploadChatImage(location: ChatLocation, file: File, key: string) {
  const response = await fetch(chatImageURL(location), {
    method: "POST",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/octet-stream",
      "Idempotency-Key": key,
      "X-CSRF-Token": csrfToken(),
      "X-File-Name": `UTF-8''${encodeURIComponent(file.name)}`,
    },
    body: file,
  });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось прикрепить изображение"));
}

export async function setChatImagePin(location: ChatLocation, image: ChatImage) {
  const response = await fetch(chatImageURL(location, image.id), {
    method: "PUT",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() },
    body: JSON.stringify({ pinned: !image.pinned }),
  });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось изменить закрепление"));
}

export async function deleteChatImage(location: ChatLocation, imageId: string) {
  const response = await fetch(chatImageURL(location, imageId), {
    method: "DELETE",
    credentials: "same-origin",
    headers: { "X-CSRF-Token": csrfToken() },
  });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось удалить изображение"));
}
