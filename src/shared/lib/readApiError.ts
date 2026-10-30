type JsonRecord = Record<string, unknown>;

function isJsonRecord(value: unknown): value is JsonRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFrom(value: unknown) {
  if (!isJsonRecord(value) || typeof value.message !== "string") return "";
  return value.message.trim();
}

export type ApiErrorDetails = {
  code: string;
  fields: Record<string, string>;
  message: string;
};

function codeFrom(value: unknown) {
  if (!isJsonRecord(value) || typeof value.code !== "string") return "";
  return value.code.trim();
}

function fieldsFrom(value: unknown): Record<string, string> {
  if (!isJsonRecord(value) || !isJsonRecord(value.fields)) return {};
  return Object.fromEntries(Object.entries(value.fields).flatMap(([field, message]) => {
    if (typeof message !== "string" || !message.trim()) return [];
    return [[field, message.trim()]];
  }));
}

export async function readApiErrorDetails(response: Response, fallback: string): Promise<ApiErrorDetails> {
  try {
    const payload: unknown = await response.json();
    if (!isJsonRecord(payload)) return { code: "", fields: {}, message: fallback };
    return {
      code: codeFrom(payload.error),
      fields: fieldsFrom(payload.error),
      message: messageFrom(payload.error) || messageFrom(payload) || fallback,
    };
  } catch {
    return { code: "", fields: {}, message: fallback };
  }
}

export async function readApiError(response: Response, fallback: string) {
  return (await readApiErrorDetails(response, fallback)).message;
}
