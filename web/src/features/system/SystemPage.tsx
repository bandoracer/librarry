import React from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { BookOpenCheck, HardDrive, HardDriveDownload, HeartPulse, ListChecks, Play, RefreshCw, Timer } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  IconButton,
  InlineNotice,
  LoadingRow,
  PageHeader,
  ProgressBar,
  StatBar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import { navItems } from "../../app/nav";
import {
  keys,
  operabilityKeys,
  useAPIState,
  useDiskSpace,
  useIntegrationHealth,
  useInvalidatingMutation,
  useProviderHealth,
  useReadarrCompatibility,
  useReadiness,
  useSystemHealth,
  useSystemStatus,
  useSystemTasks
} from "../../lib/queries";
import { demoModeEnabled } from "../../lib/demo";
import { formatBytes, formatRelativeTime } from "../../lib/format";
import { runSystemTask, type DiskSpaceEntry, type ReadinessStep, type SystemHealthSeverity, type SystemTask } from "../../lib/api";
import "./system.css";

const pageSubtitle = navItems.find((item) => item.id === "providers")?.subtitle;

/* ------------------------------- Tone helpers ----------------------------- */

type Tone = "success" | "warn" | "danger" | "info";

const toneToken: Record<Tone, string> = {
  success: "var(--success)",
  warn: "var(--warn)",
  danger: "var(--danger)",
  info: "var(--info)"
};

function StatusDot(props: { tone: Tone }) {
  return <span className="system-dot" style={{ background: toneToken[props.tone] }} aria-hidden />;
}

function titleize(value: string): string {
  return value.replace(/_/g, " ");
}

/** Provider/integration health statuses: ready | missing_credentials | error. */
function healthTone(status: string): Tone {
  const normalized = status.toLowerCase();
  if (normalized === "ready" || normalized === "ok") return "success";
  if (normalized.includes("error") || normalized.includes("fail")) return "danger";
  return "warn";
}

function readinessTone(status: ReadinessStep["status"]): Tone {
  if (status === "ready") return "success";
  if (status === "warning") return "warn";
  return "danger";
}

function compatibilityTone(status: string): Tone {
  if (status === "ready") return "success";
  if (status === "delegated") return "info";
  return "warn";
}

function compatibilityStatusLabel(status: string): string {
  switch (status) {
    case "ready":
      return "Ready";
    case "partial":
      return "Partial";
    case "delegated":
      return "Delegated";
    default:
      return titleize(status);
  }
}

function compatibilityAuthLabel(mode: string): string {
  return mode === "api_key" ? "API key" : "Open";
}

function healthSeverityTone(severity: SystemHealthSeverity): Tone {
  if (severity === "ok") return "success";
  if (severity === "warning") return "warn";
  return "danger";
}

/** ProgressBar tone for a used-space fraction: danger >90%, warn >80%. */
function diskUsageTone(usedFraction: number): Tone {
  if (usedFraction > 0.9) return "danger";
  if (usedFraction > 0.8) return "warn";
  return "success";
}

function diskUsedFraction(disk: DiskSpaceEntry): number {
  if (!disk.totalBytes || disk.totalBytes <= 0) return 0;
  return Math.max(0, Math.min(1, (disk.totalBytes - disk.freeBytes) / disk.totalBytes));
}

/** Tooltip body for a task's Last Run cell: outcome summary or error detail. */
function taskLastRunTitle(task: SystemTask): string | undefined {
  if (task.lastError) return `Error: ${task.lastError}`;
  return task.lastOutcome || undefined;
}

/**
 * Legacy readiness steps navigated by view id ("settings" | "providers").
 * The router now has real settings tabs, so map known step ids onto the tab
 * that actually fixes them and fall back to the legacy view otherwise.
 */
const stepSettingsTab: Record<string, string> = {
  database: "/settings",
  library: "/settings/media",
  indexer: "/settings/connections",
  "download-client": "/settings/connections",
  "quality-profiles": "/settings/profiles"
};

function stepActionPath(step: ReadinessStep): string | null {
  if (!step.actionLabel || !step.targetView) return null;
  const mapped = stepSettingsTab[step.id];
  if (mapped) return mapped;
  return step.targetView === "providers" ? "/providers" : "/settings";
}

