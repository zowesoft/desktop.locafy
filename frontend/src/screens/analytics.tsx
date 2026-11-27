import { useMemo, useState } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Clock, Cpu, HardDrive, Info, MemoryStick, TrendingUp } from "lucide-react";

import * as AnalyticsService from "../../bindings/locafy.dev/desktop/internal/analytics/service";
import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { useProjectLifecycles } from "@/lib/lifecycle-query";
import { useProjects } from "@/lib/projects-query";
import { useMachineStatus } from "@/lib/usage-query";
import { cn } from "@/lib/utils";

const analyticsKey = (days: number) => ["analytics", "trend", days] as const;
const retentionKey = ["analytics", "retention"] as const;
const windows = [7, 30, 90] as const;
const retentionChoices = [30, 90, 180, 365];

function gigabytes(bytes: number): string {
  return (bytes / 1024 ** 3).toFixed(1);
}

function dayLabel(day: string): string {
  const date = new Date(`${day}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return day;
  return date.toLocaleDateString([], { month: "short", day: "numeric" });
}

function recordedAt(value: string): Date {
  // History rows are stored as UTC "YYYY-MM-DD HH:MM:SS" without a zone marker.
  const isPlain = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value);
  return new Date(isPlain ? `${value.replace(" ", "T")}Z` : value);
}

function clockTime(value: string): string {
  const date = recordedAt(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleTimeString([], { hour12: false, hour: "2-digit", minute: "2-digit" });
}

function Sparkline({ data, color }: { data: number[]; color: string }) {
  const max = Math.max(...data);
  const min = Math.min(...data) * 0.9;
  const range = max - min || 1;
  const height = 40;
  const width = 120;
  const step = width / Math.max(1, data.length - 1);
  const points = data
    .map((value, index) => `${index * step},${height - ((value - min) / range) * height}`)
    .join(" ");

  return (
    <svg width={width} height={height} className="shrink-0" aria-hidden>
      <polyline
        points={points}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={color}
      />
    </svg>
  );
}

function ResourceCard({
  label,
  current,
  max,
  unit,
  icon: Icon,
  color,
  sparkline,
  scope,
}: {
  label: string;
  current: string;
  max?: string;
  unit: string;
  icon: React.ElementType;
  color: string;
  sparkline?: number[];
  scope: string;
}) {
  return (
    <Card>
      <CardContent className="pt-5">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Icon className="size-3.5" />
              {label}
            </div>
            <p className="text-2xl font-semibold tabular-nums">
              {current}
              <span className="ml-1 text-sm font-normal text-muted-foreground">{unit}</span>
            </p>
            {max && (
              <p className="text-xs text-muted-foreground">
                of {max} {unit}
              </p>
            )}
            <Badge variant="secondary" className="text-xs font-normal">
              {scope}
            </Badge>
          </div>
          {sparkline && sparkline.length > 1 ? (
            <Sparkline data={sparkline} color={color} />
          ) : (
            <span
              className="flex h-10 w-[120px] shrink-0 items-center justify-center rounded border border-dashed border-border text-[0.65rem] text-muted-foreground"
              title="Readings are not kept between samples, so there is no trend to draw yet"
            >
              no trend yet
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function ActivityDot({ scope }: { scope: string }) {
  const styles: Record<string, string> = {
    machine: "bg-muted-foreground",
    service: "bg-blue-400",
    project: "bg-primary",
    application: "bg-success",
  };
  return <span className={cn("mt-1.5 size-2 shrink-0 rounded-full", styles[scope] ?? "bg-muted-foreground")} />;
}

export function AnalyticsScreen() {
  const queryClient = useQueryClient();
  const [days, setDays] = useState<number>(30);

  const trend = useQuery({
    queryKey: analyticsKey(days),
    queryFn: () => AnalyticsService.Trend(days),
    refetchInterval: 30_000,
  });
  const retention = useQuery({
    queryKey: retentionKey,
    queryFn: () => AnalyticsService.Retention(),
  });
  const changeRetention = useMutation({
    mutationFn: (next: number) => AnalyticsService.SetRetention(next),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["analytics"] });
    },
  });

  const machine = useMachineStatus();
  const projects = useProjects();
  const lifecycles = useProjectLifecycles();
  const projectList = projects.data ?? [];
  const applicationQueries = useQueries({
    queries: projectList.map((project) => ({
      queryKey: ["applications", project.ID],
      queryFn: async () => (await ApplicationsService.List(project.ID)) ?? [],
    })),
  });
  const applicationsByProject = new Map(
    projectList.map((project, index) => [project.ID, applicationQueries[index]?.data ?? []]),
  );

  const recent = useQuery({
    queryKey: ["analytics", "recent"],
    queryFn: async () => (await AnalyticsService.Recent(50)) ?? [],
    refetchInterval: 30_000,
  });
  const today = new Date().toDateString();
  const todaysActivity = (recent.data ?? []).filter(
    (entry) => recordedAt(entry.at).toDateString() === today,
  );

  const data = trend.data;
  const total = data?.total ?? 0;
  const peak = Math.max(1, ...(data?.days ?? []).map((point) => point.count));
  const busiest = useMemo(() => {
    return (data?.days ?? []).reduce((best, point) => (point.count > best.count ? point : best), {
      day: "",
      count: 0,
    });
  }, [data]);
  const updatedSeconds = trend.dataUpdatedAt ? Math.round((Date.now() - trend.dataUpdatedAt) / 1000) : null;

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="border-b border-border px-6 py-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-lg font-semibold tracking-tight">Analytics</h1>
            <p className="mt-0.5 text-xs text-muted-foreground">
              Local-only resource and activity data · nothing is sent externally
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex rounded-md border border-border bg-muted/40 p-0.5">
              {windows.map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => setDays(option)}
                  className={cn(
                    "rounded px-2.5 py-1 text-xs transition-colors",
                    days === option
                      ? "bg-card text-foreground shadow-sm"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {option}d
                </button>
              ))}
            </div>
            <Badge variant="secondary" className="font-normal">
              <Clock className="mr-1 size-3" />
              {updatedSeconds === null ? "Reading…" : `Updated ${updatedSeconds} s ago`}
            </Badge>
          </div>
        </div>
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {/* Resource cards */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Resource usage — this machine
          </h2>
          <div className="grid gap-3 sm:grid-cols-3">
            <ResourceCard
              label="CPU"
              current={`${Math.round(machine.data?.CPUPercent ?? 0)}`}
              unit="%"
              icon={Cpu}
              color="text-primary"
              scope="Machine-wide"
            />
            <ResourceCard
              label="Memory"
              current={gigabytes(machine.data?.Memory.UsedBytes ?? 0)}
              max={gigabytes(machine.data?.Memory.TotalBytes ?? 0)}
              unit="GB"
              icon={MemoryStick}
              color="text-blue-400"
              scope="Machine-wide"
            />
            <ResourceCard
              label="Disk"
              current={gigabytes(machine.data?.Disk.UsedBytes ?? 0)}
              max={gigabytes(machine.data?.Disk.TotalBytes ?? 0)}
              unit="GB"
              icon={HardDrive}
              color="text-amber-400"
              scope="Machine-wide"
            />
          </div>
          <p className="text-[0.7rem] text-muted-foreground">
            These are the current reading; resource history is not stored, so no trend line is drawn.
          </p>
        </section>

        {/* Recorded actions */}
        <section className="space-y-2">
          <h2 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            <TrendingUp className="size-3.5" /> Recorded actions — last {days} days
          </h2>
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
            <Card>
              <CardContent className="space-y-3 pt-5">
                {trend.error ? (
                  <p role="alert" className="text-sm text-destructive">
                    Could not read the activity history: {String(trend.error)}
                  </p>
                ) : trend.isPending ? (
                  <p className="text-sm text-muted-foreground">Reading the history…</p>
                ) : total === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Nothing has been recorded in this window yet. Starting a project, attaching a
                    service or adding an application writes the first entries.
                  </p>
                ) : (
                  <>
                    <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
                      <span className="text-2xl font-semibold tabular-nums">{total}</span>
                      <span className="text-xs text-muted-foreground">actions in the last {days} days</span>
                      <span className="text-xs text-muted-foreground">
                        busiest day {busiest.count > 0 ? `${busiest.count} · ${dayLabel(busiest.day)}` : "—"}
                      </span>
                    </div>
                    <div
                      className="flex h-32 items-end gap-1"
                      role="img"
                      aria-label={`Recorded actions per day over ${days} days, ${total} in total`}
                    >
                      {(data?.days ?? []).map((point) => (
                        <div
                          key={point.day}
                          className={cn(
                            "min-w-0 flex-1 rounded-t-sm",
                            point.count === 0 ? "bg-muted/60" : "bg-primary/70 hover:bg-primary",
                          )}
                          style={{ height: `${Math.max(2, (point.count / peak) * 100)}%` }}
                          title={`${point.day}: ${point.count} action${point.count === 1 ? "" : "s"}`}
                        />
                      ))}
                    </div>
                    <div className="flex justify-between text-[0.7rem] text-muted-foreground">
                      <span>{dayLabel(data?.days?.[0]?.day ?? "")}</span>
                      <span>{dayLabel(data?.days?.[(data?.days?.length ?? 1) - 1]?.day ?? "")}</span>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardContent className="space-y-3 pt-5">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    What happened
                  </p>
                  <select
                    aria-label="Keep history for"
                    value={retention.data ?? ""}
                    disabled={changeRetention.isPending}
                    onChange={(event) => changeRetention.mutate(Number(event.target.value))}
                    className="h-7 rounded-md border border-border bg-background px-1.5 text-xs focus:outline-none"
                  >
                    {retentionChoices.map((choice) => (
                      <option key={choice} value={choice}>
                        Keep {choice} days
                      </option>
                    ))}
                  </select>
                </div>
                {(data?.actions?.length ?? 0) === 0 ? (
                  <p className="text-sm text-muted-foreground">No actions recorded in this window.</p>
                ) : (
                  <ul className="divide-y divide-border">
                    {(data?.actions ?? []).map((action) => {
                      const share = total === 0 ? 0 : Math.round((action.count / total) * 100);
                      return (
                        <li key={action.action} className="flex items-center gap-3 py-2">
                          <span className="min-w-0 flex-1 truncate text-xs">{action.action}</span>
                          <span className="h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-muted">
                            <span
                              className="block h-full rounded-full bg-primary"
                              style={{ width: `${Math.max(4, share)}%` }}
                            />
                          </span>
                          <span className="w-8 shrink-0 text-right font-mono text-xs text-muted-foreground">
                            {action.count}
                          </span>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>
        </section>

        {/* Status history */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Project status — now
          </h2>
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-muted/40">
                <tr>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Project</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Status</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Apps</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Running</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Folder</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {projectList.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-6 text-center text-xs text-muted-foreground">
                      No projects recorded yet.
                    </td>
                  </tr>
                ) : (
                  projectList.map((project) => {
                    const apps = applicationsByProject.get(project.ID) ?? [];
                    const running = apps.filter((app) => app.Status === "running").length;
                    const state = lifecycles.data?.[project.ID] ?? project.LifecycleState;
                    return (
                      <tr key={project.ID}>
                        <td className="px-4 py-2.5 font-medium">{project.Name}</td>
                        <td className="px-4 py-2.5">
                          <span
                            className={cn(
                              "inline-flex items-center gap-1.5 text-xs capitalize",
                              state === "running"
                                ? "text-success"
                                : state === "error"
                                  ? "text-destructive"
                                  : "text-muted-foreground",
                            )}
                          >
                            <span
                              className={cn(
                                "size-1.5 rounded-full",
                                state === "running"
                                  ? "bg-success"
                                  : state === "error"
                                    ? "bg-destructive"
                                    : "bg-muted-foreground/40",
                              )}
                            />
                            {state}
                          </span>
                        </td>
                        <td className="px-4 py-2.5 text-muted-foreground">{apps.length}</td>
                        <td className="px-4 py-2.5 font-mono text-xs">{running}</td>
                        <td className="max-w-56 truncate px-4 py-2.5 font-mono text-xs text-muted-foreground">
                          {project.RootPath}
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
          <p className="text-[0.7rem] text-muted-foreground">
            This is the state right now. Status changes are not stored yet, so the table cannot show a
            history of restarts or failures.
          </p>
        </section>

        {/* Activity timeline */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Activity timeline — today
          </h2>
          <div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
            {recent.isPending ? (
              <p className="px-4 py-6 text-xs text-muted-foreground">Reading today's activity…</p>
            ) : todaysActivity.length === 0 ? (
              <p className="px-4 py-6 text-xs text-muted-foreground">
                Nothing has been recorded today yet.
              </p>
            ) : (
              todaysActivity.map((entry) => (
                <div key={entry.id} className="flex items-start gap-3 px-4 py-2.5">
                  <span className="mt-0.5 w-10 shrink-0 font-mono text-xs text-muted-foreground/70">
                    {clockTime(entry.at)}
                  </span>
                  <ActivityDot scope={entry.scope} />
                  <span className="flex-1 text-xs">
                    {entry.action}
                    {entry.detail && <span className="text-muted-foreground"> — {entry.detail}</span>}
                  </span>
                  {entry.scope !== "machine" && (
                    <Badge variant="outline" className="shrink-0 text-xs font-normal capitalize">
                      {entry.scope}
                    </Badge>
                  )}
                </div>
              ))
            )}
          </div>
        </section>

        {/* Local-only note */}
        <div className="flex items-center gap-2 rounded-lg border border-border p-3 text-xs text-muted-foreground">
          <Info className="size-3.5 shrink-0" />
          All analytics data is stored locally on this machine and is never sent to Locafy servers.
          Action history is kept for {retention.data ?? 90} days and then removed automatically.
        </div>
      </div>
    </div>
  );
}
