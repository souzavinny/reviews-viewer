import { StarIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { formatAbsolute, formatRelative } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Review } from "@/schemas";

function Stars({ score }: { score: number }) {
  return (
    <div
      role="img"
      aria-label={`${score} out of 5 stars`}
      className="flex shrink-0 items-center gap-0.5"
    >
      {[1, 2, 3, 4, 5].map((n) => (
        <StarIcon
          key={n}
          className={cn(
            "size-4",
            n <= score
              ? "fill-yellow-400 text-yellow-400"
              : "fill-muted text-muted-foreground/40",
          )}
        />
      ))}
    </div>
  );
}

export function ReviewCard({ review }: { review: Review }) {
  return (
    <Card>
      <CardContent className="space-y-2">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate font-medium">{review.author}</p>
            <p className="text-xs text-muted-foreground">
              {formatAbsolute(review.submittedAt)} ·{" "}
              {formatRelative(review.submittedAt)}
            </p>
          </div>
          <Stars score={review.score} />
        </div>
        <p className="whitespace-pre-wrap text-sm">{review.content}</p>
      </CardContent>
    </Card>
  );
}
