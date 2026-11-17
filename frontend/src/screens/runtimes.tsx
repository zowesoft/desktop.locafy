import { useState } from "react";
import { Download, Info, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

// ─── Mock data ────────────────────────────────────────────────────────────────

const RUNTIMES = [
  {
    id: "php",
    language: "PHP",
    color: "bg-blue-500/15 text-blue-400",
    installedVersions: [
      { version: "8.4.1", status: "installed", default: true, updateAvailable: "8.4.2", usedBy: ["Etugora API"] },
      { version: "8.3.14", status: "installed", default: false, updateAvailable: null, usedBy: [] },
    ],
    latestStable: "8.4.2",
  },
  {
    id: "node",
    language: "Node.js",
    color: "bg-green-500/15 text-green-400",
    installedVersions: [
      { version: "24.0.1", status: "installed", default: true, updateAvailable: "24.0.2", usedBy: ["Etugora Auth", "Dashboard"] },
      { version: "22.14.0", status: "installed", default: false, updateAvailable: null, usedBy: ["Folio"] },
    ],
    latestStable: "24.0.2",
  },
  {
    id: "python",
    language: "Python",
    color: "bg-yellow-500/15 text-yellow-400",
    installedVersions: [
      { version: "3.13.0", status: "installed", default: true, updateAvailable: null, usedBy: ["ML Service"] },
    ],
    latestStable: "3.13.1",
  },
  {
    id: "go",
    language: "Go",
    color: "bg-cyan-500/15 text-cyan-400",
    installedVersions: [
      { version: "1.24.3", status: "installed", default: true, updateAvailable: null, usedBy: [] },
    ],
    latestStable: "1.24.3",
  },
  {
    id: "ruby",
    language: "Ruby",
    color: "bg-red-500/15 text-red-400",
    installedVersions: [],
    latestStable: "3.4.1",
  },
];

type RuntimeStatus = "installed" | "installing" | "not-installed";

function StatusPill({ status }: { status: RuntimeStatus }) {
  if (status === "installed") {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-success">
        <span className="size-1.5 rounded-full bg-success" />
        Installed
      </span>
    );
  }
  if (status === "installing") {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-primary">
        <span className="size-1.5 animate-pulse rounded-full bg-primary" />
        Installing…
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className="size-1.5 rounded-full bg-muted-foreground/40" />
      Not installed
    </span>
  );
}

export function RuntimesScreen() {
  const [search, setSearch] = useState("");
  const [installDir] = useState("~/.locafy/runtimes");
  const [diskUsed] = useState("3.2 GB");

  const filtered = RUNTIMES.filter(
    (rt) =>
      rt.language.toLowerCase().includes(search.toLowerCase()) ||
      rt.id.toLowerCase().includes(search.toLowerCase()),
  );

  const updatesAvailable = RUNTIMES.flatMap((rt) =>
    rt.installedVersions.filter((v) => v.updateAvailable),
  ).length;

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-border px-6 py-5">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Runtimes</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Language runtimes installed and managed by Locafy
          </p>
        </div>
        <div className="flex gap-2">
          {updatesAvailable > 0 && (
            <Button variant="outline" size="sm">
              <RefreshCw className="mr-1.5 size-3.5" />
              Update all ({updatesAvailable})
            </Button>
          )}
          <Button size="sm">
            <Plus className="mr-1.5 size-3.5" />
            Add runtime
          </Button>
        </div>
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {/* Update banner */}
        {updatesAvailable > 0 && (
          <div className="flex items-center gap-3 rounded-lg border border-blue-500/20 bg-blue-500/5 px-4 py-3">
            <Info className="size-4 shrink-0 text-blue-400" />
            <span className="flex-1 text-sm">
              {updatesAvailable} runtime update{updatesAvailable > 1 ? "s" : ""} available — PHP
              8.4.2 and Node.js 24.0.2
            </span>
          </div>
        )}

        {/* Search */}
        <Input
          placeholder="Filter runtimes…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
        />

        {/* Runtime cards */}
        <div className="space-y-3">
          {filtered.map((rt) => (
            <Card key={rt.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span
                      className={cn(
                        "inline-flex min-w-[72px] items-center justify-center rounded px-2 py-0.5 text-xs font-medium",
                        rt.color,
                      )}
                    >
                      {rt.language}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      Latest stable: <span className="font-mono font-medium text-foreground">{rt.latestStable}</span>
                    </span>
                  </div>
                  <Button size="sm" variant="outline" className="h-7 text-xs">
                    <Plus className="mr-1 size-3" />
                    Install version
                  </Button>
                </div>
              </CardHeader>
              {rt.installedVersions.length > 0 ? (
                <CardContent className="pt-0">
                  <div className="overflow-hidden rounded-lg border border-border">
                    <table className="w-full text-sm">
                      <thead className="border-b border-border bg-muted/40">
                        <tr>
                          <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">
                            Version
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">
                            Status
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">
                            Used by
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">
                            Update
                          </th>
                          <th className="px-4 py-2" />
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {rt.installedVersions.map((v) => (
                          <tr key={v.version}>
                            <td className="px-4 py-2.5">
                              <div className="flex items-center gap-2">
                                <span className="font-mono text-sm">{v.version}</span>
                                {v.default && (
                                  <Badge className="text-xs">Default</Badge>
                                )}
                              </div>
                            </td>
                            <td className="px-4 py-2.5">
                              <StatusPill status="installed" />
                            </td>
                            <td className="px-4 py-2.5 text-xs text-muted-foreground">
                              {v.usedBy.length > 0 ? v.usedBy.join(", ") : "—"}
                            </td>
                            <td className="px-4 py-2.5">
                              {v.updateAvailable ? (
                                <span className="inline-flex items-center gap-1.5 text-xs text-primary">
                                  <Download className="size-3" />
                                  {v.updateAvailable}
                                </span>
                              ) : (
                                <span className="text-xs text-muted-foreground">Up to date</span>
                              )}
                            </td>
                            <td className="px-4 py-2.5">
                              <div className="flex justify-end gap-1">
                                {v.updateAvailable && (
                                  <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                                    Update
                                  </Button>
                                )}
                                {!v.default && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                                    disabled={v.usedBy.length > 0}
                                    title={v.usedBy.length > 0 ? "Cannot remove — in use" : "Remove"}
                                  >
                                    <Trash2 className="size-3" />
                                  </Button>
                                )}
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              ) : (
                <CardContent className="pt-0">
                  <div className="rounded-lg border border-dashed border-border py-5 text-center">
                    <p className="text-xs text-muted-foreground">
                      No {rt.language} versions installed.{" "}
                      <button className="text-primary hover:underline">Install {rt.latestStable}</button>
                    </p>
                  </div>
                </CardContent>
              )}
            </Card>
          ))}
        </div>

        {/* Install location card */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">Install location</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex gap-2">
              <Input
                value={installDir}
                readOnly
                className="flex-1 font-mono text-xs text-muted-foreground"
              />
              <Button variant="outline">Change</Button>
            </div>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Disk used by runtimes</span>
                <span className="font-medium text-foreground">{diskUsed}</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                <div className="h-full w-1/3 rounded-full bg-primary" />
              </div>
            </div>
            <Button variant="outline" size="sm" className="text-xs">
              <Trash2 className="mr-1.5 size-3.5" />
              Remove unused runtimes
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
