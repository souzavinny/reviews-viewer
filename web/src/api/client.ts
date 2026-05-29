import type { z } from "zod";
import { errorSchema } from "@/schemas";

type HttpMethod = "POST" | "PUT" | "PATCH" | "DELETE";

const baseURL = (
  import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080"
).replace(/\/+$/, "");

// ApiError carries the HTTP status and the server's error message so the UI can
// distinguish, e.g., a 404 (unknown app) from a transport failure.
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function send(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(`${baseURL}${path}`, init);
  if (!res.ok) {
    throw new ApiError(res.status, await errorMessage(res));
  }
  return res;
}

async function errorMessage(res: Response): Promise<string> {
  const body = await res.json().catch(() => null);
  const parsed = errorSchema.safeParse(body);
  return parsed.success ? parsed.data.error : `request failed (${res.status})`;
}

export async function getJSON<T>(
  path: string,
  schema: z.ZodType<T>,
): Promise<T> {
  const res = await send(path, { headers: { Accept: "application/json" } });
  return schema.parse(await res.json());
}

export async function sendJSON<T>(
  path: string,
  method: HttpMethod,
  body: unknown,
  schema: z.ZodType<T>,
): Promise<T> {
  const res = await send(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return schema.parse(await res.json());
}

export async function sendNoContent(
  path: string,
  method: HttpMethod,
): Promise<void> {
  await send(path, { method });
}
