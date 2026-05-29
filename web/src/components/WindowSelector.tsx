import { Button } from "@/components/ui/button";

const WINDOWS = [
  { label: "24h", hours: 24 },
  { label: "48h", hours: 48 },
  { label: "7d", hours: 168 },
];

export function WindowSelector({
  value,
  onChange,
}: {
  value: number;
  onChange: (hours: number) => void;
}) {
  return (
    <div className="inline-flex gap-0.5 rounded-lg border p-0.5">
      {WINDOWS.map((window) => (
        <Button
          key={window.hours}
          type="button"
          size="sm"
          variant={value === window.hours ? "default" : "ghost"}
          onClick={() => onChange(window.hours)}
        >
          {window.label}
        </Button>
      ))}
    </div>
  );
}
