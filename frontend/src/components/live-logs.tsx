import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Search, X } from "lucide-react";

import * as LogsService from "../../bindings/locafy.dev/desktop/internal/logs/service";
import type { Entry } from "../../bindings/locafy.dev/desktop/internal/logs/models";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type Severity = "error" | "warn" | "info" | "debug";

const SEVERITIES: { value: Severity | "all"; label: string }[] = [
  { value: "all", label: "All" },
  { value: "error", label: "Error" },
  { value: "warn", label: "Warn" },
  { value: "info", label: "Info" },
  { value: "debug", label: "Debug" },
];

const LAYERS = [
  { value: "all", label: "All sources" },
  { value: "application", label: "Applications" },
  { value: "service", label: "Services" },
];

const SOURCE_COLOURS = [
  "text-emerald-400",
  "text-sky-400",
  "text-amber-400",
  "text-cyan-400",
  "text-fuchsia-400",
  "text-lime-400",
  "text-rose-400",
  "text-primary",
];

function sourceColour(name: string): string {
  let hash = 0;
  for (let index = 0; index < name.length; index += 1) {
    hash = (hash * 31 + name.charCodeAt(index)) % 997;
  }
  return SOURCE_COLOURS[hash % SOURCE_COLOURS.length];
}

// The row shows a readable local time; the export keeps the exact instant.
function timestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (part: number) => String(part).padStart(2, "0");
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  );
}

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
        "inline-flex shrink-0 items-center rounded border px-1.5 py-0.5 text-xs font-medium",
        styles[severity] ?? styles.info,
      )}
    >
      {severity}
    </span>
  );
}

function SourceTag({ source }: { source: string }) {
  return (
    <span className={cn("min-w-[96px] shrink-0 truncate font-mono text-xs", sourceColour(source))}>
      {source}
    </span>
  );
}

/*
 * One attributed stream of what Locafy's processes printed: the same rows the
 * log surfaces always showed, now read from the engine's capture rather than
 * sample data. The engine assigns the severity from the line's wording, since a
 * supervised process prints plain text.
 */
