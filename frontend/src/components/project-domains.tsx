import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Globe, Plus, Trash2 } from "lucide-react";

import * as NetworkService from "../../bindings/locafy.dev/desktop/internal/network/service";
import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
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

export const domainsKey = (projectID: string) => ["domains", projectID] as const;
const proxyKey = ["network", "proxy"] as const;

function messageOf(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

/**
 * Local hostnames for a project. The list, the assignment and the proxy's own
 * state all come from the engine; nothing here is guessed.
 */
export function ProjectDomains({ projectID }: { projectID: string }) {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [hostname, setHostname] = useState("");
  const [applicationID, setApplicationID] = useState("");
  const [removing, setRemoving] = useState<{ id: string; hostname: string } | null>(null);

  const domains = useQuery({
    queryKey: domainsKey(projectID),
    queryFn: async () => (await NetworkService.List(projectID)) ?? [],
  });
  const applications = useQuery({
    queryKey: ["applications", projectID],
    queryFn: async () => (await ApplicationsService.List(projectID)) ?? [],
  });
  const proxy = useQuery({
    queryKey: proxyKey,
    queryFn: () => NetworkService.ProxyStatus(),
    refetchInterval: 3000,
  });
  const trustPlan = useQuery({
    queryKey: ["network", "trust-plan"],
    queryFn: () => NetworkService.TrustPlan(),
    retry: false,
  });
  const [trusted, setTrusted] = useState("");

  const trust = useMutation({
    mutationFn: () => NetworkService.TrustCertificate(),
    onSuccess: (message) => setTrusted(message),
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: domainsKey(projectID) });
    void queryClient.invalidateQueries({ queryKey: proxyKey });
  };

  const assign = useMutation({
    mutationFn: () => NetworkService.Assign(projectID, applicationID, hostname),
    onSuccess: () => {
      setDialogOpen(false);
      setHostname("");
      setApplicationID("");
      refresh();
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => NetworkService.Remove(id),
    onSuccess: () => {
      setRemoving(null);
      refresh();
    },
  });

  const rows = domains.data ?? [];
  const running = proxy.data?.running ?? false;

  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold">Domains</h2>
          <p className="text-xs text-muted-foreground">
            Local hostnames served by Locafy's proxy to this project's applications
          </p>
        </div>
        <Button size="sm" onClick={() => setDialogOpen(true)}>
          <Plus className="mr-1.5 size-3.5" />
          Add domain
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border p-3 text-xs">
        <Globe className="size-3.5 text-muted-foreground" />
        {proxy.data?.error ? (
          <span className="flex items-center gap-1.5 text-destructive">
            <AlertTriangle className="size-3.5" />
            The local proxy is not listening: {proxy.data.error}
          </span>
        ) : (
          <span className="text-muted-foreground">
            {running ? "Local proxy listening on" : "Local proxy is stopped;"}{" "}
            <span className="font-mono text-foreground">
              {proxy.data?.httpAddress ?? "127.0.0.1:80"}
            </span>
            {proxy.data?.httpsAddress ? (
              <>
                {" "}and <span className="font-mono text-foreground">{proxy.data.httpsAddress}</span>
              </>
            ) : null}
            {running ? ` · ${proxy.data?.routes ?? 0} routes` : ""}
          </span>
        )}
        {proxy.data?.httpsAddress ? (
          <Badge className="border-success/20 bg-success/10 text-xs text-success">HTTPS</Badge>
        ) : null}
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          disabled={trust.isPending || !trustPlan.data}
          onClick={() => trust.mutate()}
        >
          Trust Locafy's certificate
        </Button>
      </div>
      {trustPlan.data ? (
        <p className="text-xs text-muted-foreground">
          Trusting runs <span className="font-mono text-foreground">{trustPlan.data.command}</span> —{" "}
          {trustPlan.data.explanation}
        </p>
      ) : null}
      {trusted ? <p className="text-xs text-muted-foreground">{trusted}</p> : null}
      {proxy.data?.httpsAddress ? (
        <p className="text-xs text-muted-foreground">
          The certificate names a revocation list this proxy serves at{" "}
          <span className="font-mono">/.well-known/locafy/ca.crl</span>, so clients that insist on
          one — <span className="font-mono">curl</span> with schannel, .NET with
          <span className="font-mono"> CheckCertificateRevocationList </span>
          — accept it too. If no local listener is running, they have nothing to check against.
        </p>
      ) : null}
      {trust.isError ? (
        <p className="text-xs text-destructive">{messageOf(trust.error)}</p>
      ) : null}

      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="border-b border-border bg-muted/40">
            <tr>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Domain
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                App
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Port
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Routing
              </th>
              <th className="px-4 py-2.5" />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rows.map((domain) => (
              <tr key={domain.id} className="hover:bg-muted/30">
                <td className="px-4 py-2.5 font-mono text-xs">{domain.hostname}</td>
                <td className="px-4 py-2.5 text-muted-foreground">{domain.applicationName}</td>
                <td className="px-4 py-2.5 font-mono text-xs">
                  {domain.port ?? <span className="text-muted-foreground">—</span>}
                </td>
                <td className="px-4 py-2.5">
                  {domain.port ? (
                    <Badge className="border-success/20 bg-success/10 text-xs text-success">
                      Routing
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs text-muted-foreground">
                      Waiting for a port
                    </Badge>
                  )}
                </td>
                <td className="px-4 py-2.5">
                  <div className="flex justify-end">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Remove ${domain.hostname}`}
                      onClick={() => setRemoving({ id: domain.id, hostname: domain.hostname })}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 && !domains.isLoading ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-xs text-muted-foreground">
                  No domain is assigned yet. Give an application a name like{" "}
                  <span className="font-mono">api.etugora.test</span> to reach it without a port.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <p className="text-xs text-muted-foreground">
        A domain routes once its application has bound a port. HTTPS uses a certificate from
        Locafy's own local authority; browsers only accept it after you trust that certificate,
        and the button above shows exactly what trusting it runs. Locafy does not edit your hosts
        file: add <span className="font-mono">127.0.0.1 {rows[0]?.hostname ?? "api.project.test"}</span> to
        it, or use a wildcard <span className="font-mono">.test</span> resolver, so the name reaches
        this machine.
      </p>

      <Dialog open={dialogOpen} onOpenChange={(open) => !assign.isPending && setDialogOpen(open)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add domain</DialogTitle>
            <DialogDescription>
              Pick an application and the name you want to type in the browser. The name must end
              in <span className="font-mono">.test</span>.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="domain-hostname">Hostname</Label>
              <Input
                id="domain-hostname"
                value={hostname}
                placeholder="api.etugora.test"
                onChange={(event) => setHostname(event.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="domain-application">Application</Label>
              <select
                id="domain-application"
                className="h-9 w-full rounded-md border border-border bg-background px-2 text-sm"
                value={applicationID}
                onChange={(event) => setApplicationID(event.target.value)}
              >
                <option value="">Choose an application</option>
                {(applications.data ?? []).map((application) => (
                  <option key={application.ID} value={application.ID}>
                    {application.Name}
                  </option>
                ))}
              </select>
            </div>
            {assign.isError ? (
              <p className="text-xs text-destructive">{messageOf(assign.error)}</p>
            ) : null}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)} disabled={assign.isPending}>
              Cancel
            </Button>
            <Button
              onClick={() => assign.mutate()}
              disabled={assign.isPending || hostname.trim() === "" || applicationID === ""}
            >
              Add domain
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={removing !== null} onOpenChange={(open) => !open && setRemoving(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {removing?.hostname}?</AlertDialogTitle>
            <AlertDialogDescription>
              The hostname stops routing immediately and becomes available to any project. The
              application keeps running.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={remove.isPending}
              onClick={(event) => {
                event.preventDefault();
                if (removing) remove.mutate(removing.id);
              }}
            >
              Remove domain
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
