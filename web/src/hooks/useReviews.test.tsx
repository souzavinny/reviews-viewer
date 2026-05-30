import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useReviews } from "@/hooks/useReviews";
import type { Review } from "@/schemas";

const { getReviews } = vi.hoisted(() => ({ getReviews: vi.fn() }));
vi.mock("@/api/reviews", () => ({ getReviews }));

const reviews: Review[] = [
  {
    id: "r1",
    appId: "389801252",
    author: "SuoQin16",
    content: "Great app.",
    score: 5,
    submittedAt: "2026-05-28T06:25:43-07:00",
  },
];

describe("useReviews", () => {
  it("fetches once for the given app and window, then exposes the reviews", async () => {
    getReviews.mockResolvedValue(reviews);

    const { result } = renderHook(() => useReviews("389801252", 48));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(getReviews).toHaveBeenCalledTimes(1);
    expect(getReviews).toHaveBeenCalledWith("389801252", 48);
    expect(result.current.reviews).toEqual(reviews);
    expect(result.current.error).toBeNull();
  });

  it("surfaces an error message when the fetch rejects", async () => {
    getReviews.mockReset();
    getReviews.mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useReviews("389801252", 48));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe("network down");
    expect(result.current.reviews).toEqual([]);
  });
});
