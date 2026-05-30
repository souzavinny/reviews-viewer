import { type ReactNode, useEffect, useState } from "react";
import { AddAppForm } from "@/components/AddAppForm";
import { AppBanner } from "@/components/AppBanner";
import { AppSelector } from "@/components/AppSelector";
import { Pagination } from "@/components/Pagination";
import { ReviewList } from "@/components/ReviewList";
import { SummaryBar } from "@/components/SummaryBar";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { WindowSelector } from "@/components/WindowSelector";
import { useApps } from "@/hooks/useApps";
import { useReviews } from "@/hooks/useReviews";
import { useSummary } from "@/hooks/useSummary";
import { errorMessage } from "@/lib/errors";

const DEFAULT_HOURS = 48;
const PAGE_SIZE = 20;
const WIDEST_WINDOW_HOURS = 168; // mirrors the widest option in WindowSelector

function windowLabel(hours: number): string {
  return hours === WIDEST_WINDOW_HOURS ? "7 days" : `${hours} hours`;
}

export function ReviewsPage() {
  const {
    apps,
    loading: appsLoading,
    error: appsError,
    add,
    remove,
  } = useApps();
  const [selected, setSelected] = useState<string | null>(null);
  const [hours, setHours] = useState(DEFAULT_HOURS);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (apps.length === 0) {
      setSelected(null);
    } else if (!selected || !apps.some((app) => app.id === selected)) {
      setSelected(apps[0].id);
    }
  }, [apps, selected]);

  const {
    reviews,
    loading: reviewsLoading,
    error: reviewsError,
  } = useReviews(selected, hours);
  const { summary } = useSummary(selected, hours);
  const selectedApp = apps.find((app) => app.id === selected);

  async function handleRemove() {
    if (!selected) {
      return;
    }
    setActionError(null);
    try {
      await remove(selected);
    } catch (err) {
      setActionError(errorMessage(err));
    }
  }

  // Display-only pagination over the full in-memory list (no API change).
  const [page, setPage] = useState(0);
  // Reset to the first page whenever the app or window changes — React's
  // adjust-state-on-change pattern, so it also covers auto-selection.
  const windowKey = `${selected ?? ""}:${hours}`;
  const [prevWindowKey, setPrevWindowKey] = useState(windowKey);
  if (windowKey !== prevWindowKey) {
    setPrevWindowKey(windowKey);
    setPage(0);
  }

  const totalPages = Math.max(1, Math.ceil(reviews.length / PAGE_SIZE));
  // Clamp for display so a refresh that shrinks the list lands on the last page.
  const currentPage = Math.min(page, totalPages - 1);
  const visibleReviews = reviews.slice(
    currentPage * PAGE_SIZE,
    currentPage * PAGE_SIZE + PAGE_SIZE,
  );

  let body: ReactNode;
  if (!selected) {
    body = appsLoading ? (
      <LoadingState />
    ) : (
      <EmptyState message="Add an app by its App Store id to see reviews." />
    );
  } else if (reviewsLoading) {
    body = <LoadingState />;
  } else if (reviewsError && reviews.length === 0) {
    body = <ErrorState message={reviewsError} />;
  } else if (reviews.length === 0) {
    body = (
      <EmptyState
        message={`No reviews in the last ${windowLabel(hours)}.`}
        hint={
          hours < WIDEST_WINDOW_HOURS
            ? "The App Store feed usually lags about a day, so the newest reviews are often 24–48h old. Try a wider window."
            : undefined
        }
      />
    );
  } else {
    body = (
      <div className="space-y-4">
        <ReviewList reviews={visibleReviews} />
        {totalPages > 1 && (
          <Pagination
            page={currentPage}
            totalPages={totalPages}
            onPageChange={setPage}
          />
        )}
      </div>
    );
  }

  return (
    <div className="mx-auto flex min-h-svh w-full max-w-3xl flex-col gap-6 p-6">
      <header className="space-y-4">
        <div>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">
            App Store reviews
          </h1>
          <p className="text-sm text-muted-foreground">
            Recent customer reviews, newest first.
          </p>
        </div>

        {apps.length > 0 && selected && (
          <div className="flex flex-wrap items-center gap-3">
            <AppSelector apps={apps} value={selected} onChange={setSelected} />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleRemove}
            >
              Remove
            </Button>
            <WindowSelector value={hours} onChange={setHours} />
          </div>
        )}

        <AddAppForm onAdd={add} />
        {appsError && <ErrorState message={appsError} />}
        {actionError && <ErrorState message={actionError} />}
      </header>

      <main className="flex-1 space-y-4">
        {selectedApp && <AppBanner key={selectedApp.id} app={selectedApp} />}
        {summary && <SummaryBar summary={summary} />}
        {body}
      </main>
    </div>
  );
}
