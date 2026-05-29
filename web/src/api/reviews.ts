import { getJSON, sendJSON, sendNoContent } from "@/api/client";
import {
  type AddAppRequest,
  type App,
  appSchema,
  appsSchema,
  type Review,
  reviewsSchema,
  type Summary,
  summarySchema,
} from "@/schemas";

export function listApps(): Promise<App[]> {
  return getJSON("/apps", appsSchema);
}

export function addApp(id: string): Promise<App> {
  const body: AddAppRequest = { id };
  return sendJSON("/apps", "POST", body, appSchema);
}

export function removeApp(id: string): Promise<void> {
  return sendNoContent(`/apps/${encodeURIComponent(id)}`, "DELETE");
}

export function getReviews(appId: string, hours?: number): Promise<Review[]> {
  return getJSON(appResource(appId, "reviews", hours), reviewsSchema);
}

export function getSummary(appId: string, hours?: number): Promise<Summary> {
  return getJSON(appResource(appId, "summary", hours), summarySchema);
}

function appResource(appId: string, resource: string, hours?: number): string {
  const path = `/apps/${encodeURIComponent(appId)}/${resource}`;
  return hours === undefined ? path : `${path}?hours=${hours}`;
}
