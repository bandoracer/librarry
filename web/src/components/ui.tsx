import React from "react";
import type { LucideIcon } from "lucide-react";
import { Loader2 } from "lucide-react";

/*
 * Librarry UI primitives.
 *
 * Every feature page composes these instead of writing bespoke markup so the
 * app keeps one visual language. Class names map to styles.css. Feature CSS
 * files may add layout-only classes prefixed with the feature name
 * (e.g. `.wanted-…`), but colors, spacing, and typography come from here.
 */

type Tone = "neutral" | "accent" | "success" | "warn" | "danger" | "info";

/* ---------------------------------- Page --------------------------------- */

export function PageHeader(props: {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <header className="page-header">
      <div className="page-header-row">
        <div className="page-header-text">
          <h1>{props.title}</h1>
          {props.subtitle ? <p>{props.subtitle}</p> : null}
        </div>
        {props.actions ? <Toolbar>{props.actions}</Toolbar> : null}
      </div>
      {props.children}
    </header>
  );
}

/** arr-style action strip rendered inside PageHeader or above tables. */
export function Toolbar(props: { children: React.ReactNode; align?: "start" | "end" }) {
  return <div className={`toolbar${props.align === "start" ? " toolbar-start" : ""}`}>{props.children}</div>;
}

export function ToolbarButton(props: {
  icon: LucideIcon;
  label: string;
  onClick?: () => void;
  disabled?: boolean;
  busy?: boolean;
  tone?: Tone;
  title?: string;
}) {
  const Icon = props.busy ? Loader2 : props.icon;
  return (
    <button
      type="button"
      className={`toolbar-button${props.tone && props.tone !== "neutral" ? ` tone-${props.tone}` : ""}`}
      onClick={props.onClick}
      disabled={props.disabled || props.busy}
      title={props.title ?? props.label}
    >
      <Icon size={15} className={props.busy ? "spin" : undefined} aria-hidden />
      <span>{props.label}</span>
    </button>
  );
}

/* --------------------------------- Buttons -------------------------------- */

export function Button(props: {
  children: React.ReactNode;
  onClick?: () => void;
  type?: "button" | "submit";
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md";
  disabled?: boolean;
  busy?: boolean;
  icon?: LucideIcon;
  title?: string;
}) {
  const { variant = "secondary", size = "md" } = props;
  const Icon = props.busy ? Loader2 : props.icon;
  return (
    <button
      type={props.type ?? "button"}
      className={`btn btn-${variant} btn-${size}`}
      onClick={props.onClick}
      disabled={props.disabled || props.busy}
      title={props.title}
    >
      {Icon ? <Icon size={size === "sm" ? 13 : 15} className={props.busy ? "spin" : undefined} aria-hidden /> : null}
      {props.children}
    </button>
  );
}

export function IconButton(props: {
  icon: LucideIcon;
  label: string;
  onClick?: () => void;
  disabled?: boolean;
  busy?: boolean;
  tone?: Tone;
  size?: "sm" | "md";
}) {
  const Icon = props.busy ? Loader2 : props.icon;
  return (
    <button
      type="button"
      className={`icon-btn${props.tone && props.tone !== "neutral" ? ` tone-${props.tone}` : ""}${props.size === "sm" ? " icon-btn-sm" : ""}`}
      onClick={props.onClick}
      disabled={props.disabled || props.busy}
      title={props.label}
      aria-label={props.label}
    >
      <Icon size={props.size === "sm" ? 13 : 15} className={props.busy ? "spin" : undefined} aria-hidden />
    </button>
  );
}

/* ---------------------------------- Cards --------------------------------- */

export function Card(props: {
  title?: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  padded?: boolean;
  className?: string;
}) {
  const { padded = true } = props;
  return (
    <section className={`card${props.className ? ` ${props.className}` : ""}`}>
      {props.title || props.actions ? (
        <div className="card-head">
          <div>
            {props.title ? <h2>{props.title}</h2> : null}
            {props.subtitle ? <p>{props.subtitle}</p> : null}
          </div>
          {props.actions ? <div className="card-head-actions">{props.actions}</div> : null}
        </div>
      ) : null}
      <div className={padded ? "card-body" : "card-body card-body-flush"}>{props.children}</div>
    </section>
  );
}

/* --------------------------------- Badges --------------------------------- */

export function Badge(props: { children: React.ReactNode; tone?: Tone; title?: string }) {
  return (
    <span className={`badge tone-${props.tone ?? "neutral"}`} title={props.title}>
      {props.children}
    </span>
  );
}