export function LiveLogs({ projectID = "" }: { projectID?: string }) {
  const [layer, setLayer] = useState("all");
  const [source, setSource] = useState("all");
  const [severity, setSeverity] = useState<Severity | "all">("all");
  const [search, setSearch] = useState("");

  const sourcesQuery = useQuery({
    queryKey: ["log-sources", projectID],
    queryFn: async () =>
      (await LogsService.Entries({
        projectId: projectID,
        layer: "",
        sourceId: "",
        level: "",
        contains: "",
        limit: 500,
      })) ?? [],
    refetchInterval: 5000,
  });

  const entriesQuery = useQuery({
    queryKey: ["logs", projectID, layer, source, severity, search],
    queryFn: async () =>
      (await LogsService.Entries({
        projectId: projectID,
        layer: layer === "all" ? "" : layer,
        sourceId: source === "all" ? "" : source,
        level: severity === "all" ? "" : severity,
        contains: search,
        limit: 300,
      })) ?? [],
    refetchInterval: 1000,
  });

  const sources = useMemo(() => {
    const seen = new Map<string, { id: string; name: string; layer: string }>();
    for (const entry of sourcesQuery.data ?? []) {
      const key = `${entry.layer}/${entry.sourceId}`;
      if (!seen.has(key)) {
        seen.set(key, { id: entry.sourceId, name: entry.source || entry.sourceId, layer: entry.layer });
      }
    }
    return [...seen.values()];
  }, [sourcesQuery.data]);

  const entries: Entry[] = entriesQuery.data ?? [];
  const masked = entries.filter((entry) => entry.masked).length;

  const exportLogs = () => {
    const body = entries
      .map((entry) => `${entry.time} ${entry.level.toUpperCase().padEnd(5)} ${entry.source || entry.sourceId} ${entry.line}`)
      .join("\n");
    const blob = new Blob([body.endsWith("\n") ? body : `${body}\n`], { type: "text/plain" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `locafy-logs-${new Date().toISOString().replace(/[:.]/g, "-")}.log`;
    link.click();
    URL.revokeObjectURL(link.href);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="px-6 pt-4">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <select
              aria-label="Filter by source kind"
              value={layer}
              onChange={(event) => {
                setLayer(event.target.value);
                setSource("all");
              }}
              className="h-8 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
            >
              {LAYERS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <ChevronDown className="pointer-events-none absolute right-2 top-1/2 size-3 -translate-y-1/2 text-muted-foreground" />
          </div>

          <div className="relative">
            <select
              aria-label="Filter by source"
              value={source}
              onChange={(event) => setSource(event.target.value)}
              className="h-8 max-w-52 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="all">All sources</option>
              {sources
                .filter((option) => layer === "all" || option.layer === layer)
                .map((option) => (
                  <option key={`${option.layer}/${option.id}`} value={option.id}>
                    {option.name}
                  </option>
                ))}
            </select>
            <ChevronDown className="pointer-events-none absolute right-2 top-1/2 size-3 -translate-y-1/2 text-muted-foreground" />
          </div>

          <div className="flex rounded-md border border-border bg-muted/40 p-0.5">
            {SEVERITIES.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => setSeverity(option.value)}
                className={cn(
                  "rounded px-2.5 py-1 text-xs transition-colors",
                  severity === option.value
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {option.label}
              </button>
            ))}
          </div>

          <div className="relative min-w-[200px] flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              aria-label="Search log lines"
              className="h-8 pl-8 text-xs"
              placeholder="Search log messages…"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            {search && (
              <button
                type="button"
                aria-label="Clear the search"
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                onClick={() => setSearch("")}
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>

          <span className="text-xs text-muted-foreground">
            {entries.length} {entries.length === 1 ? "entry" : "entries"}
            {masked > 0 ? ` · ${masked} masked` : ""}
          </span>

          <Button variant="outline" size="sm" disabled={entries.length === 0} onClick={exportLogs}>
            Export logs
          </Button>
        </div>
      </div>

      <div className="mt-3 flex-1 overflow-y-auto font-mono">
        {entriesQuery.isPending ? (
          <div className="flex items-center justify-center py-20">
            <p className="font-sans text-sm text-muted-foreground">Reading output…</p>
          </div>
        ) : entriesQuery.error ? (
          <div className="flex items-center justify-center py-20">
            <p role="alert" className="font-sans text-sm text-destructive">
              Could not read output: {String(entriesQuery.error)}
            </p>
          </div>
        ) : entries.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-1 py-20">
            <p className="font-sans text-sm text-muted-foreground">
              {search || severity !== "all" || layer !== "all" || source !== "all"
                ? "No log entries match your filters."
                : "No output captured yet."}
            </p>
            {!search && severity === "all" && layer === "all" && source === "all" && (
              <p className="font-sans text-xs text-muted-foreground/70">
                Start an application or a service and what it prints appears here.
              </p>
            )}
          </div>
        ) : (
          <div className="divide-y divide-border/50">
            {entries.map((entry) => (
              <div
                key={entry.id}
                className={cn(
                  "flex items-start gap-3 px-6 py-2 text-xs hover:bg-muted/30",
                  entry.level === "error" && "bg-destructive/5",
                  entry.level === "warn" && "bg-amber-500/5",
                )}
              >
                <span className="shrink-0 tabular-nums text-muted-foreground/60">
                  {timestamp(entry.time)}
                </span>
                <SourceTag source={entry.source || entry.sourceId} />
                <SeverityBadge severity={entry.level as Severity} />
                <span
                  className={cn(
                    "min-w-0 flex-1 break-all",
                    entry.level === "error"
                      ? "text-destructive"
                      : entry.level === "debug"
                        ? "text-muted-foreground/70"
                        : "text-foreground",
                  )}
                >
                  {entry.line}
                  {entry.masked && (
                    <span
                      className="ml-2 rounded border border-border px-1 align-middle text-[0.65rem] uppercase tracking-wide text-muted-foreground"
                      title="A credential in this line was replaced before it was stored"
                    >
                      masked
                    </span>
                  )}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <p className="border-t border-border px-6 py-2 font-sans text-[0.7rem] text-muted-foreground">
        The engine keeps the most recent 2,000 lines per session. Severity is read from each line's
        wording, and credentials are replaced before a line is stored.
      </p>
    </div>
  );
}
