import { useQuery } from "@tanstack/react-query";
import { Bell, CircleCheck, HardDrive, Monitor, Clock, TriangleExclamationPoint } from "lucide-react";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import { buttonVariants } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

const statusKey = ["engine-status"] as const;
const notificationsKey = ["notifications"] as const;
const gibibyte = 1024 ** 3;

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 GB";
  if (bytes >= gibibyte) {
    const value = bytes / gibibyte;
    return `${value >= 10 ? Math.round(value) : value.toFixed(1)} GB`;
  }
  return `${Math.round(bytes / 1024 ** 2)} MB`;
}

/** Pressure is shown as colour plus the percentage, so it never relies on colour alone. */
function usageTone(percent: number): string {
  if (percent >= 90) return "bg-destructive";
  if (percent >= 75) return "bg-accent";
  return "bg-success";
}

function formatUptime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m`;
  return `${seconds}s`;
}

function formatRelative(iso: string): string {
  const timestamp = Date.parse(iso);
  if (Number.isNaN(timestamp)) return "";
  const seconds = Math.max(0, Math.round((Date.now() - timestamp) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

function DetailRow({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Monitor;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-start gap-2">
      <Icon className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
      <span className="w-20 shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 flex-1 break-words text-right">{value}</span>
    </div>
  );
}

function ResourceBar({
  label,
  percent,
  detail,
}: {
  label: string;
  percent: number;
  detail: string;
}) {
  const bounded = Math.min(100, Math.max(0, Math.round(percent)));

  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium tabular-nums">{bounded}%</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuenow={bounded}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-1.5 overflow-hidden rounded-full bg-muted"
      >
        <div
          className={cn("h-full rounded-full transition-[width]", usageTone(bounded))}
          style={{ width: `${bounded}%` }}
        />
      </div>
      <p className="text-[0.7rem] text-muted-foreground">{detail}</p>
    </div>
  );
}

/** Engine and machine state, read from the backend rather than assumed. */
export function EngineStatus() {
  const status = useQuery({
    queryKey: statusKey,
    queryFn: () => StatusService.Health(),
    refetchInterval: 30_000,
  });

  const healthy = status.data?.Healthy === true;
  const label = status.isPending
    ? "Checking…"
    : status.isError || !status.data
      ? "Status unavailable"
      : healthy
        ? "Locafy running"
        : "Needs attention";

  return (
    <Popover>
      {/* The trigger is the button itself: passing a component through `render`
          would drop the anchor ref it needs to position the popover. */}
      <PopoverTrigger
        className={cn(buttonVariants({ variant: "secondary", size: "sm" }), "gap-2 font-normal")}
      >
        <span
          className={cn(
            "size-1.5 rounded-full",
            status.isPending
              ? "bg-muted-foreground"
              : healthy
                ? "bg-success"
                : "bg-destructive",
          )}
        />
        {label}
        {healthy && status.data && (
          <span className="tabular-nums text-muted-foreground">
            {Math.round(status.data.CPUPercent)}% CPU
          </span>
        )}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80">
        <PopoverHeader>
          <PopoverTitle className="flex items-center gap-2">
            <CircleCheck
              className={cn("size-4", healthy ? "text-success" : "text-destructive")}
            />
            {status.data?.Name ?? "Locafy"}
          </PopoverTitle>
          <PopoverDescription>
            {status.isError
              ? "The engine did not respond to a status request."
              : (status.data?.Summary ?? "Reading engine status…")}
          </PopoverDescription>
        </PopoverHeader>

        {status.data && (
          <div className="mt-3 space-y-1.5 text-xs">
            <DetailRow icon={Monitor} label="Machine" value={status.data.Hostname} />
            <DetailRow
              icon={Monitor}
              label="Platform"
              value={`${status.data.Platform} · ${status.data.Architecture}`}
            />
            <DetailRow
              icon={HardDrive}
              label="Database"
              value={
                status.data.DatabaseOK ? (status.data.DatabasePath ?? "Connected") : "Unavailable"
              }
            />
            <DetailRow
              icon={Clock}
              label="Uptime"
              value={formatUptime(status.data.UptimeSeconds)}
            />
          </div>
        )}

        {status.data && (
          <div className="mt-4 space-y-3 border-t border-border pt-3">
            <ResourceBar
              label="CPU"
              percent={status.data.CPUPercent}
              detail="Averaged over the last few seconds"
            />
            <ResourceBar
              label="Memory"
              percent={status.data.Memory.Percent}
              detail={`${formatBytes(status.data.Memory.UsedBytes)} of ${formatBytes(status.data.Memory.TotalBytes)} in use`}
            />
            <ResourceBar
              label="Disk"
              percent={status.data.Disk.Percent}
              detail={`${formatBytes(status.data.Disk.UsedBytes)} of ${formatBytes(status.data.Disk.TotalBytes)} on the data volume`}
            />
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}

/** Recent engine activity, so warnings and failures have somewhere to appear. */
export function Notifications() {
  const notifications = useQuery({
    queryKey: notificationsKey,
    queryFn: async () => (await StatusService.Notifications(10)) ?? [],
    refetchInterval: 60_000,
  });

  const entries = notifications.data ?? [];
  const count = entries.length;
  const warnings = entries.filter((entry) => entry.Severity === "warning").length;
  const label =
    count > 0
      ? `Notifications, ${count} recent entries${warnings > 0 ? `, ${warnings} warning` : ""}`
      : "Notifications";

  return (
    <Popover>
      <PopoverTrigger
        aria-label={label}
        className={cn(buttonVariants({ variant: "ghost", size: "icon" }), "relative")}
      >
        <Bell />
        {count > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[0.6rem] font-medium leading-4 text-accent-foreground">
            {count > 9 ? "9+" : count}
          </span>
        )}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80">
        <PopoverHeader>
          <PopoverTitle>Notifications</PopoverTitle>
          <PopoverDescription>
            Recent engine activity. Warnings and failures are recorded here as features add them.
          </PopoverDescription>
        </PopoverHeader>

        {notifications.isPending ? (
          <p className="mt-3 text-xs text-muted-foreground">Loading…</p>
        ) : notifications.isError ? (
          <p role="alert" className="mt-3 text-xs text-destructive">
            Could not read activity: {String(notifications.error)}
          </p>
        ) : count === 0 ? (
          <p className="mt-3 text-xs text-muted-foreground">Nothing recorded yet.</p>
        ) : (
          <ul className="mt-3 max-h-64 divide-y divide-border overflow-y-auto">
            {entries.map((entry) => {
              const isWarning = entry.Severity === "warning";

              return (
              <li key={entry.ID} className="flex items-start gap-2 py-2">
                {isWarning ? (
                  <TriangleExclamationPoint className="mt-0.5 size-3.5 shrink-0 text-accent" />
                ) : (
                  <span className="mt-1 size-1.5 shrink-0 rounded-full bg-muted-foreground/60" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="flex items-center gap-1.5 truncate text-xs">
                    {entry.Action}
                    {isWarning && (
                      <span className="shrink-0 text-[0.65rem] font-medium uppercase tracking-wide text-accent">
                        Warning
                      </span>
                    )}
                  </p>
                  <p className="truncate text-[0.7rem] text-muted-foreground">
                    {entry.Scope}
                    {entry.Detail ? ` · ${entry.Detail}` : ""}
                  </p>
                </div>
                <span className="shrink-0 text-[0.7rem] text-muted-foreground">
                  {formatRelative(entry.CreatedAt)}
                </span>
              </li>
              );
            })}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  );
}
