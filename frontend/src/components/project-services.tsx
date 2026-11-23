import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import {
  AlertTriangle,
  ChevronDown,
  Copy,
  Database,
  Play,
  Plus,
  RotateCw,
  Square,
  Trash2,
} from "lucide-react";

import * as ServicesService from "../../bindings/locafy.dev/desktop/internal/services/service";
import type {
  Instance,
  Template,
} from "../../bindings/locafy.dev/desktop/internal/services/models";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { cn } from "@/lib/utils";

export const servicesKey = (projectID: string) => ["services", projectID] as const;
export const serviceCatalogueKey = ["services", "catalogue"] as const;
/** A service's recent output: live while it runs, the last run's words after it stops. */
export const serviceOutputKey = (serviceID: string) => ["service-output", serviceID] as const;

/** One connection detail with a copy action, since these get pasted a lot. */
function CopyField({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="space-y-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="flex items-center gap-1">
        <p className="font-mono text-xs break-all">{value}</p>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-5 px-1"
          aria-label={`Copy ${label}`}
          onClick={() => void copy()}
        >
          <Copy className="size-3" />
        </Button>
        {copied && <span className="text-[10px] text-success">Copied</span>}
      </div>
    </div>
  );
}

function stateWord(service: Instance): string {
  switch (service.Status) {
    case "running":
      return "Running";
    case "starting":
      return "Starting";
    case "stopping":
      return "Stopping";
    case "error":
      return "Error";
    default:
      return "Stopped";
  }
}

function healthWord(service: Instance): string {
  if (service.Status !== "running") return "";
  if (service.Health === "healthy") return "Answering";
  if (service.Health === "unhealthy") return "Not answering";
  return "Checking";
}

