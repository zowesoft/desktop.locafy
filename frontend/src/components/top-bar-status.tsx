import { useQuery } from "@tanstack/react-query";
import { Bell, CircleCheck, HardDrive, Monitor, Clock } from "lucide-react";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import { Button } from "@/components/ui/button";
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
      <PopoverTrigger
        render={
          <Button variant="secondary" size="sm" className="gap-2 font-normal">
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
          </Button>
        }
      />
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

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label={count > 0 ? `Notifications, ${count} recent entries` : "Notifications"}
            className="relative"
          >
            <Bell />
            {count > 0 && (
              <span className="absolute -right-0.5 -top-0.5 flex min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[0.6rem] font-medium leading-4 text-accent-foreground">
                {count > 9 ? "9+" : count}
              </span>
            )}
          </Button>
        }
      />
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
            {entries.map((entry) => (
              <li key={entry.ID} className="flex items-start gap-2 py-2">
                <span className="mt-1 size-1.5 shrink-0 rounded-full bg-muted-foreground/60" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs">{entry.Action}</p>
                  <p className="truncate text-[0.7rem] text-muted-foreground">
                    {entry.Scope}
                    {entry.Detail ? ` · ${entry.Detail}` : ""}
                  </p>
                </div>
                <span className="shrink-0 text-[0.7rem] text-muted-foreground">
                  {formatRelative(entry.CreatedAt)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  );
}
