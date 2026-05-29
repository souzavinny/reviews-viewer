const absoluteFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
});

const relativeFormat = new Intl.RelativeTimeFormat(undefined, {
  numeric: "auto",
});

const DIVISIONS: { amount: number; unit: Intl.RelativeTimeFormatUnit }[] = [
  { amount: 60, unit: "second" },
  { amount: 60, unit: "minute" },
  { amount: 24, unit: "hour" },
  { amount: 7, unit: "day" },
  { amount: 4.34524, unit: "week" },
  { amount: 12, unit: "month" },
  { amount: Number.POSITIVE_INFINITY, unit: "year" },
];

export function formatAbsolute(iso: string): string {
  return absoluteFormat.format(new Date(iso));
}

export function formatRelative(iso: string): string {
  let delta = (new Date(iso).getTime() - Date.now()) / 1000;
  for (const division of DIVISIONS) {
    if (Math.abs(delta) < division.amount) {
      return relativeFormat.format(Math.round(delta), division.unit);
    }
    delta /= division.amount;
  }
  return formatAbsolute(iso);
}
