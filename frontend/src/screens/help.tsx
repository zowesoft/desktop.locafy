import { useState } from "react";
import {
  BookOpen,
  ChevronDown,
  ChevronRight,
  Download,
  ExternalLink,
  HelpCircle,
  Search,
  Shield,
  Wrench,
  Zap,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

// ─── Mock content ─────────────────────────────────────────────────────────────

interface HelpSection {
  id: string;
  title: string;
  icon: React.ElementType;
  articles: HelpArticle[];
}

interface HelpArticle {
  id: string;
  title: string;
  body: string;
}

const HELP_SECTIONS: HelpSection[] = [
  {
    id: "getting-started",
    title: "Getting started",
    icon: Zap,
    articles: [
      {
        id: "create-project",
        title: "Create your first project",
        body: "Open the Projects screen and click \"New project\". Give it a name, choose Managed (Locafy picks the folder) or Linked (point to an existing folder), then click Create. Your project appears immediately in the list.",
      },
      {
        id: "add-app",
        title: "Add an application to a project",
        body: "Open a project and go to the Applications tab. Click \"Add application\", point it at your app folder, and Locafy will auto-detect the framework. Review and confirm the detected settings, then click Save.",
      },
      {
        id: "install-runtime",
        title: "Install a language runtime",
        body: "Go to Runtimes (in the sidebar) and find the language you need. Click \"Install version\" next to the version you want. Locafy downloads and configures it automatically — no PATH changes required.",
      },
    ],
  },
  {
    id: "troubleshooting",
    title: "Troubleshooting",
    icon: Wrench,
    articles: [
      {
        id: "port-in-use",
        title: "Port already in use",
        body: "If Locafy can't start an app because the port is taken, go to Settings → Network and adjust the app port range, or stop the conflicting process. Use \"Warn when port is in use\" to catch this early.",
      },
      {
        id: "ca-not-trusted",
        title: "HTTPS certificate not trusted in browser",
        body: "Open Settings → Domains & HTTPS and click \"Reinstall CA\". On Firefox, also enable security.enterprise_roots.enabled in about:config. Restart your browser after reinstalling.",
      },
      {
        id: "project-not-found",
        title: "Project disappeared from the list",
        body: "Locafy checks that the project folder exists when it starts. If you moved or deleted the folder, Locafy removes the entry. Re-link the project by clicking \"Link existing project\" in the Projects screen and pointing to the new location.",
      },
      {
        id: "service-failed",
        title: "Service failed to start",
        body: "Open the Logs screen and filter by the failing service. Common causes: port conflict, missing data directory, or Docker not running. Shared service failures affect all projects — check Settings → Services & Instances for lifecycle options.",
      },
    ],
  },
  {
    id: "concepts",
    title: "Key concepts",
    icon: BookOpen,
    articles: [
      {
        id: "managed-vs-linked",
        title: "Managed vs Linked projects",
        body: "Managed projects have their folder created inside Locafy's workspace directory. Locafy may write config files there. Linked projects point to any existing folder — Locafy never writes inside a linked project's folder.",
      },
      {
        id: "shared-vs-dedicated",
        title: "Shared vs Dedicated services",
        body: "Shared services (e.g. a single PostgreSQL instance) serve all projects from one process — lower memory, but projects share data. Dedicated services give each project its own isolated instance — more memory, but complete isolation.",
      },
      {
        id: "local-domains",
        title: "Local domain names",
        body: "Locafy assigns each app a local hostname like api.etugora.test. These resolve via the system hosts file (or a local DNS resolver). They never leave your machine — the .test suffix is reserved for local testing by ICANN.",
      },
    ],
  },
];

function ArticleRow({ article }: { article: HelpArticle }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="border-b border-border last:border-0">
      <button
        type="button"
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-muted/40"
        onClick={() => setOpen((p) => !p)}
        aria-expanded={open}
      >
        <span className="text-sm font-medium">{article.title}</span>
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {open && (
        <div className="bg-muted/20 px-4 pb-4 pt-1">
          <p className="text-sm text-muted-foreground leading-relaxed">{article.body}</p>
        </div>
      )}
    </div>
  );
}

export function HelpScreen() {
  const [search, setSearch] = useState("");
  const [exporting, setExporting] = useState(false);

  const filtered = HELP_SECTIONS.map((section) => ({
    ...section,
    articles: section.articles.filter(
      (a) =>
        !search ||
        a.title.toLowerCase().includes(search.toLowerCase()) ||
        a.body.toLowerCase().includes(search.toLowerCase()),
    ),
  })).filter((section) => section.articles.length > 0);

  const handleExport = () => {
    setExporting(true);
    setTimeout(() => setExporting(false), 2000);
  };

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="border-b border-border px-6 py-5">
        <h1 className="text-lg font-semibold tracking-tight">Help & Support</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Offline guide — works without internet
        </p>
      </div>

      <div className="flex-1 space-y-6 px-6 py-6">
        {/* Search */}
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="Search help articles…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        {/* Help sections */}
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-16">
            <HelpCircle className="size-10 text-muted-foreground/40" />
            <p className="text-sm text-muted-foreground">No articles match "{search}".</p>
          </div>
        ) : (
          <div className="space-y-4">
            {filtered.map((section) => {
              const Icon = section.icon;
              return (
                <Card key={section.id}>
                  <CardHeader className="pb-0 pt-4">
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <Icon className="size-4 text-primary" />
                      {section.title}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="pt-3">
                    <div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
                      {section.articles.map((article) => (
                        <ArticleRow key={article.id} article={article} />
                      ))}
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}

        {/* Documentation link */}
        <div className="flex items-center justify-between rounded-lg border border-border p-4">
          <div className="flex items-center gap-3">
            <BookOpen className="size-5 text-muted-foreground" />
            <div>
              <p className="text-sm font-medium">Full documentation</p>
              <p className="text-xs text-muted-foreground">locafy.dev/docs — requires internet</p>
            </div>
          </div>
          <Button variant="outline" size="sm">
            <ExternalLink className="mr-1.5 size-3.5" />
            Open docs
          </Button>
        </div>

        {/* Diagnostics export */}
        <Card className="border-muted">
          <CardContent className="pt-5 space-y-3">
            <div className="flex items-start gap-3">
              <Shield className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
              <div className="space-y-1">
                <p className="text-sm font-medium">Export diagnostics bundle</p>
                <p className="text-xs text-muted-foreground">
                  Creates a zip with logs, config, and system info to share with support. All
                  passwords, tokens, and secrets are automatically masked before export — review the
                  bundle before sharing.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Button
                variant="outline"
                size="sm"
                onClick={handleExport}
                disabled={exporting}
              >
                <Download className="mr-1.5 size-3.5" />
                {exporting ? "Preparing bundle…" : "Export diagnostics"}
              </Button>
              <Badge
                variant="secondary"
                className="bg-amber-500/10 text-amber-500 border-amber-500/20 text-xs"
              >
                Review before sharing
              </Badge>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
