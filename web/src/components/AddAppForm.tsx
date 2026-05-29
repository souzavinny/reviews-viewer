import { type FormEvent, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { errorMessage } from "@/lib/errors";
import { addAppRequestSchema } from "@/schemas";

export function AddAppForm({
  onAdd,
}: {
  onAdd: (id: string) => Promise<void>;
}) {
  const [id, setId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const parsed = addAppRequestSchema.safeParse({ id: id.trim() });
    if (!parsed.success) {
      setError("App id must be numeric");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onAdd(parsed.data.id);
      setId("");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex items-start gap-2">
      <div className="flex flex-col gap-1">
        <Input
          value={id}
          onChange={(event) => setId(event.target.value)}
          placeholder="App Store id (e.g. 389801252)"
          inputMode="numeric"
          aria-label="App Store id"
          className="w-60"
        />
        {error && <p className="text-xs text-destructive">{error}</p>}
      </div>
      <Button type="submit" disabled={submitting || id.trim() === ""}>
        {submitting ? "Adding…" : "Add app"}
      </Button>
    </form>
  );
}
