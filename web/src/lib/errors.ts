import { ApiError } from "@/api/client";

// errorMessage turns a caught error into a string for display. It lives here
// (not in format.ts) so the time-formatting utilities stay free of the API layer.
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return err.message;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "Something went wrong";
}
