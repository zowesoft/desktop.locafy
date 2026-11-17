import { useState } from "react";
import { ChevronDown, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

// ─── Types ────────────────────────────────────────────────────────────────────

type Severity = "error" | "warn" | "info" | "debug";
type LogSource =
  | "locafy"
  | "etugora-api"
  | "etugora-auth"
  | "postgres"
  | "redis"
  | "mailpit"
  | "ml-service"
  | "folio-web";

interface LogEntry {
  id: string;
  timestamp: string;
  source: LogSource;
  severity: Severity;
  message: string;
}

// ─── Mock log entries ─────────────────────────────────────────────────────────

const MOCK_LOGS: LogEntry[] = [
  { id: "1", timestamp: "16:47:03.218", source: "locafy", severity: "info", message: "Engine started — managing 3 projects, 4 services" },
  { id: "2", timestamp: "16:47:03.561", source: "postgres", severity: "info", message: "shared PostgreSQL 17.3 listening on 127.0.0.1:5432" },
  { id: "3", timestamp: "16:47:03.802", source: "redis", severity: "info", message: "Redis 7.4.1 ready to accept connections" },
  { id: "4", timestamp: "16:47:04.011", source: "mailpit", severity: "info", message: "SMTP server listening on 127.0.0.1:2525" },
  { id: "5", timestamp: "16:47:04.213", source: "etugora-api", severity: "info", message: "Laravel application started — serving on http://api.etugora.test" },
  { id: "6", timestamp: "16:47:05.001", source: "etugora-auth", severity: "info", message: "NestJS server started on port 3001" },
  { id: "7", timestamp: "16:47:06.337", source: "ml-service", severity: "warn", message: "CUDA not available — falling back to CPU inference (slower)" },
  { id: "8", timestamp: "16:47:08.892", source: "etugora-api", severity: "error", message: "DB_PASSWORD env var is not set — using empty password (not recommended for production)" },
  { id: "9", timestamp: "16:47:09.100", source: "postgres", severity: "warn", message: "max_connections (100) approaching limit — 87 currently open" },
  { id: "10", timestamp: "16:47:11.500", source: "locafy", severity: "debug", message: "Project health check: etugora OK, folio STOPPED" },
  { id: "11", timestamp: "16:47:14.012", source: "etugora-api", severity: "info", message: "GET /api/v1/users 200 42ms" },
  { id: "12", timestamp: "16:47:14.880", source: "etugora-api", severity: "info", message: "POST /api/v1/auth/login 200 88ms" },
  { id: "13", timestamp: "16:47:15.233", source: "ml-service", severity: "error", message: "RuntimeError: ONNX model weights not found at models/sentiment-v2.onnx" },
  { id: "14", timestamp: "16:47:16.009", source: "redis", severity: "debug", message: "AOF rewrite triggered — 2.1MB flushed to disk" },
  { id: "15", timestamp: "16:47:18.441", source: "etugora-api", severity: "warn", message: "Slow query detected (>500ms): SELECT * FROM audit_logs WHERE project_id = 42" },
  { id: "16", timestamp: "16:47:20.000", source: "locafy", severity: "info", message: "Disk usage: Locafy runtime data 5.6 GB (threshold: 10 GB)" },
];

const ALL_SOURCES: LogSource[] = [
  "locafy",
  "etugora-api",
  "etugora-auth",
  "postgres",
  "redis",
  "mailpit",
  "ml-service",
  "folio-web",
];

const SEVERITIES: { value: Severity | "all"; label: string }[] = [
  { value: "all", label: "All" },
  { value: "error", label: "Error" },
  { value: "warn", label: "Warn" },
  { value: "info", label: "Info" },
  { value: "debug", label: "Debug" },
];

// ─── Components ───────────────────────────────────────────────────────────────

function SeverityBadge({ severity }: { severity: Severity }) {
  const styles: Record<Severity, string> = {
    error: "bg-destructive/10 text-destructive border-destructive/20",
    warn: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    info: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    debug: "bg-muted text-muted-foreground border-transparent",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium border",
        styles[severity],
      )}
    >
      {severity}
    </span>
  );
}

function SourceTag({ source }: { source: LogSource }) {
  const colors: Record<LogSource, string> = {
    locafy: "text-primary",
    "etugora-api": "text-green-400",
    "etugora-auth": "text-green-300",
    postgres: "text-blue-400",
    redis: "text-red-400",
    mailpit: "text-amber-400",
    "ml-service": "text-yellow-400",
    "folio-web": "text-cyan-400",
  };
  return (
    <span className={cn("min-w-[96px] font-mono text-xs", colors[source])}>
      {source}
    </span>
  );
}

export function LogsScreen() {
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<LogSource | "all">("all");
  const [severityFilter, setSeverityFilter] = useState<Severity | "all">("all");

  const filtered = MOCK_LOGS.filter((entry) => {
    if (sourceFilter !== "all" && entry.source !== sourceFilter) return false;
    if (severityFilter !== "all" && entry.severity !== severityFilter) return false;
    if (search && !entry.message.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="border-b border-border px-6 py-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-lg font-semibold tracking-tight">Logs</h1>
            <p className="mt-0.5 text-xs text-muted-foreground">
              Machine-wide log stream from all Locafy-managed processes · Project-specific logs are
              in each project's Logs tab
            </p>
          </div>
          <Button variant="outline" size="sm">
            Export logs
          </Button>
        </div>

        {/* Filter bar */}
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {/* Source selector */}
          <div className="relative">
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value as LogSource | "all")}
              className="h-8 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="all">All sources</option>
              {ALL_SOURCES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            <ChevronDown className="pointer-events-none absolute right-2 top-1/2 size-3 -translate-y-1/2 text-muted-foreground" />
          </div>

          {/* Severity filter */}
          <div className="flex rounded-md border border-border bg-muted/40 p-0.5">
            {SEVERITIES.map((s) => (
              <button
                key={s.value}
                type="button"
                onClick={() => setSeverityFilter(s.value as Severity | "all")}
                className={cn(
                  "rounded px-2.5 py-1 text-xs transition-colors",
                  severityFilter === s.value
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {s.label}
              </button>
            ))}
          </div>

          {/* Search */}
          <div className="relative flex-1 min-w-[200px]">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 pl-8 text-xs"
              placeholder="Search log messages…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {search && (
              <button
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                onClick={() => setSearch("")}
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>

          <span className="text-xs text-muted-foreground">
            {filtered.length} of {MOCK_LOGS.length} entries
          </span>
        </div>
      </div>

      {/* Log lines */}
      <div className="flex-1 overflow-y-auto font-mono">
        {filtered.length === 0 ? (
          <div className="flex items-center justify-center py-20">
            <p className="text-sm text-muted-foreground">No log entries match your filters.</p>
          </div>
        ) : (
          <div className="divide-y divide-border/50">
            {filtered.map((entry) => (
              <div
                key={entry.id}
                className={cn(
                  "flex items-start gap-3 px-6 py-2 text-xs hover:bg-muted/30",
                  entry.severity === "error" && "bg-destructive/5",
                  entry.severity === "warn" && "bg-amber-500/5",
                )}
              >
                <span className="shrink-0 text-muted-foreground/60 tabular-nums">
                  {entry.timestamp}
                </span>
                <SourceTag source={entry.source} />
                <SeverityBadge severity={entry.severity} />
                <span
                  className={cn(
                    "flex-1 break-all",
                    entry.severity === "error"
                      ? "text-destructive"
                      : entry.severity === "debug"
                        ? "text-muted-foreground/70"
                        : "text-foreground",
                  )}
                >
                  {entry.message}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
