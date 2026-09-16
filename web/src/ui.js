const buttonBase =
  "cursor-pointer rounded-sm border whitespace-nowrap motion-safe:transition-colors disabled:cursor-not-allowed disabled:opacity-40";

export const primaryButton = `${buttonBase} border-accent bg-accent px-3 py-2 text-sm text-white hover:bg-accent-dark`;

export const button = `${buttonBase} border-line bg-white px-3 py-2 text-sm hover:bg-hover`;

export const rowButton = `${buttonBase} border-line bg-white px-2 py-1 text-2xs hover:bg-hover`;

export const dangerButton = `${rowButton} text-alert`;

export const input =
  "min-w-0 rounded-sm border border-line bg-white px-2.5 py-2 text-sm text-ink placeholder:text-faint";

export const select =
  "cursor-pointer rounded-sm border border-line bg-white px-2 py-1 text-xs text-ink";

export const alert =
  "my-3 rounded-sm border border-alert-line px-3 py-2 text-sm text-alert";

export const dot = "inline-block h-1.5 w-1.5 shrink-0 rounded-full";

export const badge = "badge inline-flex items-center gap-2 text-xs text-muted";

export const gridRow =
  "grid w-full grid-cols-[56px_minmax(100px,1fr)_110px_140px] items-center gap-3 max-narrow:grid-cols-[32px_minmax(80px,1fr)_74px_84px] max-narrow:gap-2";

export const statusDot = {
  pending: "bg-pending",
  running: "bg-running",
  completed: "bg-completed",
  failed: "bg-failed",
};
