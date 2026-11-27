import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Search } from "lucide-react";

import * as LogsService from "../../bindings/locafy.dev/desktop/internal/logs/service";
import type { Entry } from "../../bindings/locafy.dev/desktop/internal/logs/models";
import { cn } from "@/lib/utils";

const layers = [
  { value: "all", label: "All sources" },
  { value: "application", label: "Applications" },
  { value: "service", label: "Services" },
];

function time(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString([], { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function EntryIcon({ layer }: { layer: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "mt-1.5 size-1.5 shrink-0 rounded-full",
        layer === "service" ? "bg-primary" : "bg-success",
      )}
    />
  );
}

/*
 * One attributed stream of what the project's processes printed. The engine
 * captures the lines as they arrive, so this view survives a tab switch and
 * shows the same history in the project's tab and the global screen.
 */
export function LiveLogs({ projectID = "" }: { projectID?: string }) {
  const [layer, setLayer] = useState("all");
  const [source, setSource] = useState("all");
  const [contains, setContains] = useState("");

  const sourcesQuery = useQuery({
    queryKey: ["log-sources", projectID],
    queryFn: async () =>
      (await LogsService.Entries({
        projectId: projectID,
        layer: "",
        sourceId: "",
        contains: "",
        limit: 500,
      })) ?? [],
    refetchInterval: 5000,
  });

  const entriesQuery = useQuery({
    queryKey: ["logs", projectID, layer, source, contains],
    queryFn: async () =>
      (await LogsService.Entries({
        projectId: projectID,
        layer: layer === "all" ? "" : layer,
        sourceId: source === "all" ? "" : source,
        contains,
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

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-6 py-3">
        <div className="relative">
          <select
            aria-label="Filter by source kind"
            value={layer}
            onChange={(event) => {
              setLayer(event.target.value);
              setSource("all");
            }}
            className="h-8 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none"
          >
            {layers.map((option) => (
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
            className="h-8 max-w-52 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none"
          >
            <option value="all">Every source</option>
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

        <div className="relative min-w-48 flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            aria-label="Search log lines"
            value={contains}
            onChange={(event) => setContains(event.target.value)}
            placeholder="Search lines"
            className="h-8 w-full rounded-md border border-border bg-background pl-8 pr-2 text-xs focus:outline-none"
          />
        </div>

        <span className="ml-auto text-xs text-muted-foreground">
          {entries.length} line{entries.length === 1 ? "" : "s"}
          {masked > 0 ? ` · ${masked} with secrets masked` : ""}
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-3 font-mono text-xs">
        {entriesQuery.isPending ? (
          <p className="font-sans text-sm text-muted-foreground">Reading output…</p>
        ) : entriesQuery.error ? (
          <p role="alert" className="font-sans text-sm text-destructive">
            Could not read output: {String(entriesQuery.error)}
          </p>
        ) : entries.length === 0 ? (
          <p className="font-sans text-sm text-muted-foreground">
            {contains || layer !== "all" || source !== "all"
              ? "No captured line matches these filters."
              : "No output captured yet. Start an application or a service and its output appears here."}
          </p>
        ) : (
          <ul className="space-y-1">
            {entries.map((entry) => (
              <li key={entry.id} className="flex items-start gap-2.5">
                <span className="shrink-0 text-muted-foreground">{time(entry.time)}</span>
                <EntryIcon layer={entry.layer} />
                <span
                  className={cn(
                    "w-32 shrink-0 truncate",
                    entry.layer === "service" ? "text-primary" : "text-success",
                  )}
                  title={`${entry.layer === "service" ? "Service" : "Application"} · ${entry.source || entry.sourceId}`}
                >
                  {entry.source || entry.sourceId}
                </span>
                <span className="min-w-0 flex-1 whitespace-pre-wrap break-words">{entry.line}</span>
                {entry.masked && (
                  <span
                    className="shrink-0 rounded border border-border px-1 text-[0.65rem] uppercase tracking-wide text-muted-foreground"
                    title="A credential in this line was replaced before it was stored"
                  >
                    masked
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <p className="border-t border-border px-6 py-2 font-sans text-[0.7rem] text-muted-foreground">
        The engine keeps the most recent 2,000 lines per session. Lines arrive as the processes print
        them, and credentials are replaced before a line is stored.
      </p>
    </div>
  );
}
