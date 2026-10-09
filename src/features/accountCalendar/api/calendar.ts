import { readApiError } from "@/shared/lib/readApiError";

import type { GlobalCalendarFeed } from "../model/types";

export async function loadGlobalCalendar(from: string, to: string, projectIds: string[], signal?: AbortSignal): Promise<GlobalCalendarFeed> {
  const query = new URLSearchParams({ from, to });
  projectIds.forEach((projectId) => query.append("projectId", projectId));
  const response = await fetch(`/api/v1/calendar?${query}`, { credentials: "same-origin", signal });
  if (!response.ok) throw new Error(await readApiError(response, "Не удалось загрузить календарь проектов"));
  return response.json() as Promise<GlobalCalendarFeed>;
}
