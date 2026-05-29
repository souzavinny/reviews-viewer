import { type ReactNode, useEffect, useState } from "react";
import { AddAppForm } from "@/components/AddAppForm";
import { AppSelector } from "@/components/AppSelector";
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
    body = <EmptyState message="No reviews in this window." />;
  } else {
    body = <ReviewList reviews={reviews} />;
  }

  return (
    <div className="mx-auto flex min-h-svh w-full max-w-3xl flex-col gap-6 p-6">
      <header className="space-y-4">
        <div>
          <h1 className="text-xl font-semibold">App Store reviews</h1>
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
        {summary && <SummaryBar summary={summary} />}
        {body}
      </main>
    </div>
  );
}
