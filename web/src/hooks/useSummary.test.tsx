import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useSummary } from "@/hooks/useSummary";
import type { Summary } from "@/schemas";

const { getSummary } = vi.hoisted(() => ({ getSummary: vi.fn() }));
vi.mock("@/api/reviews", () => ({ getSummary }));

const summary: Summary = {
  total: 3,
  average: 4.33,
  countByStar: { "1": 0, "2": 0, "3": 1, "4": 0, "5": 2 },
  lastUpdated: "2026-05-28T06:25:43-07:00",
};

describe("useSummary", () => {
  it("fetches once for the given app and window, then exposes the summary", async () => {
    getSummary.mockResolvedValue(summary);

    const { result } = renderHook(() => useSummary("389801252", 48));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(getSummary).toHaveBeenCalledTimes(1);
    expect(getSummary).toHaveBeenCalledWith("389801252", 48);
    expect(result.current.summary).toEqual(summary);
    expect(result.current.error).toBeNull();
  });

  it("surfaces an error message when the fetch rejects", async () => {
    getSummary.mockReset();
    getSummary.mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useSummary("389801252", 48));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe("network down");
    expect(result.current.summary).toBeNull();
  });
});
