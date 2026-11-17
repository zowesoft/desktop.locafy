import { ExternalLink, Terminal as TerminalIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export function TerminalScreen() {
  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="border-b border-border px-6 py-5">
        <h1 className="text-lg font-semibold tracking-tight">Terminal</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Machine-scope terminal — runs in the context of your local environment
        </p>
      </div>

      <div className="flex-1 space-y-5 px-6 py-6">
        {/* Coming soon card */}
        <Card className="overflow-hidden">
          {/* Terminal preview area */}
          <div className="flex items-center gap-1.5 border-b border-border bg-muted/40 px-4 py-2">
            <span className="size-3 rounded-full bg-destructive/50" />
            <span className="size-3 rounded-full bg-amber-400/50" />
            <span className="size-3 rounded-full bg-success/50" />
            <span className="ml-3 text-xs text-muted-foreground font-mono">
              locafy — machine terminal
            </span>
          </div>
          <CardContent className="flex min-h-[320px] flex-col items-center justify-center gap-4 bg-[#0D0D12] py-16">
            <div className="flex size-14 items-center justify-center rounded-xl bg-primary/15 text-primary">
              <TerminalIcon className="size-7" />
            </div>
            <div className="text-center space-y-2">
              <p className="font-medium text-white">Built-in terminal is coming soon</p>
              <p className="text-sm text-muted-foreground max-w-sm">
                The integrated xterm.js terminal is planned for a later phase. In the meantime, use
                your system terminal to interact with projects.
              </p>
            </div>
            <div className="mt-2 flex flex-wrap justify-center gap-2 text-xs text-muted-foreground/70 font-mono">
              <span>Opens in: </span>
              <span className="text-muted-foreground">your projects folder</span>
              <span>·</span>
              <span className="text-muted-foreground">your default shell</span>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="mt-2 bg-transparent border-muted-foreground/30 text-muted-foreground hover:text-foreground hover:border-muted-foreground"
            >
              <ExternalLink className="mr-1.5 size-3.5" />
              Open in system terminal
            </Button>
          </CardContent>
        </Card>

        {/* What it will offer */}
        <div className="rounded-lg border border-border p-4 space-y-3">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            When built-in terminal arrives
          </p>
          <ul className="space-y-2 text-sm text-muted-foreground">
            {[
              "Full xterm.js terminal with 256-color support",
              "Multiple tabs — machine-scope and per-project",
              "Automatic PATH injection of project runtimes",
              "Persistent shell history across sessions",
            ].map((item) => (
              <li key={item} className="flex items-start gap-2">
                <span className="mt-1.5 size-1 shrink-0 rounded-full bg-muted-foreground/40" />
                {item}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
