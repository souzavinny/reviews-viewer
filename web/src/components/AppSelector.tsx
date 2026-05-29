import type { App } from "@/schemas";

// A native <select> keeps the bundle lean — a styled dropdown component would
// pull in a positioning engine for a short, static list.
export function AppSelector({
  apps,
  value,
  onChange,
}: {
  apps: App[];
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      aria-label="Select app"
      className="h-9 rounded-lg border border-input bg-transparent px-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      {apps.map((app) => (
        <option key={app.id} value={app.id}>
          {app.name ?? app.id}
        </option>
      ))}
    </select>
  );
}