/**
 * Readiness and system-status queries have no demo seeds, so in demo installs
 * their failures are expected rather than actionable.
 */
function queryFailureNotice(error: unknown, demoHint: string) {
  if (demoModeEnabled) {
    return <InlineNotice tone="info">{demoHint}</InlineNotice>;
  }
  return (
    <InlineNotice tone="danger">{error instanceof Error ? error.message : "Request failed."}</InlineNotice>
  );
}

const VISIBLE_EXAMPLE_CHIPS = 4;

/* ---------------------------------- Page ---------------------------------- */

export default function SystemPage() {
  const client = useQueryClient();
  const toast = useToast();
  const apiState = useAPIState();
  const status = useSystemStatus();
  const readiness = useReadiness();
  const providers = useProviderHealth();
  const integrations = useIntegrationHealth();
  const compatibility = useReadarrCompatibility();
  const health = useSystemHealth();
  const tasks = useSystemTasks();
  const diskSpace = useDiskSpace();
  const [expandedCategories, setExpandedCategories] = React.useState<Record<string, boolean>>({});
  const [showAllHealth, setShowAllHealth] = React.useState(false);

  const runTask = useInvalidatingMutation(runSystemTask, [operabilityKeys.systemTasks]);

  const refreshing =
    status.isFetching ||
    readiness.isFetching ||
    providers.isFetching ||
    integrations.isFetching ||
    compatibility.isFetching ||
    health.isFetching ||
    tasks.isFetching ||
    diskSpace.isFetching;

  const refresh = () => {
    const targets = [
      keys.systemStatus,
      keys.readiness,
      keys.providerHealth,
      keys.integrationHealth,
      keys.readarrCompatibility,
      operabilityKeys.systemHealth,
      operabilityKeys.systemTasks,
      operabilityKeys.diskSpace
    ];
    void Promise.all(targets.map((queryKey) => client.invalidateQueries({ queryKey })));
  };

  const databaseReady = status.data?.databaseType ? status.data.databaseType !== "none" : false;
  const providerReadyCount = (providers.data ?? []).filter((provider) => provider.status === "ready").length;
  const integrationReadyCount = (integrations.data ?? []).filter((integration) => integration.status === "ready").length;

  const healthChecks = health.data ?? [];
  const healthIssues = healthChecks.filter((check) => check.severity !== "ok");
  const visibleHealthChecks = showAllHealth ? healthChecks : healthIssues;

  async function triggerTask(task: SystemTask) {
    try {
      const outcome = await runTask.mutateAsync(task.id);
      if (outcome.alreadyRunning) {
        toast.notify(outcome.message ?? `${task.name} is already running.`, "info");
      } else {
        toast.success(`${task.name} started.`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : `${task.name} run failed.`);
    }
  }

  return (
    <>
      <PageHeader
        title="System"
        subtitle={pageSubtitle}
        actions={<ToolbarButton icon={RefreshCw} label="Refresh" onClick={refresh} busy={refreshing} />}
      />

      <Card
        title="Status"
        subtitle="API build, database persistence, and connection state."
        actions={
          <Badge tone={apiState === "live" ? "success" : apiState === "demo" ? "info" : "danger"}>
            {apiState === "live" ? "Connected" : apiState === "demo" ? "Demo data" : "Offline"}
          </Badge>
        }
      >
        {status.isPending ? (
          <LoadingRow label="Checking system status…" />
        ) : status.isError ? (
          queryFailureNotice(status.error, "System status needs a live API and is not part of the demo data set.")
        ) : (
          <>
            <StatBar
              stats={[
                { label: "Instance", value: status.data.instanceName || status.data.appName },
                { label: "Version", value: status.data.version },
                {
                  label: "Database",
                  value: databaseReady ? `${status.data.databaseType} persistence` : "No database",
                  tone: databaseReady ? "success" : "warn"
                },
                {
                  label: "Authentication",
                  value: status.data.authentication ? titleize(status.data.authentication) : "Open"
                },
                {
                  label: "Runtime",
                  value: status.data.runtimeName
                    ? `${status.data.runtimeName} ${status.data.runtimeVersion ?? ""}`.trim()
                    : "—"
                }
              ]}
            />
            {!databaseReady ? (
              <div className="system-notice-gap">
                <InlineNotice tone="warn">
                  <strong>Database persistence is required for the Readarr workflow.</strong> Wanted queues, author
                  monitoring, release decisions, import reviews, history, and persisted integration settings need
                  Postgres. Set <code>LIBRARRY_DATABASE_URL</code> and restart the API.{" "}
                  <Link to="/settings">Open settings</Link>
                </InlineNotice>
              </div>
            ) : null}
          </>
        )}
      </Card>

      <Card
        title="Health"
        subtitle="Recurring checks over clients, indexers, roots, and disk space."
        actions={
          <>
            {healthChecks.length > 0 ? (
              <Badge tone={healthIssues.length === 0 ? "success" : healthSeverityTone(healthIssues.some((check) => check.severity === "error") ? "error" : "warning")}>
                {healthIssues.length === 0 ? "Healthy" : `${healthIssues.length} issue${healthIssues.length === 1 ? "" : "s"}`}
              </Badge>
            ) : null}
            {healthChecks.length > 0 && healthIssues.length < healthChecks.length ? (
              <Button size="sm" variant="ghost" onClick={() => setShowAllHealth((current) => !current)}>
                {showAllHealth ? "Issues only" : "Show all"}
              </Button>
            ) : null}
          </>
        }
      >
        {health.isPending ? (
          <LoadingRow label="Running health checks…" />
        ) : health.isError ? (
          queryFailureNotice(health.error, "Health checks need a live API and are not part of the demo data set.")
        ) : visibleHealthChecks.length === 0 ? (
          <EmptyState icon={HeartPulse} title="All healthy">
            {healthChecks.length === 0
              ? "No health checks reported any findings."
              : `All ${healthChecks.length} checks passed.`}
          </EmptyState>
        ) : (
          <div className="system-health-list">
            {visibleHealthChecks.map((check) => (
              <article className="system-tile system-health" key={check.id}>
                <div className="system-health-head">
                  <StatusDot tone={healthSeverityTone(check.severity)} />
                  <strong>{check.name}</strong>
                  <Badge tone={healthSeverityTone(check.severity)}>{titleize(check.severity)}</Badge>
                </div>
                <p className="system-muted">{check.message}</p>
              </article>
            ))}
          </div>
        )}
      </Card>

      <Card
        title="Setup checklist"
        subtitle={readiness.data?.summary ?? "Steps required before the Readarr workflow is usable."}
        actions={
          readiness.data ? (
            <Badge tone={readinessTone(readiness.data.status)}>{titleize(readiness.data.status)}</Badge>
          ) : undefined
        }
      >
        {readiness.isPending ? (
          <LoadingRow label="Evaluating setup steps…" />
        ) : readiness.isError ? (
          queryFailureNotice(readiness.error, "Readiness checks need a live API and are not part of the demo data set.")
        ) : readiness.data.steps.length === 0 ? (
          <EmptyState icon={ListChecks} title="No setup steps reported">
            The readiness endpoint returned an empty checklist.
          </EmptyState>
        ) : (
          <>
            <div className="system-step-list">
              {readiness.data.steps.map((step) => {
                const actionPath = stepActionPath(step);
                return (
                  <article className="system-tile system-step" key={step.id}>
                    <div className="system-step-main">
                      <StatusDot tone={readinessTone(step.status)} />
                      <div className="system-step-text">
                        <strong>{step.title}</strong>
                        <span className="system-muted">{step.message}</span>
                      </div>
                    </div>
                    <div className="system-step-side">
                      <Badge tone={step.required ? "accent" : "neutral"}>
                        {step.required ? "Required" : "Optional"}
                      </Badge>
                      {actionPath && step.actionLabel ? (
                        <Link className="btn btn-secondary btn-sm" to={actionPath}>
                          {step.actionLabel}
                        </Link>
                      ) : null}
                    </div>
                  </article>
                );
              })}
            </div>
            <p className="system-meta system-notice-gap">
              Checked {formatRelativeTime(readiness.data.generatedAt)}
            </p>
          </>
        )}
      </Card>

      <Card
        title="Metadata providers"
        subtitle={
          providers.data?.length
            ? `${providerReadyCount}/${providers.data.length} providers ready for lookup and import evidence.`
            : "Provider credentials and lookup health."
        }
      >
        {providers.isPending ? (
          <LoadingRow label="Checking provider health…" />
        ) : providers.isError ? (
          <InlineNotice tone="danger">
            {providers.error instanceof Error ? providers.error.message : "Provider health check failed."}
          </InlineNotice>
        ) : providers.data.length === 0 ? (
          <EmptyState icon={BookOpenCheck} title="No metadata providers">
            The API reported no metadata providers to check.
          </EmptyState>
        ) : (
          <div className="system-health-list">
            {providers.data.map((provider) => (
              <article className="system-tile system-health" key={provider.name}>
                <div className="system-health-head">
                  <StatusDot tone={healthTone(provider.status)} />
                  <strong>{provider.name}</strong>
                  <Badge tone={healthTone(provider.status)}>{titleize(provider.status)}</Badge>
                  {!provider.configured ? <Badge tone="neutral">Not configured</Badge> : null}
                </div>
                <p className="system-muted">{provider.message}</p>
                <span className="system-meta">Checked {formatRelativeTime(provider.checkedAt)}</span>
              </article>
            ))}
          </div>
        )}
      </Card>

      <Card
        title="Download & indexer integrations"
        subtitle={
          integrations.data?.length
            ? `${integrationReadyCount}/${integrations.data.length} acquisition integrations ready.`
            : "Prowlarr and download client health."
        }
      >
        {integrations.isPending ? (
          <LoadingRow label="Checking integration health…" />
        ) : integrations.isError ? (
          <InlineNotice tone="danger">
            {integrations.error instanceof Error ? integrations.error.message : "Integration health check failed."}
          </InlineNotice>
        ) : integrations.data.length === 0 ? (
          <EmptyState icon={HardDriveDownload} title="No acquisition integrations">
            Configure Prowlarr and a download client under Settings → Connections.
          </EmptyState>
        ) : (
          <div className="system-health-list">
            {integrations.data.map((integration) => (
              <article className="system-tile system-health" key={integration.name}>
                <div className="system-health-head">
                  <StatusDot tone={healthTone(integration.status)} />
                  <strong>{integration.name}</strong>
                  <Badge tone={healthTone(integration.status)}>{titleize(integration.status)}</Badge>
                  {!integration.configured ? <Badge tone="neutral">Not configured</Badge> : null}
                </div>
                <p className="system-muted">{integration.message}</p>
              </article>
            ))}
          </div>
        )}
      </Card>

      <Card
        title="Scheduled Tasks"
        subtitle="Background workers: cadence, last outcome, and manual run-now."
        padded={!tasks.data?.length}
      >
        {tasks.isPending ? (
          <LoadingRow label="Loading scheduled tasks…" />
        ) : tasks.isError ? (
          queryFailureNotice(tasks.error, "Scheduled tasks need a live API and are not part of the demo data set.")
        ) : tasks.data.length === 0 ? (
          <EmptyState icon={Timer} title="No scheduled tasks">
            The scheduler registry reported no tasks. Workers appear here once the API runs with scheduling enabled.
          </EmptyState>
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Interval</th>
                <th>Last Run</th>
                <th>Next Run</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {tasks.data.map((task) => (
                <tr key={task.id}>
                  <td className="cell-primary">
                    <span className="system-task-name">
                      {task.name}
                      {task.running ? <Badge tone="info">Running</Badge> : null}
                    </span>
                  </td>
                  <td className="cell-muted">{task.interval}</td>
                  <td>
                    <span className="system-task-lastrun" title={taskLastRunTitle(task)}>
                      {formatRelativeTime(task.lastRunAt)}
                      {task.lastError ? <StatusDot tone="danger" /> : null}
                    </span>
                  </td>
                  <td>{formatRelativeTime(task.nextRunAt)}</td>
                  <td>
                    <div className="cell-actions">
                      <IconButton
                        icon={Play}
                        size="sm"
                        label={`Run ${task.name} now`}
                        busy={task.running || (runTask.isPending && runTask.variables === task.id)}
                        disabled={runTask.isPending}
                        onClick={() => void triggerTask(task)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        )}
      </Card>

      <Card
        title="Disk Space"
        subtitle="Free space per library root and download path."
        padded={!diskSpace.data?.length}
      >
        {diskSpace.isPending ? (
          <LoadingRow label="Measuring disk space…" />
        ) : diskSpace.isError ? (
          queryFailureNotice(diskSpace.error, "Disk space needs a live API and is not part of the demo data set.")
        ) : diskSpace.data.length === 0 ? (
          <EmptyState icon={HardDrive} title="No disks reported">
            Configure library roots and a download client path so the API has locations to measure.
          </EmptyState>
        ) : (
          <DataTable>
            <thead>
              <tr>
                <th>Path</th>
                <th>Label</th>
                <th>Free</th>
                <th>Total</th>
                <th className="system-disk-usage-col">Used</th>
              </tr>
            </thead>
            <tbody>
              {diskSpace.data.map((disk) => {
                const used = diskUsedFraction(disk);
                return (
                  <tr key={disk.path}>
                    <td>
                      <code>{disk.path}</code>
                    </td>
                    <td className="cell-muted">{disk.label || "—"}</td>
                    <td>{formatBytes(disk.freeBytes)}</td>
                    <td>{formatBytes(disk.totalBytes)}</td>
                    <td>
                      <div className="system-disk-usage" title={`${Math.round(used * 100)}% used`}>
                        <ProgressBar value={used} tone={diskUsageTone(used)} />
                        <span className="system-meta">{Math.round(used * 100)}%</span>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </DataTable>
        )}
      </Card>

      <Card
        title="Readarr API compatibility"
        subtitle={compatibility.data?.summary ?? "Readarr-style API surface exposed by this instance."}
        actions={
          compatibility.data ? (
            <Badge tone={compatibility.data.status === "ready" ? "success" : "info"}>
              {titleize(compatibility.data.status)}
            </Badge>
          ) : undefined
        }
      >
        {compatibility.isPending ? (
          <LoadingRow label="Loading compatibility report…" />
        ) : compatibility.isError ? (
          <InlineNotice tone="danger">
            {compatibility.error instanceof Error
              ? compatibility.error.message
              : "Compatibility report failed to load."}
          </InlineNotice>
        ) : (
          <>
            <StatBar
              stats={[
                { label: "Compatible routes", value: compatibility.data.compatibleRoutes },
                { label: "Ready areas", value: compatibility.data.readyAreas, tone: "success" },
                {
                  label: "Partial areas",
                  value: compatibility.data.partialAreas,
                  tone: compatibility.data.partialAreas > 0 ? "warn" : "neutral"
                },
                { label: "Delegated areas", value: compatibility.data.delegatedAreas },
                { label: "Auth", value: compatibilityAuthLabel(compatibility.data.authMode) }
              ]}
            />
            {compatibility.data.categories.length === 0 ? (
              <EmptyState icon={ListChecks} title="No compatibility categories">
                The compatibility report contained no endpoint categories.
              </EmptyState>
            ) : (
              <div className="system-compat-grid">
                {compatibility.data.categories.map((category) => {
                  const isExpanded = Boolean(expandedCategories[category.id]);
                  const visibleExamples = isExpanded
                    ? category.examples
                    : category.examples.slice(0, VISIBLE_EXAMPLE_CHIPS);
                  const hiddenCount = category.examples.length - VISIBLE_EXAMPLE_CHIPS;
                  return (
                    <article className="system-tile system-compat-card" key={category.id}>
                      <div className="system-compat-head">
                        <StatusDot tone={compatibilityTone(category.status)} />
                        <strong>{category.title}</strong>
                        <Badge tone={compatibilityTone(category.status)}>
                          {compatibilityStatusLabel(category.status)}
                        </Badge>
                      </div>
                      <p className="system-muted">{category.message}</p>
                      <div className="system-compat-endpoints">
                        <span className="system-meta">
                          {category.endpointCount} endpoint{category.endpointCount === 1 ? "" : "s"}
                        </span>
                        {visibleExamples.map((example) => (
                          <code key={example}>{example}</code>
                        ))}
                        {hiddenCount > 0 ? (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() =>
                              setExpandedCategories((current) => ({
                                ...current,
                                [category.id]: !isExpanded
                              }))
                            }
                          >
                            {isExpanded ? "Show fewer" : `+${hiddenCount} more`}
                          </Button>
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
            <p className="system-meta system-notice-gap">
              Generated {formatRelativeTime(compatibility.data.generatedAt)}
            </p>
          </>
        )}
      </Card>
    </>
  );
}
