import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ReviewCard } from "@/components/ReviewCard";
import { formatAbsolute } from "@/lib/format";
import type { Review } from "@/schemas";

const review: Review = {
  id: "r1",
  appId: "389801252",
  author: "SuoQin16",
  content: "I really need to be able to edit a playlist offline.",
  score: 4,
  submittedAt: "2026-05-28T06:25:43-07:00",
};

describe("ReviewCard", () => {
  it("renders author, content, submitted time, and score as stars", () => {
    const { container } = render(<ReviewCard review={review} />);
    const text = container.textContent ?? "";

    expect(text).toContain(review.author);
    expect(text).toContain(review.content);
    expect(text).toContain(formatAbsolute(review.submittedAt));
    expect(
      container.querySelector('[aria-label="4 out of 5 stars"]'),
    ).not.toBeNull();
  });
});
