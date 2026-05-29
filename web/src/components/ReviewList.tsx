import { ReviewCard } from "@/components/ReviewCard";
import type { Review } from "@/schemas";

export function ReviewList({ reviews }: { reviews: Review[] }) {
  return (
    <div className="space-y-3">
      {reviews.map((review) => (
        <ReviewCard key={review.id} review={review} />
      ))}
    </div>
  );
}
