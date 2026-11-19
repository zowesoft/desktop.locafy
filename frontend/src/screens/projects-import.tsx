import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { FolderOpen } from "lucide-react";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import type {
  CreateInput,
  Detection,
} from "../../bindings/locafy.dev/desktop/internal/applications/models";
import * as DialogsService from "../../bindings/locafy.dev/desktop/internal/dialogs/service";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import type {
  Project,
  ScanResult,
} from "../../bindings/locafy.dev/desktop/internal/projects/models";
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

/* Directories are stored normalised and compared case-insensitively, the way
 * the database's unique index does. */
const directoryKey = (path: string) => path.replace(/\\/g, "/").toLowerCase();

/*
 * Imports an existing folder. Two shapes are offered after a scan: the
 * selected folder itself as one project — with the applications found inside
 * it reviewed before anything is recorded — or each folder inside as its own
 * project. Detection is advisory: folders without a marker stay available
 * behind a filter, and nothing is written into any folder.
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
  const [scan, setScan] = useState<ScanResult | null>(null);
  const [mode, setMode] = useState<"root" | "children">("children");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [showAll, setShowAll] = useState(false);
  const [failures, setFailures] = useState<string[]>([]);
  const [step, setStep] = useState<"scan" | "applications">("scan");
  const [project, setProject] = useState<Project | null>(null);
  const [proposals, setProposals] = useState<Detection[]>([]);
  const [chosen, setChosen] = useState<ReadonlySet<string>>(new Set());
  const [foundCount, setFoundCount] = useState(0);

  const scanMutation = useMutation({
    mutationFn: (root: string) => ProjectsService.ScanFolder(root),
    onMutate: () => setFailures([]),
    onSuccess: (found) => {
      const children = found.Children ?? [];
      setScan(found);
      // A folder that looks like a project itself is offered as one first; a
      // plain container keeps its contents as the default.
      setMode(found.Root.Marker !== "" || children.length === 0 ? "root" : "children");
      setSelected(new Set(children.filter((child) => child.Marker).map((child) => child.Path)));
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
      setScan((current) =>
        current
          ? {
              ...current,
              Children: (current.Children ?? []).filter(
                (child) => !imported.has(child.Path),
              ),
            }
          : current,
      );
      setSelected((current) => new Set([...current].filter((chosen2) => !imported.has(chosen2))));
      onImported();
      if (failed.length === 0) {
        close();
        return;
      }
      setFailures(failed.map((outcome) => `${baseName(outcome.Path)}: ${outcome.Error}`));
    },
    onError: (error) => setFailures([String(error)]),
  });

  /*
   * The selected folder becomes a project — or, when it already is one, the
   * existing project is picked up so a second import adds what is missing.
   * Only then are the applications inside it proposed, minus the ones already
   * recorded.
   */
  const importRoot = useMutation({
    mutationFn: async (rootPath: string) => {
      let linked: Project;
      try {
        linked = await ProjectsService.Link(rootPath);
      } catch (error) {
        const existing = ((await ProjectsService.List()) ?? []).find(
          (candidate) => directoryKey(candidate.RootPath) === directoryKey(rootPath),
        );
        if (!existing) throw error;
        linked = existing;
      }
      const found = (await ApplicationsService.Detect(linked.ID)) ?? [];
      const recorded = (await ApplicationsService.List(linked.ID)) ?? [];
      const taken = new Set(recorded.map((app) => directoryKey(app.Directory)));
      return {
        project: linked,
        found: found.length,
        proposals: found.filter(
          (detection) => !taken.has(directoryKey(detection.Directory)),
        ),
      };
    },
    onMutate: () => setFailures([]),
    onSuccess: (result) => {
      setProject(result.project);
      setProposals(result.proposals);
      setFoundCount(result.found);
      // The project folder itself is rarely an application of its own; with
      // siblings to choose from it starts unchecked.
      setChosen(
        new Set(
          result.proposals
            .filter((detection) => result.proposals.length === 1 || detection.Directory !== ".")
            .map((detection) => detection.Directory),
        ),
      );
      setStep("applications");
    },
    onError: (error) => setFailures([String(error)]),
  });

  const addApplications = useMutation({
    mutationFn: async (items: Detection[]) => {
      if (!project) throw new Error("the project is not open");
      const created: string[] = [];
      const failed: string[] = [];
      for (const item of items) {
        const input: CreateInput = {
          Name: item.Name,
          Directory: item.Directory,
          Language: item.Language,
          Framework: item.Framework,
          Marker: item.Marker,
          StartCommand: item.StartCommand,
          StopCommand: "",
          Source: "detected",
          ScaffoldRecipe: "",
          ScaffoldOptions: "",
        };
        try {
          await ApplicationsService.Create(project.ID, input);
          created.push(item.Directory);
        } catch (error) {
          failed.push(`${item.Name}: ${String(error)}`);
        }
      }
      return { created, failed };
    },
    onMutate: () => setFailures([]),
    onSuccess: ({ created, failed }) => {
      const added = new Set(created);
      setProposals((current) => current.filter((item) => !added.has(item.Directory)));
      setChosen((current) => new Set([...current].filter((dir) => !added.has(dir))));
      if (created.length > 0) onImported();
      if (failed.length > 0) {
        setFailures(failed);
        return;
      }
      close();
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
    setScan(null);
    setMode("children");
    setSelected(new Set());
    setShowAll(false);
    setFailures([]);
    setStep("scan");
    setProject(null);
    setProposals([]);
    setChosen(new Set());
    setFoundCount(0);
    onClose();
  };

  const children = scan?.Children ?? [];
  const root = scan?.Root ?? null;
  const visible = showAll ? children : children.filter((child) => child.Marker !== "");
  const chosenChildren = visible.filter((child) => selected.has(child.Path));

  const toggleChild = (candidatePath: string, checked: boolean) => {
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

  const toggleProposal = (directory: string, checked: boolean) => {
    setChosen((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(directory);
      } else {
        next.delete(directory);
      }
      return next;
    });
  };

  const submitScan = (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = path.trim();
    if (trimmed === "" || scanMutation.isPending) return;
    scanMutation.mutate(trimmed);
  };

  const submitImport = () => {
    if (scanMutation.isPending || link.isPending || importRoot.isPending) return;
    if (mode === "root") {
      if (root) importRoot.mutate(root.Path);
      return;
    }
    if (chosenChildren.length > 0) {
      link.mutate(chosenChildren.map((child) => child.Path));
    }
  };

  const submitApplications = () => {
    if (addApplications.isPending || chosen.size === 0) return;
    addApplications.mutate(proposals.filter((item) => chosen.has(item.Directory)));
  };

  const backToScan = () => {
    // The project may have just been linked, so the dashboard refreshes.
    setStep("scan");
    setFailures([]);
    onImported();
  };

  if (step === "applications" && project) {
    return (
      <Dialog open={open} onOpenChange={(next) => !next && close()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Applications in {project.Name}</DialogTitle>
            <DialogDescription>
              Locafy found runnable folders inside this project. Nothing is added until you
              confirm; uncheck anything that should not become an application.
            </DialogDescription>
          </DialogHeader>

          <div className="mt-4">
            {proposals.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {foundCount === 0
                  ? "Locafy found no applications inside this folder."
                  : "Every application Locafy found here is already registered."}
              </p>
            ) : (
              <ul className="max-h-60 space-y-1 overflow-y-auto pr-1">
                {proposals.map((item) => (
                  <li key={item.Directory}>
                    <label className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2 hover:bg-secondary">
                      <Checkbox
                        aria-label={item.Name}
                        checked={chosen.has(item.Directory)}
                        onCheckedChange={(next) => toggleProposal(item.Directory, next)}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm">{item.Name}</span>
                        <span className="block truncate font-mono text-xs text-muted-foreground">
                          {item.Directory} · {item.StartCommand || "no start command"}
                        </span>
                      </span>
                      <Badge variant="outline">{item.Marker}</Badge>
                    </label>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {failures.length > 0 && (
            <div role="alert" className="mt-3 space-y-1 text-sm text-destructive">
              {failures.map((failure) => (
                <p key={failure}>{failure}</p>
              ))}
            </div>
          )}

          <DialogFooter className="mt-5">
            <Button type="button" variant="outline" onClick={backToScan}>
              Back
            </Button>
            {proposals.length > 0 ? (
              <Button
                type="button"
                disabled={chosen.size === 0 || addApplications.isPending}
                onClick={submitApplications}
              >
                {addApplications.isPending
                  ? "Adding…"
                  : `Add ${chosen.size} ${chosen.size === 1 ? "application" : "applications"}`}
              </Button>
            ) : (
              <Button
                type="button"
                onClick={() => {
                  onImported();
                  close();
                }}
              >
                Done
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent>
        <form onSubmit={submitScan}>
          <DialogHeader>
            <DialogTitle>Import projects from a folder</DialogTitle>
            <DialogDescription>
              Import the folder itself as one project with its applications, or the folders
              inside it as separate projects. Nothing is moved, copied or changed.
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
                disabled={path.trim() === "" || scanMutation.isPending}
              >
                {scanMutation.isPending ? "Scanning…" : "Scan"}
              </Button>
            </div>
          </div>

          {scan !== null && root && (
            <div className="mt-4 space-y-3">
              <div className="space-y-2">
                <label className="flex cursor-pointer items-start gap-2.5 rounded-md border border-border px-3 py-2 hover:bg-secondary">
                  <input
                    type="radio"
                    name="import-mode"
                    className="mt-1 size-4 accent-primary"
                    aria-label={`Import ${root.Name} as one project`}
                    checked={mode === "root"}
                    onChange={() => setMode("root")}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-2 text-sm">
                      <span className="truncate">Import {root.Name} as one project</span>
                      {root.Marker ? (
                        <Badge variant="outline">{root.Marker}</Badge>
                      ) : (
                        <span className="text-xs text-muted-foreground">no marker</span>
                      )}
                    </span>
                    <span className="block text-xs text-muted-foreground">
                      Links this folder, then reviews the applications found inside it.
                    </span>
                  </span>
                </label>
                {children.length > 0 && (
                  <label className="flex cursor-pointer items-start gap-2.5 rounded-md border border-border px-3 py-2 hover:bg-secondary">
                    <input
                      type="radio"
                      name="import-mode"
                      className="mt-1 size-4 accent-primary"
                      aria-label="Import folders inside as separate projects"
                      checked={mode === "children"}
                      onChange={() => setMode("children")}
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm">
                        Import the {children.length}{" "}
                        {children.length === 1 ? "folder" : "folders"} inside as separate projects
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        Each folder becomes its own project.
                      </span>
                    </span>
                  </label>
                )}
              </div>

              {mode === "root" && (
                <p className="truncate text-sm text-muted-foreground" title={root.Path}>
                  {root.Path}
                </p>
              )}

              {mode === "children" && (
                <div>
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-sm text-muted-foreground">
                      {children.length === 0
                        ? "No folders inside this location."
                        : `${children.length} ${children.length === 1 ? "folder" : "folders"} found`}
                    </p>
                    {children.length > 0 && (
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
                      No folders with a project marker here. Turn on “Show folders without a
                      marker” to include them.
                    </p>
                  ) : (
                    <ul className="mt-2 max-h-60 space-y-1 overflow-y-auto pr-1">
                      {visible.map((child) => (
                        <li key={child.Path}>
                          <label className="flex cursor-pointer items-center gap-2.5 rounded-md border border-border px-3 py-2 hover:bg-secondary">
                            <Checkbox
                              aria-label={child.Name}
                              checked={selected.has(child.Path)}
                              onCheckedChange={(next) => toggleChild(child.Path, next)}
                            />
                            <span className="min-w-0 flex-1 truncate text-sm" title={child.Path}>
                              {child.Name}
                            </span>
                            {child.Marker ? (
                              <Badge variant="outline">{child.Marker}</Badge>
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
            {mode === "root" ? (
              <Button type="button" disabled={importRoot.isPending} onClick={submitImport}>
                {importRoot.isPending || link.isPending
                  ? "Importing…"
                  : `Import ${root?.Name ?? "folder"}`}
              </Button>
            ) : (
              <Button
                type="button"
                disabled={chosenChildren.length === 0 || link.isPending}
                onClick={submitImport}
              >
                Import {chosenChildren.length}{" "}
                {chosenChildren.length === 1 ? "project" : "projects"}
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