function ServiceRow({
  service,
  busy,
  onStart,
  onStop,
  onRestart,
  onRemove,
}: {
  service: Instance;
  busy: boolean;
  onStart: () => void;
  onStop: () => void;
  onRestart: () => void;
  onRemove: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const running = service.Status === "running" || service.Status === "starting";

  // Read only while the row is open, so a project full of services does not
  // poll transcripts nobody is looking at.
  const output = useQuery({
    queryKey: serviceOutputKey(service.ID),
    queryFn: async () => (await ServicesService.Output(service.ID)) ?? [],
    enabled: expanded,
    refetchInterval: expanded && running ? 2000 : false,
  });
  const lines = output.data ?? [];

  return (
    <>
      <tr className="hover:bg-muted/30">
        <td className="px-4 py-2.5">
          <div className="flex items-center gap-2">
            <span className="font-medium">{service.Name}</span>
            <Badge variant="outline" className="text-[10px] text-muted-foreground">
              {service.Engine === "postgresql" ? "PostgreSQL" : service.Engine} {service.Version}
            </Badge>
          </div>
          {service.Error && (
            <p className="mt-1 text-xs text-destructive">{service.Error}</p>
          )}
        </td>
        <td className="px-4 py-2.5 font-mono text-xs">127.0.0.1:{service.Port}</td>
        <td className="px-4 py-2.5">
          <span
            className={cn(
              "inline-flex items-center gap-1.5 text-xs",
              service.Status === "running" && "text-success",
              service.Status === "error" && "text-destructive",
              service.Status !== "running" && service.Status !== "error" && "text-muted-foreground",
            )}
          >
            <span
              className={cn(
                "size-1.5 rounded-full",
                service.Status === "running" && "bg-success",
                service.Status === "error" && "bg-destructive",
                service.Status !== "running" && service.Status !== "error" && "bg-muted-foreground/40",
              )}
            />
            {stateWord(service)}
            {healthWord(service) && ` · ${healthWord(service)}`}
          </span>
        </td>
        <td className="px-4 py-2.5">
          <div className="flex justify-end gap-1">
            <Button
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs"
              aria-label={`${expanded ? "Hide" : "Show"} details for ${service.Name}`}
              onClick={() => setExpanded(!expanded)}
            >
              <ChevronDown className={cn("size-3.5 transition-transform", expanded && "rotate-180")} />
            </Button>
            {running ? (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-xs"
                aria-label={`Stop ${service.Name}`}
                disabled={busy}
                onClick={onStop}
              >
                <Square className="size-3.5" />
              </Button>
            ) : (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-xs"
                aria-label={`Start ${service.Name}`}
                disabled={busy}
                onClick={onStart}
              >
                <Play className="size-3.5" />
              </Button>
            )}
            <Button
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs"
              aria-label={`Restart ${service.Name}`}
              disabled={busy}
              onClick={onRestart}
            >
              <RotateCw className="size-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-xs text-destructive"
              aria-label={`Remove ${service.Name}`}
              disabled={busy}
              onClick={onRemove}
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="bg-muted/20">
          <td colSpan={4} className="px-4 py-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <CopyField label="URL" value={service.Connection.URL} />
              <CopyField label="Environment" value={service.Connection.Environment} />
              <CopyField label="Host" value={service.Connection.HostPort} />
              <CopyField label="Database" value={service.Connection.Database} />
              <CopyField label="User" value={service.Connection.User} />
              <div className="space-y-0.5">
                <span className="text-xs text-muted-foreground">Data directory</span>
                <p className="font-mono text-xs break-all">{service.DataDirectory}</p>
              </div>
            </div>

            <div className="mt-3 space-y-1.5 border-t border-border pt-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Output</span>
                  {!running && lines.length > 0 && (
                    <span className="text-[10px] text-muted-foreground">last run</span>
                  )}
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 px-2 text-xs"
                  aria-label={`Refresh output for ${service.Name}`}
                  disabled={output.isFetching}
                  onClick={() => void output.refetch()}
                >
                  <RotateCw className={cn("size-3", output.isFetching && "animate-spin")} />
                </Button>
              </div>
              {output.isError ? (
                <p className="text-xs text-destructive">
                  The output could not be read: {(output.error as Error).message}
                </p>
              ) : lines.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  {running
                    ? "The process has not written anything yet."
                    : "The last run wrote nothing, or none has happened yet."}
                </p>
              ) : (
                <pre
                  aria-label={`Output for ${service.Name}`}
                  className="max-h-56 overflow-auto rounded-md bg-muted/50 p-2 font-mono text-[11px] leading-relaxed text-foreground/80"
                >
                  {lines.join("\n")}
                </pre>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

/**
 * A project's services, kept current by the supervisor's events. Any surface
 * that shows service state uses this, so a project start updates all of them.
 */
export function useProjectServices(projectID: string) {
  const queryClient = useQueryClient();

  useEffect(() => {
    const refresh = () => {
      void queryClient.invalidateQueries({ queryKey: servicesKey(projectID) });
      // A process that has just exited has said its last words; read them again
      // so an open output panel shows why instead of the live tail that stopped.
      void queryClient.invalidateQueries({ queryKey: ["service-output"] });
    };
    const offState = Events.On("service:state", refresh);
    const offHealth = Events.On("service:health", refresh);
    return () => {
      offState();
      offHealth();
    };
  }, [queryClient, projectID]);

  return useQuery({
    queryKey: servicesKey(projectID),
    queryFn: async () => (await ServicesService.List(projectID)) ?? [],
  });
}

/** The project's backing services: attach, start, stop, restart and remove. */
export function ProjectServices({ projectID }: { projectID: string }) {
  const queryClient = useQueryClient();
  const [addOpen, setAddOpen] = useState(false);
  const [name, setName] = useState("");
  const [engine, setEngine] = useState("postgresql");
  const [actionError, setActionError] = useState("");
  const [pendingRemoval, setPendingRemoval] = useState<Instance | null>(null);

  const services = useProjectServices(projectID);
  const catalogue = useQuery({
    queryKey: serviceCatalogueKey,
    queryFn: async () => (await ServicesService.Catalogue()) ?? [],
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: servicesKey(projectID) });

  const create = useMutation({
    mutationFn: (template: string) =>
      ServicesService.Create(projectID, { Engine: template, Name: name }),
    onSuccess: async () => {
      setAddOpen(false);
      setName("");
      await refresh();
    },
  });

  const start = useMutation({
    mutationFn: (service: Instance) => ServicesService.Start(service.ID),
    onSuccess: refresh,
    onError: (error: Error) => setActionError(error.message),
  });
  const stop = useMutation({
    mutationFn: (service: Instance) => ServicesService.Stop(service.ID),
    onSuccess: refresh,
    onError: (error: Error) => setActionError(error.message),
  });
  const restart = useMutation({
    mutationFn: (service: Instance) => ServicesService.Restart(service.ID),
    onSuccess: refresh,
    onError: (error: Error) => setActionError(error.message),
  });
  const remove = useMutation({
    mutationFn: (service: Instance) => ServicesService.Remove(service.ID),
    onSuccess: async () => {
      setPendingRemoval(null);
      await refresh();
    },
    onError: (error: Error) => setActionError(error.message),
  });

  const templates: Template[] = catalogue.data ?? [];
  const selected = templates.find((template) => template.Engine === engine) ?? templates[0];
  const busy = start.isPending || stop.isPending || restart.isPending || remove.isPending;

  // Reasons stay on screen only while they describe the current action.
  function act(action: () => void) {
    setActionError("");
    action();
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-6 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold">Services</h2>
          <p className="text-xs text-muted-foreground">
            Databases, caches and other backing services for this project
          </p>
        </div>
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus className="mr-1.5 size-3.5" />
          Add service
        </Button>
      </div>

      {actionError && (
        <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
          <span>{actionError}</span>
        </div>
      )}

      {services.isError && (
        <p className="text-sm text-destructive">
          The services could not be read: {(services.error as Error).message}
        </p>
      )}

      {(services.data?.length ?? 0) === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-16 text-center">
          <Database className="mx-auto mb-3 size-8 text-muted-foreground/40" />
          <p className="text-sm font-medium">No services yet</p>
          <p className="text-xs text-muted-foreground">
            Add a database to give this project its own data directory and port.
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Name</th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Port</th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">Status</th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {services.data?.map((service) => (
                <ServiceRow
                  key={service.ID}
                  service={service}
                  busy={busy}
                  onStart={() => act(() => start.mutate(service))}
                  onStop={() => act(() => stop.mutate(service))}
                  onRestart={() => act(() => restart.mutate(service))}
                  onRemove={() => {
                    setActionError("");
                    setPendingRemoval(service);
                  }}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Dialog
        open={addOpen}
        onOpenChange={(open) => {
          setAddOpen(open);
          if (!open) {
            create.reset();
            setName("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add a service</DialogTitle>
            <DialogDescription>
              Locafy creates a data directory of its own and allocates a free port. The engine runs
              as a native process on loopback.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Engine</Label>
              <div className="grid gap-2">
                {templates.map((template) => (
                  <button
                    key={template.Engine}
                    type="button"
                    aria-pressed={template.Engine === selected?.Engine}
                    className={cn(
                      "rounded-lg border px-3 py-2 text-left text-sm",
                      template.Engine === selected?.Engine
                        ? "border-primary/50 bg-primary/10"
                        : "border-border hover:bg-muted/40",
                      !template.Available && "opacity-60",
                    )}
                    onClick={() => setEngine(template.Engine)}
                  >
                    <span className="flex items-center justify-between gap-2">
                      <span className="font-medium">
                        {template.Name} {template.Available ? template.Version : ""}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {template.Available ? `port ${template.DefaultPort}` : "Not installed"}
                      </span>
                    </span>
                    <span className="mt-0.5 block text-xs text-muted-foreground">
                      {template.Available ? template.Summary : template.Detail}
                    </span>
                  </button>
                ))}
                {templates.length === 0 && (
                  <p className="text-xs text-muted-foreground">No engine is in the catalogue.</p>
                )}
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="service-name">Name</Label>
              <Input
                id="service-name"
                value={name}
                placeholder={selected?.Name ?? "PostgreSQL"}
                onChange={(event) => setName(event.target.value)}
              />
            </div>

            {create.isError && (
              <p className="text-xs text-destructive">{(create.error as Error).message}</p>
            )}
          </div>

          <DialogFooter>
            <Button variant="ghost" onClick={() => setAddOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!selected?.Available || create.isPending}
              onClick={() => selected && create.mutate(selected.Engine)}
            >
              {create.isPending ? "Adding…" : "Add service"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={pendingRemoval !== null}
        onOpenChange={(open) => !open && setPendingRemoval(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {pendingRemoval?.Name}?</AlertDialogTitle>
            <AlertDialogDescription>
              The service is stopped and its Locafy record is deleted. Its data directory stays on
              disk — nothing in it is deleted, and it can be attached again later:
            </AlertDialogDescription>
          </AlertDialogHeader>
          <p className="font-mono text-xs break-all text-muted-foreground">
            {pendingRemoval?.DataDirectory}
          </p>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>Keep service</AlertDialogCancel>
            <AlertDialogAction
              disabled={remove.isPending}
              onClick={() => pendingRemoval && remove.mutate(pendingRemoval)}
            >
              Remove service
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
