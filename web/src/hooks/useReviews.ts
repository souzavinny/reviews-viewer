import { useEffect, useState } from "react";
import { getReviews } from "@/api/reviews";
import { errorMessage } from "@/lib/errors";
import type { Review } from "@/schemas";

const REFRESH_MS = 60_000;

// useReviews loads an app's reviews for the window and silently re-fetches on an
// interval so the list stays current without flashing the loading state.
export function useReviews(appId: string | null, hours: number) {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!appId) {
      setReviews([]);
      setError(null);
      return;
    }
    let active = true;

    const load = (initial: boolean) => {
      if (initial) {
        setLoading(true);
        setError(null);
      }
      getReviews(appId, hours)
        .then((result) => {
          if (active) {
            setReviews(result);
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

  return { reviews, loading, error };
}
