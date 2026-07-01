# Frontend Architecture

Last updated: 2026-07-01.

The web UI is a Vite + React 18 + TypeScript single-page app served as static
files (nginx in Docker, the Vite dev server locally). In July 2026 it was
migrated from a single 7,445-line `App.tsx` into a modular, feature-based
architecture with TanStack Query as the server-state layer.

## Layout

```
web/src/
  main.tsx                 Entry: router + app mount
  app/
    App.tsx                Route table (lazy-loaded pages), providers
    AppLayout.tsx          Shell: sidebar, mobile drawer, connection status
    nav.ts                 Navigation model (labels, icons, paths)
  components/
    ui.tsx                 Design-system primitives (see below)
    toast.tsx              ToastProvider / useToast
  lib/
    api.ts                 Typed API client (fetch + X-Api-Key header)
    queries.ts             TanStack Query client, shared hooks, query keys
    demo.ts                Demo-mode gate + seed fallback wrapper
    seed.ts                Seed data for demo installs
    format.ts              Shared formatting helpers
  features/
    dashboard/  library/  search/  wanted/
    activity/   imports/  system/  settings/
  styles.css               Design tokens + primitive styles (single source)
```

Conventions:

- **Pages own their data.** Each feature page fetches through hooks in
  `lib/queries.ts` (shared lists) or feature-local `useQuery` calls. No state
  is threaded through the shell.
- **Primitives, not bespoke markup.** Pages compose `PageHeader`, `Toolbar`,
  `Card`, `DataTable`, `StatBar`, `Badge`, `Modal`, `EmptyState`,
  `InlineNotice`, form primitives, and toasts from `components/ui.tsx`.
  Feature CSS files contain layout-only rules and are prefixed with the
  feature name.
- **Colors/spacing/typography** come exclusively from CSS custom properties in
  `styles.css` (light and dark themes; `data-theme` on `<html>`, persisted at
  `librarry-theme` in localStorage).
- **Mutations toast their outcome** (success and failure). Persistent fetch
  failures render as inline notices; empty datasets render `EmptyState` with a
  next action, never a blank panel or a raw `refresh failed: 500` banner.
- **Cross-page flows use URL contracts**, not shared state:
  - `/search?query=<text>&mode=book|author` prefills and auto-runs a search.
  - `/wanted?item=<id>` selects a wanted item; `&search=1` also triggers a
    release search; `?filter=missing|review|wanted|grabbed|all` sets the list
    filter; `?tab=authors` opens author subscriptions.
  - `/downloads?state=failed` (and other states) pre-filters the queue;
    `/downloads/history` opens the History tab.
  - `/settings`, `/settings/media`, `/settings/profiles`,
    `/settings/connections`, `/settings/import` address the settings tabs.

## Navigation

Route paths are stable API (bookmarks, reverse-proxy rules, deployed E2E
flows). The 2026-07 redesign changed labels and ordering only:

| Path         | Label          | Notes                                        |
| ------------ | -------------- | -------------------------------------------- |
| `/dashboard` | Dashboard      | Needs-attention queues, pipeline, health     |
| `/library`   | Library        | Default route; authors + monitored books     |
| `/search`    | Add New        | Metadata search → add book / monitor author  |
| `/wanted`    | Wanted         | Books + Authors tabs, releases, provenance   |
| `/downloads` | Activity       | Queue + History tabs                         |
| `/imports`   | Library Import | Scans, manual import, import reviews         |
| `/providers` | System         | Setup checklist, health, Readarr compat      |
| `/settings`  | Settings       | General/Media/Profiles/Connections/Import    |

Responsive behavior: full sidebar ≥1100px, icon rail 720–1099px, hamburger +
overlay drawer <720px. The pre-migration UI hid most navigation on small
screens; treat nav reachability as a regression test when touching the shell.

## Server state

`lib/queries.ts` owns the `QueryClient`, query keys, and polling cadence
(downloads 10s; wanted/acquisition/history/import reviews 30s; health 60s).
Anything that writes goes through `useMutation` (or the
`useInvalidatingMutation` helper) and invalidates the shared keys so every
open page converges without manual refresh plumbing.

## Demo mode

`VITE_LIBRARRY_DEMO=true` (build-time) enables demo mode:

- `withDemoFallback(fetcher, seed)` returns seeded data when a request fails,
  so screenshots/demos work without a backend and without error walls.
- Outside demo mode there is **no** silent fallback — failures surface as
  inline notices. Real deployments must never render fake data.
- The sidebar connection indicator shows Connected / Demo data / Offline from
  a polled `system/status` probe.

Demo seeds live in `lib/seed.ts` and only cover: search results, wanted items
(+ metadata provenance for seeded IDs), downloads (+details/resources/
preferences), provider/integration health, and the Readarr compatibility
report. Author subscriptions, import reviews, library files, history, and the
acquisition queue have no seeds and render their (real) empty states in demo
mode.

## Build

```
cd web
npm run dev     # Vite dev server on 5173, /api proxied to 127.0.0.1:8080
npm run build   # tsc -b && vite build → web/dist
```

Pages are lazy-loaded (`React.lazy`) so each feature is its own chunk.
