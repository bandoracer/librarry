import React, { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { CalendarDays, CalendarRange, ChevronLeft, ChevronRight, Copy, Link2 } from "lucide-react";
import {
  Button,
  Card,
  EmptyState,
  Field,
  FormGrid,
  IconButton,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import { calendarFeedURL, type CalendarItem } from "../../lib/api";
import { useCalendar } from "../../lib/queries";
import { demoModeEnabled } from "../../lib/demo";
import { formatDate } from "../../lib/format";
import "./calendar.css";

/* ------------------------------- Date math --------------------------------- */
/* Plain Date arithmetic only — no date libraries. Weeks start Monday. */

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, months: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
}

/** Monday-based weekday index: Mon=0 … Sun=6. */
function mondayIndex(date: Date): number {
  return (date.getDay() + 6) % 7;
}

/** Local YYYY-MM-DD (toISOString would shift days across timezones). */
function isoDate(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

function sameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

/** Full weeks (Mon–Sun) covering the month, as a flat list of days. */
function monthGridDays(monthStart: Date): Date[] {
  const gridStart = addDays(monthStart, -mondayIndex(monthStart));
  const monthEnd = addDays(addMonths(monthStart, 1), -1);
  const gridEnd = addDays(monthEnd, 6 - mondayIndex(monthEnd));
  const days: Date[] = [];
  for (let day = gridStart; day <= gridEnd; day = addDays(day, 1)) {
    days.push(day);
  }
  return days;
}

const weekdayLabels = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

function monthLabel(monthStart: Date): string {
  return monthStart.toLocaleDateString(undefined, { month: "long", year: "numeric" });
}

/* -------------------------------- Item tone -------------------------------- */

type CalendarTone = "success" | "danger" | "neutral";

const presentStatuses = new Set(["imported", "downloaded", "available", "present"]);

/** success = has a file, danger = monitored + missing, neutral = unmonitored. */
function itemTone(item: CalendarItem): CalendarTone {
  if (presentStatuses.has((item.status || "").toLowerCase())) return "success";
  return item.monitored ? "danger" : "neutral";
}

const toneToken: Record<CalendarTone, string> = {
  success: "var(--success)",
  danger: "var(--danger)",
  neutral: "var(--muted)"
};

const toneLabel: Record<CalendarTone, string> = {
  success: "in library",
  danger: "missing",
  neutral: "unmonitored"
};

function ToneDot(props: { tone: CalendarTone }) {
  return <span className="calendar-dot" style={{ background: toneToken[props.tone] }} aria-hidden />;
}

function itemTitle(item: CalendarItem): string {
  return `${item.title} — ${item.authorName} (${toneLabel[itemTone(item)]})`;
}

/* -------------------------------- iCal modal ------------------------------- */

function ICalModal(props: { open: boolean; onClose: () => void }) {
  const toast = useToast();
  const [pastDays, setPastDays] = useState(7);
  const [futureDays, setFutureDays] = useState(30);
  const feedURL = calendarFeedURL({ pastDays, futureDays });

  async function copy() {
    try {
      await navigator.clipboard.writeText(feedURL);
      toast.success("iCal feed URL copied to clipboard.");
    } catch {
      toast.error("Copy failed — select the URL and copy it manually.");
    }
  }

  return (
    <Modal
      title="iCal Feed"
      open={props.open}
      onClose={props.onClose}
      footer={<Button onClick={props.onClose}>Close</Button>}
    >
      <FormGrid columns={2}>
        <Field label="Past days" hint="How far back the feed includes releases.">
          <input
            type="number"
            min={0}
            value={pastDays}
            onChange={(event) => setPastDays(Math.max(0, Number(event.target.value) || 0))}
            aria-label="Past days"
          />
        </Field>
        <Field label="Future days" hint="How far ahead the feed includes releases.">
          <input
            type="number"
            min={0}
            value={futureDays}
            onChange={(event) => setFutureDays(Math.max(0, Number(event.target.value) || 0))}
            aria-label="Future days"
          />
        </Field>
      </FormGrid>
      <div className="calendar-feed-row">
        <Field
          label="Feed URL"
          hint="Subscribe from any calendar app. The URL embeds this browser's API key; open installs need no key."
        >
          <input readOnly value={feedURL} aria-label="iCal feed URL" onFocus={(event) => event.target.select()} />
        </Field>
        <Button icon={Copy} onClick={() => void copy()} title="Copy feed URL">
          Copy
        </Button>
      </div>
    </Modal>
  );
}

/* ---------------------------------- Page ----------------------------------- */

/**
 * Calendar: monitored books by release date. Classic arr month grid on wide
 * screens; an agenda list replaces it below 720px (CSS-only swap).
 */
export default function CalendarPage() {
  const [monthStart, setMonthStart] = useState(() => startOfMonth(new Date()));
  const [includeUnmonitored, setIncludeUnmonitored] = useState(false);
  const [icalOpen, setICalOpen] = useState(false);

  const days = useMemo(() => monthGridDays(monthStart), [monthStart]);
  const rangeStart = isoDate(days[0]);
  const rangeEnd = isoDate(days[days.length - 1]);
  const today = new Date();

  const calendar = useCalendar(rangeStart, rangeEnd, includeUnmonitored);
  const items = useMemo(() => calendar.data ?? [], [calendar.data]);

  const itemsByDay = useMemo(() => {
    const map = new Map<string, CalendarItem[]>();
    for (const item of items) {
      const key = (item.releaseDate || "").slice(0, 10);
      if (!key) continue;
      const bucket = map.get(key);
      if (bucket) {
        bucket.push(item);
      } else {
        map.set(key, [item]);
      }
    }
    return map;
  }, [items]);

  /** Agenda entries: visible-range days that actually have items, in order. */
  const agendaDays = useMemo(
    () =>
      days
        .map((day) => ({ day, key: isoDate(day), items: itemsByDay.get(isoDate(day)) ?? [] }))
        .filter((entry) => entry.items.length > 0),
    [days, itemsByDay]
  );

  return (
    <>
      <PageHeader
        title="Calendar"
        subtitle="Monitored books by release date."
        actions={<ToolbarButton icon={Link2} label="iCal Link" onClick={() => setICalOpen(true)} />}
      >
        <div className="calendar-toolbar">
          <div className="calendar-nav">
            <IconButton icon={ChevronLeft} label="Previous month" onClick={() => setMonthStart((current) => addMonths(current, -1))} />
            <Button size="sm" onClick={() => setMonthStart(startOfMonth(new Date()))}>
              Today
            </Button>
            <IconButton icon={ChevronRight} label="Next month" onClick={() => setMonthStart((current) => addMonths(current, 1))} />
            <span className="calendar-month-label">{monthLabel(monthStart)}</span>
          </div>
          <label className="calendar-toggle">
            <input
              type="checkbox"
              checked={includeUnmonitored}
              onChange={(event) => setIncludeUnmonitored(event.target.checked)}
            />
            <span>Include unmonitored</span>
          </label>
        </div>
      </PageHeader>

      {calendar.isError && !demoModeEnabled ? (
        <InlineNotice tone="danger">
          {calendar.error instanceof Error ? calendar.error.message : "Calendar refresh failed."}
        </InlineNotice>
      ) : null}

      <Card padded={false} className="calendar-card">
        {calendar.isPending ? (
          <LoadingRow label="Loading calendar…" />
        ) : (
          <>
            {/* Month grid (≥720px) */}
            <div className="calendar-grid" role="grid" aria-label={`Calendar for ${monthLabel(monthStart)}`}>
              {weekdayLabels.map((label) => (
                <div className="calendar-weekday" role="columnheader" key={label}>
                  {label}
                </div>
              ))}
              {days.map((day) => {
                const key = isoDate(day);
                const dayItems = itemsByDay.get(key) ?? [];
                const inMonth = day.getMonth() === monthStart.getMonth();
                const isToday = sameDay(day, today);
                return (
                  <div
                    role="gridcell"
                    key={key}
                    className={`calendar-day${inMonth ? "" : " calendar-day-outside"}${isToday ? " calendar-day-today" : ""}`}
                  >
                    <span className="calendar-day-number">{day.getDate()}</span>
                    <div className="calendar-day-items">
                      {dayItems.map((item) => (
                        <Link
                          key={`${item.wantedId}:${key}`}
                          className="calendar-item"
                          to={`/wanted?filter=all&item=${encodeURIComponent(item.wantedId)}`}
                          title={itemTitle(item)}
                        >
                          <ToneDot tone={itemTone(item)} />
                          <span className="calendar-item-title">{item.title}</span>
                        </Link>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Agenda list (<720px) */}
            <div className="calendar-agenda">
              {agendaDays.length === 0 ? null : (
                agendaDays.map((entry) => (
                  <section className="calendar-agenda-day" key={entry.key}>
                    <h3 className={sameDay(entry.day, today) ? "calendar-agenda-today" : undefined}>
                      {entry.day.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}
                    </h3>
                    {entry.items.map((item) => (
                      <Link
                        key={`${item.wantedId}:${entry.key}`}
                        className="calendar-agenda-item"
                        to={`/wanted?filter=all&item=${encodeURIComponent(item.wantedId)}`}
                        title={itemTitle(item)}
                      >
                        {item.coverUrl ? (
                          <img className="calendar-agenda-cover" src={item.coverUrl} alt="" loading="lazy" />
                        ) : (
                          <span className="calendar-agenda-cover calendar-agenda-cover-empty" aria-hidden>
                            <CalendarDays size={14} />
                          </span>
                        )}
                        <span className="calendar-agenda-text">
                          <strong>{item.title}</strong>
                          <small>{item.authorName}</small>
                        </span>
                        <ToneDot tone={itemTone(item)} />
                      </Link>
                    ))}
                  </section>
                ))
              )}
            </div>

            {items.length === 0 ? (
              <div className="calendar-empty">
                <EmptyState icon={CalendarRange} title={`Nothing scheduled in ${monthLabel(monthStart)}`}>
                  Monitored books with known release dates appear here. Add books from Add New or subscribe to authors
                  to start tracking upcoming releases.
                </EmptyState>
              </div>
            ) : null}
          </>
        )}
      </Card>

      <ICalModal open={icalOpen} onClose={() => setICalOpen(false)} />
    </>
  );
}
