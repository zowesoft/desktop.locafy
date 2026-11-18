import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ArrowLeft, FileText, FolderOpen, RefreshCw } from "lucide-react";

import * as ScaffoldService from "../../bindings/locafy.dev/desktop/internal/scaffold/service";
import type { JobSummary } from "../../bindings/locafy.dev/desktop/internal/scaffold/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

const historyKey = ["scaffold", "history"] as const;
const logKey = (jobID: string) => ["scaffold", "log", jobID] as const;

function statusTone(status: string): string {
  switch (status) {
    case "succeeded":
      return "text-success";
    case "failed":
      return "text-destructive";
    case "running":
      return "text-primary";
    case "cancelled":
      return "text-accent";
    default:
      return "text-muted-foreground";
  }
}

function formatWhen(iso: string): string {
  const created = Date.parse(iso);
  if (Number.isNaN(created)) return iso;
  return new Date(created).toLocaleString();
}

/*
 * Past scaffold runs, newest first. Jobs are recorded whether or not they
 * registered a project, so a failed install can be inspected here instead of
 * hunting for a log file.
 */
export function ScaffoldHistoryDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [viewing, setViewing] = useState<JobSummary | null>(null);

  const history = useQuery({
    queryKey: historyKey,
    queryFn: async () => (await ScaffoldService.History(50)) ?? [],
    enabled: open,
  });

  const close = () => {
    setViewing(null);
    onClose();
  };

  const jobs = history.data ?? [];

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) close();
      }}
    >
      <DialogContent className="sm:max-w-2xl">
        {viewing ? (
          <ScaffoldLog job={viewing} onBack={() => setViewing(null)} onClose={close} />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Scaffold history</DialogTitle>
              <DialogDescription>
                Every framework install Locafy has run, including ones that failed before a project
                was registered. The newest run is first.
              </DialogDescription>
            </DialogHeader>

            <div className="mt-4 max-h-[55vh] space-y-2 overflow-y-auto pr-1">
              {history.isPending ? (
                <p className="text-sm text-muted-foreground">Reading the history…</p>
              ) : history.error ? (
                <p role="alert" className="text-sm text-destructive">
                  Could not read the history: {String(history.error)}
                </p>
              ) : jobs.length === 0 ? (
                <p className="rounded-lg border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
                  No framework install has run yet.
                </p>
              ) : (
                jobs.map((job) => (
                  <div key={job.ID} className="rounded-lg border border-border px-3 py-2.5 text-sm">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{job.RecipeLabel}</span>
                      <Badge variant="outline" className={cn("capitalize", statusTone(job.Status))}>
                        {job.Status}
                      </Badge>
                      <span className="ml-auto text-xs text-muted-foreground">
                        {formatWhen(job.CreatedAt)}
                      </span>
                    </div>
                    <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
                      {job.TargetPath}
                    </p>
                    {job.Error && (
                      <p className="mt-1 text-xs text-destructive">{job.Error}</p>
                    )}
                    <div className="mt-2 flex flex-wrap gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        aria-label={`View the log of the ${job.RecipeLabel} run`}
                        onClick={() => setViewing(job)}
                      >
                        <FileText /> View log
                      </Button>
                      {job.LogAvailable && (
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-label={`Open the log file of the ${job.RecipeLabel} run`}
                          onClick={() => void ScaffoldService.OpenLog(job.ID)}
                        >
                          <FolderOpen /> Open log file
                        </Button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>

            <DialogFooter className="mt-5">
              <Button
                type="button"
                variant="outline"
                disabled={history.isFetching}
                onClick={() => void history.refetch()}
              >
                <RefreshCw className={cn("size-3.5", history.isFetching && "animate-spin")} />
                Refresh
              </Button>
              <Button type="button" onClick={close}>
                Close
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function ScaffoldLog({
  job,
  onBack,
  onClose,
}: {
  job: JobSummary;
  onBack: () => void;
  onClose: () => void;
}) {
  const log = useQuery({
    queryKey: logKey(job.ID),
    queryFn: () => ScaffoldService.JobLog(job.ID),
  });
  const openFile = useMutation({ mutationFn: () => ScaffoldService.OpenLog(job.ID) });

  return (
    <>
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <ArrowLeft
            role="button"
            tabIndex={0}
            aria-label="Back to the history"
            className="size-4 cursor-pointer text-muted-foreground hover:text-foreground"
            onClick={onBack}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") onBack();
            }}
          />
          {job.RecipeLabel} log
        </DialogTitle>
        <DialogDescription>
          <span className={cn("capitalize", statusTone(job.Status))}>{job.Status}</span> ·{" "}
          <span className="break-all font-mono">{job.TargetPath}</span>
        </DialogDescription>
      </DialogHeader>

      <div className="mt-4">
        {log.isPending ? (
          <p className="text-sm text-muted-foreground">Reading the saved output…</p>
        ) : log.error ? (
          <p role="alert" className="text-sm text-destructive">
            {String(log.error)}
          </p>
        ) : (
          <pre className="max-h-[50vh] overflow-auto whitespace-pre-wrap break-all rounded-lg border border-border bg-muted/30 p-3 font-mono text-xs leading-relaxed">
            {log.data}
          </pre>
        )}
        {job.Error && (
          <p className="mt-2 text-xs text-destructive">Failed: {job.Error}</p>
        )}
        {openFile.error && (
          <p role="alert" className="mt-2 text-xs text-destructive">
            Could not open the log file: {String(openFile.error)}
          </p>
        )}
      </div>

      <DialogFooter className="mt-5">
        <Button type="button" variant="outline" onClick={onBack}>
          Back
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={!job.LogAvailable || openFile.isPending}
          onClick={() => openFile.mutate()}
        >
          <FolderOpen /> Open log file
        </Button>
        <Button type="button" onClick={onClose}>
          Close
        </Button>
      </DialogFooter>
    </>
  );
}
