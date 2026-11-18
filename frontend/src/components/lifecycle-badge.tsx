import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const states: Record<string, { label: string; dot: string }> = {
  running: { label: "Running", dot: "bg-success" },
  partial: { label: "Partially running", dot: "bg-accent" },
  starting: { label: "Starting", dot: "bg-accent" },
  stopping: { label: "Stopping", dot: "bg-accent" },
  stopped: { label: "Stopped", dot: "bg-muted-foreground" },
  error: { label: "Error", dot: "bg-destructive" },
};

/** A project's runtime state. The word carries the meaning; the dot only echoes it. */
export function LifecycleBadge({ state }: { state: string }) {
  const known = states[state];

  return (
    <Badge variant="outline" className="gap-1.5 font-normal">
      <span aria-hidden className={cn("size-1.5 rounded-full", known?.dot ?? "bg-muted-foreground")} />
      {known?.label ?? state}
    </Badge>
  );
}
