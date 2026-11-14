import {
  ArrowRight,
  ArrowUpRight,
  Box,
  Boxes,
  BrainCircuit,
  Clock,
  Copy,
  EllipsisVertical,
  FileText,
  FolderKanban,
  Globe,
  Plus,
  Puzzle,
  Server,
  ShieldCheck,
  Terminal,
  Zap,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

/*
 * Placeholder content until the projects, tunnels and activity services
 * exist. Shapes mirror the records those services are expected to return, so
 * replacing this block with queries is a matter of swapping the source.
 */
const projects = [
  {
    name: "Etugora",
    description: "E-commerce platform",
    tags: ["API", "Auth", "ML Service", "+1"],
    url: "https://etugora.locafy.dev",
    tone: "primary" as const,
    trend: [3, 5, 4, 7, 6, 9, 8, 12, 11, 14],
  },
  {
    name: "Portfolio",
    description: "Personal portfolio site",
    tags: ["Web App", "API"],
    url: "https://portfolio.locafy.dev",
    tone: "accent" as const,
    trend: [8, 7, 9, 8, 6, 7, 5, 6, 4, 5],
  },
];

const applications = [
  { name: "api", description: "REST API for Etugora", port: ":8080", icon: Server, url: "https://api.etugora.locafy.dev" },
  { name: "auth", description: "Authentication Service", port: ":8081", icon: ShieldCheck, url: "https://auth.etugora.locafy.dev" },
  { name: "ml-service", description: "Machine Learning Service", port: ":8082", icon: BrainCircuit, url: "https://ml.etugora.locafy.dev" },
];

const quickActions = [
  { label: "Create New Project", icon: FolderKanban },
  { label: "Start Tunnel", icon: Zap },
  { label: "View Logs", icon: FileText },
  { label: "Manage Extensions", icon: Puzzle },
];

const activity = [
  { text: "Project ‘Etugora’ started", time: "2m ago", tone: "bg-primary" },
  { text: "Tunnel active", time: "5m ago", tone: "bg-success" },
  { text: "Application ‘api’ deployed", time: "12m ago", tone: "bg-accent" },
];

const metrics = [
  { label: "Projects", value: "2", delta: "+1 new", icon: FolderKanban, tone: "text-primary bg-primary/15" },
  { label: "Applications", value: "5", delta: "+2 running", icon: Boxes, tone: "text-success bg-success/15" },
  { label: "Active Tunnels", value: "1", delta: "+24h active", icon: Zap, tone: "text-success bg-success/15" },
  { label: "Uptime", value: "12h 34m", delta: "+12%", icon: Clock, tone: "text-accent bg-accent/15" },
];

function Sparkline({ points, className }: { points: number[]; className?: string }) {
  const max = Math.max(...points);
  const min = Math.min(...points);
  const span = max - min || 1;
  const coords = points
    .map((point, index) => {
      const x = (index / (points.length - 1)) * 100;
      const y = 28 - ((point - min) / span) * 24;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <svg viewBox="0 0 100 30" preserveAspectRatio="none" aria-hidden className={cn("h-8 w-full", className)}>
      <polyline
        points={coords}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}

function StatusPill({ label = "Running" }: { label?: string }) {
  return (
    <Badge variant="outline" className="gap-1.5 border-success/30 text-success">
      <span className="size-1.5 rounded-full bg-success" />
      {label}
    </Badge>
  );
}

export function Dashboard() {
  return (
    <div className="space-y-6 px-6 py-6">
      <section className="relative overflow-hidden rounded-2xl border border-border bg-card px-8 py-10">
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-primary/25 via-transparent to-accent/25" />
        <div className="pointer-events-none absolute -right-16 -top-24 size-72 rounded-full bg-primary/25 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-24 right-40 size-56 rounded-full bg-accent/20 blur-3xl" />
        <div className="relative flex flex-wrap items-center justify-between gap-8">
          <div className="max-w-2xl space-y-3">
            <h1 className="text-[clamp(1.5rem,3vw,2.25rem)] font-semibold leading-[1.1] tracking-tight">
              Your Local VPS.
              <br />
              Your Development Environment.
            </h1>
            <p className="text-sm text-muted-foreground">
              Develop, test, and share your applications with ease using Locafy.
            </p>
          </div>
          <div className="hidden shrink-0 rotate-[-6deg] rounded-xl border border-border bg-background/90 p-4 shadow-2xl min-[560px]:block">
            <div className="mb-3 flex gap-1.5">
              <span className="size-2.5 rounded-full bg-destructive" />
              <span className="size-2.5 rounded-full bg-accent" />
              <span className="size-2.5 rounded-full bg-success" />
            </div>
            <div className="flex items-center gap-2 font-mono text-sm text-muted-foreground">
              <span className="text-success">{'>'}</span>
              <span className="tracking-[0.2em]">locafy up</span>
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <Card key={metric.label} className="py-0">
            <CardContent className="flex items-center gap-3 p-4">
              <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-xl", metric.tone)}>
                <metric.icon className="size-5" />
              </span>
              <div className="min-w-0">
                <p className="text-xs text-muted-foreground">{metric.label}</p>
                <p className="text-xl font-semibold tracking-tight">{metric.value}</p>
                <p className="text-xs text-success">{metric.delta}</p>
              </div>
            </CardContent>
          </Card>
        ))}
      </section>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <h2 className="text-2xl font-semibold tracking-tight">Your Projects</h2>
              <p className="text-sm text-muted-foreground">
                Manage your development projects and their applications.
              </p>
            </div>
            <Button size="lg">
              <Plus />
              New Project
            </Button>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            {projects.map((project) => (
              <Card key={project.name} className="py-0">
                <CardContent className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    <span
                      className={cn(
                        "flex size-9 shrink-0 items-center justify-center rounded-lg",
                        project.tone === "primary" ? "bg-primary/15 text-primary" : "bg-accent/15 text-accent",
                      )}
                    >
                      {project.tone === "primary" ? <Box className="size-4.5" /> : <Terminal className="size-4.5" />}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <p className="truncate font-medium">{project.name}</p>
                        <StatusPill />
                      </div>
                      <p className="truncate text-xs text-muted-foreground">{project.description}</p>
                    </div>
                  </div>

                  <div className="flex items-center justify-between gap-3">
                    <div className="flex flex-wrap gap-1.5">
                      {project.tags.map((tag) => (
                        <Badge key={tag} variant="secondary" className="font-normal">
                          {tag}
                        </Badge>
                      ))}
                    </div>

                    <Sparkline
                      points={project.trend}
                      className={cn(
                        "h-9 w-28 shrink-0",
                        project.tone === "primary" ? "text-primary" : "text-accent",
                      )}
                    />
                  </div>

                  <div className="flex items-center justify-between gap-2 border-t border-border pt-3">
                    <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                      <Globe className="size-3.5 shrink-0" />
                      <span className="truncate">{project.url}</span>
                    </span>
                    <Button variant="secondary" size="sm">
                      Manage
                      <ArrowRight />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          <Card className="py-0">
            <CardContent className="space-y-4 p-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-start gap-3">
                  <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
                    <Box className="size-5" />
                  </span>
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <p className="text-lg font-semibold tracking-tight">Etugora Project</p>
                      <StatusPill />
                    </div>
                    <p className="text-sm text-muted-foreground">
                      A full-stack e-commerce platform with AI-powered recommendations.
                    </p>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  <Button variant="secondary">
                    Open Project
                    <ArrowUpRight />
                  </Button>
                  <Button variant="ghost" size="icon" aria-label="More project actions">
                    <EllipsisVertical />
                  </Button>
                </div>
              </div>

              <div className="flex gap-6 border-b border-border">
                {["Applications", "Tunnels", "Logs", "Settings"].map((tab, index) => (
                  <span
                    key={tab}
                    className={cn(
                      "-mb-px cursor-default border-b-2 pb-2 text-sm",
                      index === 0
                        ? "border-primary text-foreground"
                        : "border-transparent text-muted-foreground",
                    )}
                  >
                    {tab}
                  </span>
                ))}
              </div>

              <ul className="divide-y divide-border">
                {applications.map((app) => (
                  <li key={app.name} className="flex items-center gap-3 py-2.5">
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent/15 text-accent">
                      <app.icon className="size-4" />
                    </span>
                    <div className="min-w-28 shrink-0">
                      <p className="truncate text-sm font-medium">{app.name}</p>
                    </div>
                    <p className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                      {app.description}
                    </p>
                    <StatusPill />
                    <span className="w-14 shrink-0 font-mono text-xs text-muted-foreground">
                      {app.port}
                    </span>
                    <span className="hidden min-w-0 shrink-0 items-center gap-1.5 text-xs text-muted-foreground md:flex">
                      <span className="max-w-56 truncate">{app.url}</span>
                      <Copy className="size-3.5" />
                    </span>
                    <Button variant="ghost" size="icon-xs" aria-label={`More actions for ${app.name}`}>
                      <EllipsisVertical />
                    </Button>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>

        <aside className="space-y-4">
          <Card className="py-0">
            <CardHeader className="p-4 pb-0">
              <CardTitle className="text-base">Quick Actions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-0.5 p-2">
              {quickActions.map((action) => (
                <Button
                  key={action.label}
                  variant="ghost"
                  className="h-9 w-full justify-start gap-2.5 px-2 font-normal text-muted-foreground"
                >
                  <action.icon className="size-4" />
                  {action.label}
                </Button>
              ))}
            </CardContent>
          </Card>

          <Card className="py-0">
            <CardContent className="space-y-3 p-4">
              <div className="flex items-center justify-between">
                <p className="text-base font-medium">Local Tunnel</p>
                <Badge variant="outline" className="gap-1.5 border-success/30 text-success">
                  <span className="size-1.5 rounded-full bg-success" />
                  Active
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">Share your local app with the world.</p>
              <div className="flex items-center justify-between gap-2 rounded-lg border border-border bg-background px-2.5 py-1.5">
                <span className="truncate font-mono text-xs text-muted-foreground">
                  https://etugora.locafy.dev
                </span>
                <Copy className="size-3.5 shrink-0 text-muted-foreground" />
              </div>
              <Button variant="outline" className="w-full border-accent/40 text-accent hover:bg-accent/10 hover:text-accent">
                Manage Tunnel
                <ArrowRight />
              </Button>
            </CardContent>
          </Card>

          <Card className="py-0">
            <CardHeader className="p-4 pb-0">
              <CardTitle className="text-base">Recent Activity</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 p-4">
              <ul className="space-y-3">
                {activity.map((entry) => (
                  <li key={entry.text} className="flex gap-2.5">
                    <span className={cn("mt-1.5 size-1.5 shrink-0 rounded-full", entry.tone)} />
                    <div className="min-w-0">
                      <p className="truncate text-xs">{entry.text}</p>
                      <p className="text-[0.7rem] text-muted-foreground">{entry.time}</p>
                    </div>
                  </li>
                ))}
              </ul>
              <div className="flex justify-end">
                <Button variant="link" className="h-auto px-0 text-xs text-primary">
                  View All
                  <ArrowRight />
                </Button>
              </div>
            </CardContent>
          </Card>
        </aside>
      </div>
    </div>
  );
}
