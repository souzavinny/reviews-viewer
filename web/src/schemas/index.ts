import { z } from "zod";

// Schemas mirror api/openapi.yaml — the contract for both ends. Timestamps are
// RFC 3339 strings (kept as strings; the UI formats them).

export const reviewSchema = z.object({
  id: z.string(),
  appId: z.string(),
  author: z.string(),
  content: z.string(),
  score: z.number().int().min(1).max(5),
  submittedAt: z.string(),
});
export type Review = z.infer<typeof reviewSchema>;

export const appSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
});
export type App = z.infer<typeof appSchema>;

export const addAppRequestSchema = z.object({
  id: z.string().regex(/^\d+$/, "app id must be numeric"),
});
export type AddAppRequest = z.infer<typeof addAppRequestSchema>;

export const summarySchema = z.object({
  total: z.number().int(),
  average: z.number(),
  countByStar: z.record(z.string(), z.number().int()),
  lastUpdated: z.string(),
});
export type Summary = z.infer<typeof summarySchema>;

export const reviewsSchema = z.array(reviewSchema);
export const appsSchema = z.array(appSchema);

export const errorSchema = z.object({ error: z.string() });
export type ErrorResponse = z.infer<typeof errorSchema>;
