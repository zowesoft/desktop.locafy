import { CheckCircle, Download, Layers, Pause, RotateCcw, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// ─── Mock data ────────────────────────────────────────────────────────────────

type DownloadStatus = "downloading" | "paused" | "completed" | "failed";

interface DownloadItem {
  id: string;
  name: string;
  kind: "runtime" | "service" | "update";
  version: string;
  size: string;
  downloaded?: string;
  progress?: number;
  status: DownloadStatus;
  error?: string;
  completedAt?: string;
}

const MOCK_DOWNLOADS: DownloadItem[] = [
  {
    id: "php-8.4.2",
    name: "PHP",
    kind: "runtime",
    version: "8.4.2",
    size: "42 MB",
    downloaded: "28 MB",
    progress: 66,
    status: "downloading",
  },
  {
    id: "locafy-0.9.3",
    name: "Locafy",
    kind: "update",
    version: "0.9.3",
    size: "18 MB",
    downloaded: "18 MB",
    progress: 100,
    status: "paused",
  },
  {
    id: "node-24.0.2",
    name: "Node.js",
    kind: "runtime",
    version: "24.0.2",
    size: "55 MB",
    downloaded: "55 MB",
    progress: 100,
    status: "completed",
    completedAt: "Today, 09:12",
  },
  {
    id: "mysql-8.0",
    name: "MySQL",
    kind: "service",
    version: "8.0",
    size: "280 MB",
    status: "failed",
    error: "Connection timed out — check your internet connection",
  },
  {
    id: "python-3.13.1",
    name: "Python",
    kind: "runtime",
    version: "3.13.1",
    size: "24 MB",
    downloaded: "24 MB",
    progress: 100,
    status: "completed",
    completedAt: "Yesterday, 14:55",
  },
];

function KindBadge({ kind }: { kind: DownloadItem["kind"] }) {
  if (kind === "runtime") {
    return (
      <Badge variant="secondary" className="bg-blue-500/10 text-blue-400 border-blue-500/20 text-xs">
        Runtime
      </Badge>
    );
  }
  if (kind === "service") {
    return (
      <Badge variant="secondary" className="bg-green-500/10 text-green-400 border-green-500/20 text-xs">
        Service
      </Badge>
    );
  }
  return (
    <Badge variant="secondary" className="bg-primary/10 text-primary border-primary/20 text-xs">
      Update
    </Badge>
  );
}

function ProgressBar({ progress, status }: { progress: number; status: DownloadStatus }) {
  return (
    <div className="h-1.5 overflow-hidden rounded-full bg-muted">
      <div
        className={cn(
          "h-full rounded-full transition-all",
          status === "completed"
            ? "bg-success"
            : status === "failed"
              ? "bg-destructive"
              : status === "paused"
                ? "bg-muted-foreground"
                : "bg-primary",
        )}
        style={{ width: `${progress}%` }}
      />
    </div>
  );
}

function DownloadRow({ item }: { item: DownloadItem }) {
  return (
    <Card>
      <CardContent className="pt-4">
        <div className="flex items-start gap-3">
          {/* Icon */}
          <div
            className={cn(
              "flex size-9 shrink-0 items-center justify-center rounded-lg",
              item.status === "completed"
                ? "bg-success/10 text-success"
                : item.status === "failed"
                  ? "bg-destructive/10 text-destructive"
                  : "bg-primary/10 text-primary",
            )}
          >
            {item.status === "completed" ? (
              <CheckCircle className="size-4" />
            ) : item.status === "failed" ? (
              <X className="size-4" />
            ) : (
              <Download className="size-4" />
            )}
          </div>

          {/* Info */}
          <div className="min-w-0 flex-1 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">
                {item.name} {item.version}
              </span>
              <KindBadge kind={item.kind} />
              {item.completedAt && (
                <span className="text-xs text-muted-foreground">{item.completedAt}</span>
              )}
            </div>

            {item.status === "failed" && item.error ? (
              <p className="text-xs text-destructive">{item.error}</p>
            ) : item.progress !== undefined ? (
              <div className="space-y-1">
                <ProgressBar progress={item.progress} status={item.status} />
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  {item.status === "downloading" ? (
                    <>
                      <span>
                        {item.downloaded} of {item.size}
                      </span>
                      <span>{item.progress}%</span>
                    </>
                  ) : item.status === "paused" ? (
                    <span>Paused — {item.progress}% complete</span>
                  ) : (
                    <span>
                      {item.size} · Download complete
                    </span>
                  )}
                </div>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">{item.size}</p>
            )}
          </div>

          {/* Actions */}
          <div className="flex shrink-0 gap-1">
            {item.status === "downloading" && (
              <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                <Pause className="size-3.5" />
              </Button>
            )}
            {item.status === "paused" && (
              <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                <Download className="size-3.5" />
              </Button>
            )}
            {item.status === "failed" && (
              <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                <RotateCcw className="size-3.5" />
                <span className="ml-1">Retry</span>
              </Button>
            )}
            {(item.status === "completed" || item.status === "failed") && (
              <Button size="sm" variant="ghost" className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground">
                <X className="size-3.5" />
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export function DownloadsScreen() {
  const active = MOCK_DOWNLOADS.filter(
    (d) => d.status === "downloading" || d.status === "paused",
  );
  const completed = MOCK_DOWNLOADS.filter((d) => d.status === "completed");
  const failed = MOCK_DOWNLOADS.filter((d) => d.status === "failed");

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-6 py-5">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Downloads</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Runtimes, services and updates being downloaded by Locafy
          </p>
        </div>
        {active.length > 0 && (
          <Button variant="outline" size="sm">
            <Pause className="mr-1.5 size-3.5" />
            Pause all
          </Button>
        )}
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {/* Empty state */}
        {MOCK_DOWNLOADS.length === 0 && (
          <div className="flex flex-col items-center justify-center gap-3 py-24">
            <div className="flex size-14 items-center justify-center rounded-full bg-muted">
              <Layers className="size-6 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium">No downloads</p>
            <p className="text-xs text-muted-foreground">
              Runtimes and services will appear here when Locafy is installing them.
            </p>
          </div>
        )}

        {/* Active */}
        {active.length > 0 && (
          <section className="space-y-2">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              In progress
            </h2>
            {active.map((d) => (
              <DownloadRow key={d.id} item={d} />
            ))}
          </section>
        )}

        {/* Failed */}
        {failed.length > 0 && (
          <section className="space-y-2">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Failed
              </h2>
              <Button variant="ghost" size="sm" className="h-6 text-xs text-primary">
                <RotateCcw className="mr-1 size-3" />
                Retry all
              </Button>
            </div>
            {failed.map((d) => (
              <DownloadRow key={d.id} item={d} />
            ))}
          </section>
        )}

        {/* Completed */}
        {completed.length > 0 && (
          <section className="space-y-2">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Completed
              </h2>
              <Button variant="ghost" size="sm" className="h-6 text-xs text-muted-foreground hover:text-foreground">
                Clear all
              </Button>
            </div>
            {completed.map((d) => (
              <DownloadRow key={d.id} item={d} />
            ))}
          </section>
        )}
      </div>
    </div>
  );
}
