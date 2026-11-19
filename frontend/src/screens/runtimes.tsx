import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useRuntimeCatalog } from "@/lib/runtimes-query";

/*
 * The runtime registry is a read of the machine: what is installed, where,
 * and in which version. Locafy does not install, move or remove anything
 * here; that work arrives with the downloads feature.
 */
export function RuntimesScreen() {
  const catalog = useRuntimeCatalog();
  const entries = catalog.data ?? [];
  const copies = entries.reduce(
    (count, entry) => count + (entry.copies?.length ?? 0),
    0,
  );

  return (
    <div className="flex min-h-full flex-col">
      <div className="border-b border-border px-6 py-5">
        <h1 className="text-lg font-semibold tracking-tight">Runtimes</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {copies === 0
            ? "What this machine has installed, read by Locafy"
            : `${copies} installed ${copies === 1 ? "copy" : "copies"} across ${entries.length} runtime kinds, read by Locafy`}
        </p>
      </div>

      <div className="flex-1 space-y-4 px-6 py-6">
        {catalog.isPending ? (
          <p className="text-sm text-muted-foreground">Reading the machine…</p>
        ) : catalog.isError ? (
          <p role="alert" className="text-sm text-destructive">
            Could not read the runtimes: {String(catalog.error)}
          </p>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            {entries.map((entry) => (
              <Card key={entry.kind.id} className="flex flex-col">
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-sm">{entry.kind.label}</CardTitle>
                    {entry.copies && entry.copies.length > 0 ? (
                      <Badge variant="secondary">
                        {entry.copies.length === 1 ? "1 copy" : `${entry.copies.length} copies`}
                      </Badge>
                    ) : (
                      <Badge variant="outline">Not found</Badge>
                    )}
                  </div>
                </CardHeader>
                <CardContent className="pt-0">
                  {entry.copies && entry.copies.length > 0 ? (
                    <ul className="space-y-1.5">
                      {entry.copies.map((copy) => (
                        <li
                          key={copy.id}
                          className="rounded-lg border border-border px-2.5 py-2 text-xs"
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="font-mono font-medium">
                              {copy.version || "unknown version"}
                            </span>
                            <Badge variant="outline">{copy.source}</Badge>
                          </div>
                          <p
                            className="mt-1 truncate font-mono text-[11px] text-muted-foreground"
                            title={copy.path}
                          >
                            {copy.path}
                          </p>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="rounded-lg border border-dashed border-border px-2 py-4 text-center text-xs text-muted-foreground">
                      No {entry.kind.label} executable on this machine
                    </p>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      <div className="border-t border-border px-6 py-4">
        <p className="text-xs text-muted-foreground">
          This screen reads what is installed and never changes it.
        </p>
      </div>
    </div>
  );
}
