import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { useEffect } from "react";

import * as DownloadsService from "../../bindings/locafy.dev/desktop/internal/downloads/service";
import type { Installable, Job } from "../../bindings/locafy.dev/desktop/internal/downloads/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const jobsKey = ["downloads"] as const;

/*
 * The machine's install jobs. The engine emits downloads:jobs for every row
 * change, so a running download's progress arrives instead of being polled.
 */
export function useInstallJobs() {
  const client = useQueryClient();
  useEffect(() => {
    const off = Events.On("downloads:jobs", () => {
      client.invalidateQueries({ queryKey: jobsKey });
    });
    return () => off();
    // The client is stable.
  }, [client]);

  return useQuery({
    queryKey: jobsKey,
    queryFn: async () => (await DownloadsService.List()) ?? [],
  });
}

/* The catalogue is static data: what this machine can install does not change
 * while the window is open. */
function useInstallables() {
  return useQuery({
    queryKey: ["downloads", "installables"] as const,
    queryFn: async () => (await DownloadsService.Installables()) ?? [],
    staleTime: Infinity,
  });
}

export function DownloadsScreen() {
  const jobs = useInstallJobs();
  const installables = useInstallables();
  const client = useQueryClient();
  const refresh = () => client.invalidateQueries({ queryKey: jobsKey });
  const entries = jobs.data ?? [];
  const running = entries.filter((job) => job.state === "running" || job.state === "queued");
  const finished = entries.filter((job) => job.state !== "running" && job.state !== "queued");

  const install = useMutation({
    mutationFn: (kindID: string) => DownloadsService.Install(kindID),
    onSuccess: refresh,
  });
  const retry = useMutation({
    mutationFn: (id: string) => DownloadsService.Retry(id),
    onSuccess: refresh,
  });
  const cancel = useMutation({
    mutationFn: (id: string) => DownloadsService.Cancel(id),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => DownloadsService.Remove(id),
    onSuccess: refresh,
  });

  return (
    <div className="flex min-h-full flex-col">
      <div className="border-b border-border px-6 py-5">
        <h1 className="text-lg font-semibold tracking-tight">Downloads</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Runtimes and components Locafy installs itself — machine-scoped, no account
        </p>
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {jobs.isPending ? (
          <p className="text-sm text-muted-foreground">Reading the install jobs…</p>
        ) : jobs.isError ? (
          <p role="alert" className="text-sm text-destructive">
            Could not read the install jobs: {String(jobs.error)}
          </p>
        ) : (
          <>
            <section>
              <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Available to install
              </h2>
              <div className="grid gap-3 sm:grid-cols-2">
                {(installables.data ?? []).map((installable) => (
                  <InstallCard
                    key={installable.kind}
                    installable={installable}
                    installed={entries.some(
                      (job) => job.kindId === installable.kind && job.state === "succeeded",
                    )}
                    onInstall={() => install.mutate(installable.kind)}
                    busy={install.isPending}
                  />
                ))}
                {!(installables.data ?? []).length && (
                  <p className="text-sm text-muted-foreground">
                    The catalogue has nothing for this machine yet.
                  </p>
                )}
              </div>
              {install.error && (
                <p role="alert" className="mt-2 text-xs text-destructive">
                  {String(install.error)}
                </p>
              )}
            </section>

            {running.length > 0 && (
              <section>
                <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  In progress
                </h2>
                <ul className="space-y-3">
                  {running.map((job) => (
                    <li key={job.id}>
                      <JobRow job={job}>
                        <Button variant="outline" size="sm" onClick={() => cancel.mutate(job.id)}>
                          Cancel
                        </Button>
                      </JobRow>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {finished.length > 0 && (
              <section>
                <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  History
                </h2>
                <ul className="space-y-3">
                  {finished.map((job) => (
                    <li key={job.id}>
                      <JobRow job={job}>
                        {(job.state === "failed" || job.state === "cancelled") && (
                          <Button variant="outline" size="sm" onClick={() => retry.mutate(job.id)}>
                            Retry
                          </Button>
                        )}
                        <Button variant="ghost" size="sm" onClick={() => remove.mutate(job.id)}>
                          Remove
                        </Button>
                      </JobRow>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {entries.length === 0 && (
              <p className="rounded-xl border border-dashed border-border px-6 py-10 text-center text-sm text-muted-foreground">
                Nothing has been installed yet. Install a runtime above and it appears in the
                Runtimes screen as Locafy-managed.
              </p>
            )}
          </>
        )}
      </div>
    </div>
  );
}

/*
 * The catalogue is read from the engine; no local cache beyond the query.
 */

function InstallCard({
  installable,
  installed,
  onInstall,
  busy,
}: {
  installable: Installable;
  installed: boolean;
  onInstall: () => void;
  busy: boolean;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm">{installable.label}</CardTitle>
          <Badge variant={installed ? "secondary" : "outline"}>
            {installed ? "Installed" : "Not installed"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <p className="mb-3 truncate font-mono text-[11px] text-muted-foreground" title={installable.spec.url}>
          {installable.spec.url}
        </p>
        <Button size="sm" disabled={busy || installed} onClick={onInstall}>
          {installed ? "Installed" : `Install ${installable.version}`}
        </Button>
      </CardContent>
    </Card>
  );
}

function stateBadge(state: string) {
  if (state === "succeeded") return <Badge variant="secondary">Installed</Badge>;
  if (state === "failed") return <Badge variant="destructive">Failed</Badge>;
  if (state === "cancelled") return <Badge variant="outline">Cancelled</Badge>;
  return <Badge variant="outline">Queued</Badge>;
}

function JobRow({ job, children }: { job: Job; children: React.ReactNode }) {
  const installing = job.state === "running";
  const fraction = job.total > 0 ? Math.min(1, job.progress / job.total) : 0;
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium">{job.label}</span>
        {installing ? (
          <span className="inline-flex items-center gap-1.5 text-xs text-primary">
            <span className="size-1.5 animate-pulse rounded-full bg-primary" />
            Installing…
          </span>
        ) : (
          stateBadge(job.state)
        )}
        <span className="ml-auto flex gap-2">{children}</span>
      </div>
      {installing && (
        <div className="mt-2 space-y-1">
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${Math.round(fraction * 100)}%` }} />
          </div>
          <p className="text-[11px] text-muted-foreground">
            {formatBytes(job.progress)} of {job.total > 0 ? formatBytes(job.total) : "unknown size"}
          </p>
        </div>
      )}
      {job.error && (
        <p role="alert" className="mt-2 text-xs text-destructive">
          {job.error}
        </p>
      )}
      {job.state === "succeeded" && job.targetDir && (
        <p className="mt-1 truncate font-mono text-[11px] text-muted-foreground" title={job.targetDir}>
          {job.targetDir}
        </p>
      )}
    </div>
  );
}

function formatBytes(value: number): string {
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
