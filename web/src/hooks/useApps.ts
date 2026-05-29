import { useCallback, useEffect, useState } from "react";
import { addApp, listApps, removeApp } from "@/api/reviews";
import { errorMessage } from "@/lib/errors";
import type { App } from "@/schemas";

function byId(apps: App[]): App[] {
  return [...apps].sort((a, b) => a.id.localeCompare(b.id));
}

export function useApps() {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(() => {
    setLoading(true);
    setError(null);
    listApps()
      .then((list) => setApps(byId(list)))
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const add = useCallback(async (id: string) => {
    const app = await addApp(id);
    setApps((prev) => byId([...prev.filter((a) => a.id !== app.id), app]));
  }, []);

  const remove = useCallback(async (id: string) => {
    await removeApp(id);
    setApps((prev) => prev.filter((a) => a.id !== id));
  }, []);

  return { apps, loading, error, add, remove, reload };
}
