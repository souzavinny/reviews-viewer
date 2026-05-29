import { formatRelative } from "@/lib/format";
import type { Summary } from "@/schemas";

const STARS = [5, 4, 3, 2, 1];

export function SummaryBar({ summary }: { summary: Summary }) {
  const max = Math.max(
    1,
    ...STARS.map((star) => summary.countByStar[String(star)] ?? 0),
  );

  return (
    <div className="flex flex-wrap items-center gap-6 rounded-xl border p-4">
      <div>
        <p className="text-2xl font-semibold">{summary.total}</p>
        <p className="text-xs text-muted-foreground">reviews</p>
      </div>
      <div>
        <p className="text-2xl font-semibold">{summary.average.toFixed(2)}</p>
        <p className="text-xs text-muted-foreground">avg score</p>
      </div>
      <div className="min-w-48 flex-1 space-y-1">
        {STARS.map((star) => {
          const count = summary.countByStar[String(star)] ?? 0;
          return (
            <div key={star} className="flex items-center gap-2 text-xs">
              <span className="w-3 text-muted-foreground">{star}</span>
              <div className="h-2 flex-1 overflow-hidden rounded bg-muted">
                <div
                  className="h-full bg-yellow-400"
                  style={{ width: `${(count / max) * 100}%` }}
                />
              </div>
              <span className="w-8 text-right text-muted-foreground">
                {count}
              </span>
            </div>
          );
        })}
      </div>
      {summary.total > 0 && (
        <p className="ml-auto self-start text-xs text-muted-foreground">
          updated {formatRelative(summary.lastUpdated)}
        </p>
      )}
    </div>
  );
}
