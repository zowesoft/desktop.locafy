import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, CalendarDays, Database, ListChecks } from "lucide-react";

import * as AnalyticsService from "../../bindings/locafy.dev/desktop/internal/analytics/service";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const analyticsKey = (days: number) => ["analytics", "trend", days] as const;
const retentionKey = ["analytics", "retention"] as const;
const windows = [7, 30, 90] as const;
const retentionChoices = [30, 90, 180, 365];

function dayLabel(day: string): string {
  const date = new Date(`${day}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return day;
  return date.toLocaleDateString([], { month: "short", day: "numeric" });
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

  const data = trend.data;
  const busiest = useMemo(() => {
    const points = data?.days ?? [];
    return points.reduce((best, point) => (point.count > best.count ? point : best), {
      day: "",
      count: 0,
    });
  }, [data]);
  const peak = Math.max(1, ...(data?.days ?? []).map((point) => point.count));
  const total = data?.total ?? 0;

  return (
    <div className="mx-auto w-full max-w-6xl space-y-4 px-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold tracking-tight">Analytics</h1>
          <p className="text-xs text-muted-foreground">
            What Locafy recorded on this machine. Read from the local action history — no account, no
            upload.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="font-normal">
            Machine-wide
          </Badge>
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
        </div>
      </div>

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
              <Activity className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Recorded actions</p>
              <p className="text-xl font-semibold tracking-tight">{trend.isPending ? "—" : total}</p>
              <p className="text-xs text-muted-foreground">in the last {days} days</p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-accent">
              <CalendarDays className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Busiest day</p>
              <p className="text-xl font-semibold tracking-tight">
                {busiest.count > 0 ? busiest.count : "—"}
              </p>
              <p className="text-xs text-muted-foreground">
                {busiest.count > 0 ? dayLabel(busiest.day) : "nothing recorded yet"}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <ListChecks className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Action kinds</p>
              <p className="text-xl font-semibold tracking-tight">{data?.actions?.length ?? 0}</p>
              <p className="text-xs text-muted-foreground">distinct in this window</p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <Database className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">History kept</p>
              <p className="text-xl font-semibold tracking-tight">
                {retention.isPending ? "—" : `${retention.data ?? 0} days`}
              </p>
              <select
                aria-label="Keep history for"
                value={retention.data ?? ""}
                disabled={changeRetention.isPending}
                onChange={(event) => changeRetention.mutate(Number(event.target.value))}
                className="mt-1 h-7 rounded-md border border-border bg-background px-1.5 text-xs focus:outline-none"
              >
                {retentionChoices.map((choice) => (
                  <option key={choice} value={choice}>
                    Keep {choice} days
                  </option>
                ))}
              </select>
            </div>
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Actions per day</CardTitle>
          <CardDescription>
            Every recorded action, machine-wide, for the last {days} days. Days with nothing recorded
            stay in the chart as gaps.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {trend.error ? (
            <p role="alert" className="text-sm text-destructive">
              Could not read the activity history: {String(trend.error)}
            </p>
          ) : trend.isPending ? (
            <p className="text-sm text-muted-foreground">Reading the history…</p>
          ) : total === 0 ? (
            <p className="text-sm text-muted-foreground">
              Nothing has been recorded in this window yet. Starting a project, attaching a service or
              adding an application writes the first entries.
            </p>
          ) : (
            <div
              className="flex h-40 items-end gap-1"
              role="img"
              aria-label={`Recorded actions per day over ${days} days, ${total} in total`}
            >
              {(data?.days ?? []).map((point) => (
                <div key={point.day} className="group flex h-full min-w-0 flex-1 flex-col justify-end">
                  <div
                    className={cn(
                      "w-full rounded-t-sm",
                      point.count === 0 ? "bg-muted/60" : "bg-primary/70 group-hover:bg-primary",
                    )}
                    style={{ height: `${Math.max(2, (point.count / peak) * 100)}%` }}
                    title={`${point.day}: ${point.count} action${point.count === 1 ? "" : "s"}`}
                  />
                </div>
              ))}
            </div>
          )}
          {total > 0 && (
            <div className="mt-2 flex justify-between text-[0.7rem] text-muted-foreground">
              <span>{dayLabel(data?.days?.[0]?.day ?? "")}</span>
              <span>{dayLabel(data?.days?.[(data?.days?.length ?? 1) - 1]?.day ?? "")}</span>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">What happened</CardTitle>
            <CardDescription>
              The actions recorded in this window, most frequent first. Each count is one recorded
              entry, not a sample.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {(data?.actions?.length ?? 0) === 0 ? (
              <p className="text-sm text-muted-foreground">No actions recorded in this window.</p>
            ) : (
              <ul className="divide-y divide-border">
                {(data?.actions ?? []).map((action) => {
                  const share = total === 0 ? 0 : Math.round((action.count / total) * 100);
                  return (
                    <li key={action.action} className="flex items-center gap-3 py-2.5">
                      <span className="min-w-0 flex-1 truncate text-sm">{action.action}</span>
                      <span className="h-1.5 w-24 shrink-0 overflow-hidden rounded-full bg-muted">
                        <span
                          className="block h-full rounded-full bg-primary"
                          style={{ width: `${Math.max(4, share)}%` }}
                        />
                      </span>
                      <span className="w-14 shrink-0 text-right font-mono text-xs text-muted-foreground">
                        {action.count}
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Not recorded yet</CardTitle>
            <CardDescription>What this view will add as the remaining work lands.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-xs text-muted-foreground">
            <p>
              Resource trends (CPU, memory, disk over time) are not stored: the meters on the overview
              show the current reading only.
            </p>
            <p>
              Status history is not stored: a restarted or failed application appears here only as the
              project action that caused it.
            </p>
            <p>
              Retention applies to the action history above. Lowering it prunes immediately; raising it
              keeps future entries for longer.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
