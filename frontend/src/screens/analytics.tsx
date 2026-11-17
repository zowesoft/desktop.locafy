import { Activity, Clock, Cpu, HardDrive, Info } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// ─── Mock data ────────────────────────────────────────────────────────────────

const ACTIVITY_ITEMS = [
  { time: "16:47", event: "Locafy engine started", project: null, kind: "system" as const },
  { time: "16:47", event: "PostgreSQL 17 started (shared)", project: null, kind: "service" as const },
  { time: "16:47", event: "Redis 7.4 started (shared)", project: null, kind: "service" as const },
  { time: "16:47", event: "Etugora project resumed", project: "Etugora", kind: "project" as const },
  { time: "16:48", event: "Etugora API started on :8000", project: "Etugora", kind: "app" as const },
  { time: "16:48", event: "Etugora Auth started on :3001", project: "Etugora", kind: "app" as const },
  { time: "16:48", event: "ML Service started on :5000", project: "Etugora", kind: "app" as const },
  { time: "16:49", event: "PHP 8.4.2 download started", project: null, kind: "system" as const },
  { time: "16:51", event: "Folio project opened", project: "Folio", kind: "project" as const },
];

const CPU_SPARKLINE = [12, 18, 24, 20, 32, 28, 24, 30, 26, 24];
const MEM_SPARKLINE = [2.1, 2.2, 2.4, 2.5, 2.7, 2.8, 2.8, 2.8, 2.8, 2.8];
const DISK_SPARKLINE = [18.0, 18.1, 18.2, 18.3, 18.4, 18.5, 18.5, 18.5, 18.6, 18.6];

function Sparkline({
  data,
  color,
  unit,
}: {
  data: number[];
  color: string;
  unit?: string;
}) {
  const max = Math.max(...data);
  const min = Math.min(...data) * 0.9;
  const range = max - min || 1;
  const height = 40;
  const width = 120;
  const step = width / (data.length - 1);

  const points = data
    .map((v, i) => {
      const x = i * step;
      const y = height - ((v - min) / range) * height;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <svg width={width} height={height} className="shrink-0">
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
  sparkline: number[];
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
          <Sparkline data={sparkline} color={color} />
        </div>
      </CardContent>
    </Card>
  );
}

function ActivityDot({ kind }: { kind: "system" | "service" | "project" | "app" }) {
  const styles = {
    system: "bg-muted-foreground",
    service: "bg-blue-400",
    project: "bg-primary",
    app: "bg-success",
  };
  return <span className={cn("size-2 shrink-0 rounded-full mt-1.5", styles[kind])} />;
}

export function AnalyticsScreen() {
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
          <Badge variant="secondary" className="font-normal">
            <Clock className="mr-1 size-3" />
            Updated 30 s ago
          </Badge>
        </div>
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {/* Resource cards */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Resource usage — Locafy-managed processes
          </h2>
          <div className="grid gap-3 sm:grid-cols-3">
            <ResourceCard
              label="CPU"
              current="24"
              unit="%"
              icon={Cpu}
              color="text-primary"
              sparkline={CPU_SPARKLINE}
              scope="Locafy-managed"
            />
            <ResourceCard
              label="Memory"
              current="2.8"
              max="8"
              unit="GB"
              icon={Activity}
              color="text-blue-400"
              sparkline={MEM_SPARKLINE}
              scope="Locafy-managed"
            />
            <ResourceCard
              label="Disk I/O"
              current="18.6"
              max="100"
              unit="GB"
              icon={HardDrive}
              color="text-amber-400"
              sparkline={DISK_SPARKLINE}
              scope="Locafy data only"
            />
          </div>
        </section>

        {/* Status history */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Project status history — today
          </h2>
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-muted/40">
                <tr>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Project
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Status
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Apps
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    Uptime
                  </th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                    CPU avg
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {[
                  { name: "Etugora", status: "running", apps: 3, uptime: "4h 12m", cpu: "18%" },
                  { name: "Folio", status: "stopped", apps: 1, uptime: "—", cpu: "—" },
                ].map((row) => (
                  <tr key={row.name}>
                    <td className="px-4 py-2.5 font-medium">{row.name}</td>
                    <td className="px-4 py-2.5">
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 text-xs",
                          row.status === "running" ? "text-success" : "text-muted-foreground",
                        )}
                      >
                        <span
                          className={cn(
                            "size-1.5 rounded-full",
                            row.status === "running" ? "bg-success" : "bg-muted-foreground/40",
                          )}
                        />
                        {row.status === "running" ? "Running" : "Stopped"}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-muted-foreground">{row.apps}</td>
                    <td className="px-4 py-2.5 font-mono text-xs">{row.uptime}</td>
                    <td className="px-4 py-2.5 font-mono text-xs">{row.cpu}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Activity timeline */}
        <section className="space-y-2">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Activity timeline — today
          </h2>
          <div className="space-y-0 overflow-hidden rounded-lg border border-border divide-y divide-border">
            {ACTIVITY_ITEMS.map((item, i) => (
              <div key={i} className="flex items-start gap-3 px-4 py-2.5">
                <span className="mt-0.5 w-10 shrink-0 font-mono text-xs text-muted-foreground/70">
                  {item.time}
                </span>
                <ActivityDot kind={item.kind} />
                <span className="flex-1 text-xs">{item.event}</span>
                {item.project && (
                  <Badge variant="outline" className="shrink-0 text-xs font-normal">
                    {item.project}
                  </Badge>
                )}
              </div>
            ))}
          </div>
        </section>

        {/* Local-only note */}
        <div className="flex items-center gap-2 rounded-lg border border-border p-3 text-xs text-muted-foreground">
          <Info className="size-3.5 shrink-0" />
          All analytics data is stored locally on this machine and is never sent to Locafy servers.
          Data is kept for 30 days and then removed automatically.
        </div>
      </div>
    </div>
  );
}
