import { useEffect, useState } from "react";
import { getSummary } from "@/api/reviews";
import { errorMessage } from "@/lib/errors";
import type { Summary } from "@/schemas";

const REFRESH_MS = 60_000;

// useSummary mirrors useReviews: initial load shows loading/error, then it
// silently refreshes on the same interval so the bar stays in step with the list.
export function useSummary(appId: string | null, hours: number) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!appId) {
      setSummary(null);
      return;
    }
    let active = true;

    const load = (initial: boolean) => {
      if (initial) {
        setLoading(true);
        setError(null);
      }
      getSummary(appId, hours)
        .then((result) => {
          if (active) {
            setSummary(result);
          }
        })
        .catch((err) => {
          if (active && initial) {
            setError(errorMessage(err));
          }
        })
        .finally(() => {
          if (active && initial) {
            setLoading(false);
          }
        });
    };

    load(true);
    const interval = setInterval(() => load(false), REFRESH_MS);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [appId, hours]);

  return { summary, loading, error };
}