/** Compact horizontal stat strip (replaces the old full-width KPI tiles). */
export function StatBar(props: { stats: { label: string; value: React.ReactNode; tone?: Tone }[] }) {
  return (
    <div className="stat-bar" role="list">
      {props.stats.map((stat) => (
        <div className={`stat${stat.tone && stat.tone !== "neutral" ? ` tone-${stat.tone}` : ""}`} role="listitem" key={stat.label}>
          <span className="stat-value">{stat.value}</span>
          <span className="stat-label">{stat.label}</span>
        </div>
      ))}
    </div>
  );
}

/* --------------------------------- Tables --------------------------------- */

export function DataTable(props: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`table-wrap${props.className ? ` ${props.className}` : ""}`}>
      <table className="data-table">{props.children}</table>
    </div>
  );
}

/* ------------------------------ Feedback states --------------------------- */

export function EmptyState(props: { icon?: LucideIcon; title: string; children?: React.ReactNode; actions?: React.ReactNode }) {
  const Icon = props.icon;
  return (
    <div className="empty-state">
      {Icon ? <Icon size={28} aria-hidden /> : null}
      <h3>{props.title}</h3>
      {props.children ? <p>{props.children}</p> : null}
      {props.actions ? <div className="empty-state-actions">{props.actions}</div> : null}
    </div>
  );
}

export function InlineNotice(props: { tone?: Tone; children: React.ReactNode; onDismiss?: () => void }) {
  return (
    <div className={`inline-notice tone-${props.tone ?? "info"}`} role={props.tone === "danger" ? "alert" : "status"}>
      <span>{props.children}</span>
      {props.onDismiss ? (
        <button type="button" className="inline-notice-dismiss" onClick={props.onDismiss} aria-label="Dismiss">
          ×
        </button>
      ) : null}
    </div>
  );
}

export function LoadingRow(props: { label?: string }) {
  return (
    <div className="loading-row">
      <Loader2 size={15} className="spin" aria-hidden />
      <span>{props.label ?? "Loading…"}</span>
    </div>
  );
}

export function ProgressBar(props: { value: number; tone?: Tone }) {
  const clamped = Math.max(0, Math.min(1, props.value));
  return (
    <div className={`progress${props.tone && props.tone !== "neutral" ? ` tone-${props.tone}` : ""}`}>
      <div className="progress-fill" style={{ width: `${Math.round(clamped * 100)}%` }} />
    </div>
  );
}

/* ---------------------------------- Forms --------------------------------- */

export function Field(props: { label: string; hint?: string; children: React.ReactNode; htmlFor?: string }) {
  return (
    <label className="field" htmlFor={props.htmlFor}>
      <span className="field-label">{props.label}</span>
      {props.children}
      {props.hint ? <span className="field-hint">{props.hint}</span> : null}
    </label>
  );
}

export function FormGrid(props: { children: React.ReactNode; columns?: 1 | 2 | 3 }) {
  return <div className={`form-grid form-grid-${props.columns ?? 2}`}>{props.children}</div>;
}

/** Segmented control used for filters and mode toggles. */
export function Segmented<T extends string>(props: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
  ariaLabel: string;
}) {
  return (
    <div className="segmented" role="tablist" aria-label={props.ariaLabel}>
      {props.options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={props.value === option.value}
          className={props.value === option.value ? "active" : undefined}
          onClick={() => props.onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

/* ---------------------------------- Tabs ---------------------------------- */

export function TabNav(props: { tabs: { label: string; to: string; end?: boolean }[]; render: (tab: { label: string; to: string; end?: boolean }) => React.ReactNode }) {
  return <nav className="tab-nav">{props.tabs.map((tab) => props.render(tab))}</nav>;
}

/* ---------------------------------- Modal --------------------------------- */

export function Modal(props: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  footer?: React.ReactNode;
  wide?: boolean;
}) {
  if (!props.open) return null;
  return (
    <div className="modal-overlay" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && props.onClose()}>
      <div className={`modal${props.wide ? " modal-wide" : ""}`} role="dialog" aria-modal="true" aria-label={props.title}>
        <div className="modal-head">
          <h2>{props.title}</h2>
          <button type="button" className="icon-btn" onClick={props.onClose} aria-label="Close">
            ×
          </button>
        </div>
        <div className="modal-body">{props.children}</div>
        {props.footer ? <div className="modal-foot">{props.footer}</div> : null}
      </div>
    </div>
  );
}
