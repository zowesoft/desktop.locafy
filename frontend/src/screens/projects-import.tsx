import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { FolderOpen } from "lucide-react";

import * as DialogsService from "../../bindings/locafy.dev/desktop/internal/dialogs/service";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type { FolderCandidate } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

const baseName = (path: string) => path.split(/[\\/]/).filter(Boolean).pop() ?? path;

/**
 * Scans a parent folder for project directories and links the ones the
 * operator picks. Detection is advisory: folders without a marker stay
 * available behind a filter, and nothing is written into any folder.
 */
export function ImportDialog({
  open,
  onClose,
  onImported,
}: {
  open: boolean;
  onClose: () => void;
  onImported: () => void;
}) {
  const [path, setPath] = useState("");
  const [browsing, setBrowsing] = useState(false);
  const [candidates, setCandidates] = useState<FolderCandidate[] | null>(null);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [showAll, setShowAll] = useState(false);
  const [failures, setFailures] = useState<string[]>([]);

  const scan = useMutation({
    mutationFn: (root: string) => ProjectsService.ScanFolder(root),
    onMutate: () => setFailures([]),
    onSuccess: (found) => {
      const list = found ?? [];
      setCandidates(list);
      setSelected(new Set(list.filter((candidate) => candidate.Marker).map((candidate) => candidate.Path)));
      setShowAll(false);
    },
    onError: (error) => setFailures([String(error)]),
  });

  const link = useMutation({
    mutationFn: (paths: string[]) => ProjectsService.LinkMany(paths),
    onMutate: () => setFailures([]),
    onSuccess: (outcomes) => {
      const failed = (outcomes ?? []).filter((outcome) => outcome.Error);
      const imported = new Set(
        (outcomes ?? []).filter((outcome) => !outcome.Error).map((outcome) => outcome.Path),
      );
      setCandidates((current) => (current ?? []).filter((candidate) => !imported.has(candidate.Path)));
      setSelected((current) => new Set([...current].filter((chosen) => !imported.has(chosen))));
      onImported();
      if (failed.length === 0) {
        close();
        return;
      }
      setFailures(failed.map((outcome) => `${baseName(outcome.Path)}: ${outcome.Error}`));
    },
    onError: (error) => setFailures([String(error)]),
  });

  // The native dialog returns an empty path when it is cancelled, which leaves
  // whatever was typed in place.
  const browse = async () => {
    setBrowsing(true);
    setFailures([]);
    try {
      const picked = await DialogsService.ChooseFolder("Scan folders inside this parent");
      if (picked) setPath(picked);
    } catch (error) {
      setFailures([String(error)]);
    } finally {
      setBrowsing(false);
    }
  };

  const close = () => {
    setPath("");
    setCandidates(null);
    setSelected(new Set());
    setShowAll(false);
    setFailures([]);
    onClose();
  };

  const visible =
    candidates === null ? [] : showAll ? candidates : candidates.filter((candidate) => candidate.Marker !== "");
  const chosen = visible.filter((candidate) => selected.has(candidate.Path));

  const toggle = (candidatePath: string, checked: boolean) => {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(candidatePath);
      } else {
        next.delete(candidatePath);
      }
      return next;
    });
  };

  const submitScan = (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = path.trim();
    if (trimmed === "" || scan.isPending) return;
    scan.mutate(trimmed);
  };

  const submitImport = () => {
    if (chosen.length === 0 || link.isPending) return;
    link.mutate(chosen.map((candidate) => candidate.Path));
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent>
        <form onSubmit={submitScan}>
          <DialogHeader>
            <DialogTitle>Import projects from a folder</DialogTitle>
            <DialogDescription>
              Locafy lists the folders inside and links the ones you pick. Nothing is moved, copied
              or changed.
            </DialogDescription>
          </DialogHeader>

          <div className="mt-4 space-y-1.5">
            <label className="text-sm font-medium" htmlFor="import-path">
              Folder path
            </label>
            <div className="flex gap-2">
              <Input
                id="import-path"
                className="min-w-0 flex-1"
                autoFocus
                placeholder="C:\laragon\www"
                value={path}
                onChange={(event) => setPath(event.target.value)}
              />
              <Button type="button" variant="outline" disabled={browsing} onClick={browse}>
                <FolderOpen /> Browse
              </Button>
              <Button
                type="submit"
                variant="secondary"
                disabled={path.trim() === "" || scan.isPending}
              >
                {scan.isPending ? "Scanning…" : "Scan"}
              </Button>
            </div>
          </div>

          {candidates !== null && (
            <div className="mt-4">
              <div className="flex items-center justify-between gap-3">
                <p className="text-sm text-muted-foreground">
                  {candidates.length === 0
                    ? "No folders inside this location."
                    : `${candidates.length} ${candidates.length === 1 ? "folder" : "folders"} found`}
                </p>
                {candidates.length > 0 && (
                  <label className="flex cursor-pointer items-center gap-2 text-sm">
                    <Checkbox
                      aria-label="Show folders without a marker"
                      checked={showAll}
                      onCheckedChange={(next) => setShowAll(next)}
                    />
                    Show folders without a marker
                  </label>
                )}
              </div>

              {visible.length === 0 ? (
                <p className="mt-3 text-sm text-muted-foreground">
                  No folders with a project marker here. Turn on “Show folders without a marker” to
                  include them.
                </p>
              ) : (
                <ul className="mt-2 max-h-60 space-y-1 overflow-y-auto pr-1">
                  {visible.map((candidate) => (
                    <li key={candidate.Path}>
                      <label className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2 hover:bg-secondary">
                        <Checkbox
                          aria-label={candidate.Name}
                          checked={selected.has(candidate.Path)}
                          onCheckedChange={(next) => toggle(candidate.Path, next)}
                        />
                        <span className="min-w-0 flex-1 truncate text-sm" title={candidate.Path}>
                          {candidate.Name}
                        </span>
                        {candidate.Marker ? (
                          <Badge variant="outline">{candidate.Marker}</Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">no marker</span>
                        )}
                      </label>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          {failures.length > 0 && (
            <div role="alert" className="mt-3 space-y-1 text-sm text-destructive">
              {failures.map((failure) => (
                <p key={failure}>{failure}</p>
              ))}
            </div>
          )}

          <DialogFooter className="mt-5">
            <Button type="button" variant="outline" onClick={close}>
              Cancel
            </Button>
            <Button type="button" disabled={chosen.length === 0 || link.isPending} onClick={submitImport}>
              Import {chosen.length} {chosen.length === 1 ? "project" : "projects"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
