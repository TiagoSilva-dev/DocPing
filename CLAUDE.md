# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
npm ci
npm run dev          # Go API (:8080, JSON store) + Vite (:5173) together via scripts/dev.mjs
npm run dev:web      # Vite only (expects an API; set API_PROXY_TARGET if not on :8080)
npm run check        # tsc -b for apps/web
npm run build        # tsc -b && vite build
npm test             # vitest run (apps/web/src/**/*.test.ts)
npm run test:e2e     # Playwright (apps/web/e2e); needs the real stack running
```

Go (from `apps/api`):

```sh
go test ./...
go test -race ./...
go vet ./...
go test ./internal/app -run TestName -v                                   # single test
MYSQL_TEST_DSN='<disposable-db-dsn>' go test ./internal/persistence -run MySQL -v   # opt-in
```

Full local stack (Docker: MySQL, MinIO, Mailpit) and the real-infrastructure
smoke test:

```sh
cp -n .env.example .env   # replace change-me values
docker compose up --build -d
docker compose --profile web up --build -d          # also serve the web app via Nginx
node scripts/integration-smoke.mjs --mysql          # 17 scenarios against real services
```

Vitest is `environment: node` — the unit tests cover pure logic (CSV parsing, date
math, notification derivation), not components. Playwright projects are filtered by
`@desktop` / `@mobile` tags in the spec titles.

`npm run dev` forces `APP_ENV=development STORE_DRIVER=json`.
Restart it after changing Go; Vite hot-reloads the frontend. Use `localhost:5173` exactly —
the server validates the request origin against `WEB_ORIGIN`.

## Architecture

Modular Go monolith (`apps/api`) + React/TS SPA (`apps/web`). No microservices, no Redis,
no external queue. Scheduled jobs and the document-analysis queue run in-process in the
same binary.

### API request pipeline

`internal/app/server.go` is the spine. Every route is registered through
`a.handle(mux, pattern, write, auth, fn)`, which wraps the handler in:

1. `security` middleware — security headers, strict `Origin`/`Sec-Fetch-Site` check,
   per-IP rate limiting by route group (`auth` 20/min, `upload` 20/min, `portal` 120/min,
   `api` 600/min), panic recovery.
2. A single `Store.Run(ctx, write, fn)` transaction. Handlers receive a `*call` holding
   the whole mutable `domain.State` plus the resolved `User`/`Session`/`Org`.
3. `authenticate` — `docping_session` cookie hashed into `State.Sessions`; write requests
   additionally require a constant-time-compared `X-CSRF-Token`.
4. Post-commit side effects: handlers append to `c.effects`, which run **after** the
   transaction commits. All external I/O (SMTP in particular) belongs there, never inside
   the transaction. See `prepareDelivery` in `jobs.go`: it persists a `pending` delivery,
   then the effect records the provider's true result.

Handlers return `(any, error)`; errors are `*APIError` created with
`fail(status, code, message)` / `invalid(msg)` / `missing()`. Messages are user-facing
**Portuguese**; unknown errors are logged without detail and returned as `internal_error`.
Three routes bypass `a.handle` because they stream or do work outside a transaction:
`uploadHTTP`, `file`, `export`.

### Persistence

`internal/persistence/store.go` exposes one abstraction: load the entire `domain.State`
into memory for the duration of a transaction, let the handler mutate it, write it back.

- `STORE_DRIVER=mysql` (production): read-only or `GET_LOCK`-serialized repeatable-read
  transaction; tables carry relational ownership/uniqueness columns plus a `payload JSON`
  aggregate snapshot. Migrations in `apps/api/migrations/001_initial.sql` are embedded and
  applied at startup.
- `STORE_DRIVER=json` (development only, enforced): atomic temp-file+rename write to
  `apps/api/.data/state.json`, objects under `apps/api/.data/objects/`.

`save` is upsert-only, so dropping an entity from the in-memory state does not
delete it — the next load brings it back. Deletion therefore does not go through
`Run`: `Store.PurgeOrganizations` issues real `DELETE`s child-first (the schema
has no `ON DELETE CASCADE`, so `purgeOrder` is load-bearing). It is the only
deletion path, used by the cleanup job in `internal/app/cleanup.go` that expires
demo and unconfirmed organizations; bucket objects are deleted by the caller
first, because afterwards nothing records which objects were theirs.

This whole-state load is a deliberate, documented first iteration — see the "Desvio
conhecido" section in `docs/02-arquitetura-e-dados.md`. Scoped SQL queries must replace it
before scale; do not add features that depend on loading all state.

### Files and uploads

`internal/storage/storage.go` wraps private S3/MinIO (or a local dir in development). The
DB stores only opaque key, SHA-256, size, MIME and `scanStatus`; bytes never touch MySQL
and objects are never public. Uploads read the body and detect the type by content *and*
extension (`detectFile`: PDF/JPG/PNG/HEIC only) **outside** the transaction; the commit
re-checks identity, token validity, item state, content deduplication and quotas, and
orphan objects are cleaned up on failure.

There is **no malware scanning**. ClamAV was removed deliberately — it cost more RAM than
the rest of the stack together — so uploads are stored as `domain.ScanUnscanned`
(`"unscanned"`) and what guards the upload is the type check, the size limit, the private
bucket and the session. `scanStatus` survives because stored files still carry `clean` or
`unscanned_development` from when the scanner existed, and `readableFile` is the one rule
that decides what may be opened: an allow-list, so no legacy quarantine value reads as
readable. Do not reintroduce a claim of antivirus anywhere user-visible without
reintroducing a scanner.

Production also demands the provider confirm SSE-S3 (`AES256`) on each object, deleting
it and failing when it does not. `S3_ACCEPT_UNCONFIRMED_ENCRYPTION=true` is a deliberate,
operator-set opt-out for providers that encrypt at rest without reporting it — Cloudflare
R2 leaves the header unimplemented and Railway's bucket omits it. It is never the default,
it turns a checked fact into trust in the provider, and the server logs a warning at every
start while it is on.

### Automated document analysis

`internal/analysis` is a leaf package with no persistence: `Extractor` (Docling, with
PaddleOCR as the fallback when under 240 characters come back), `Jev` (TypeSafe System
One), and `Decide`, which composes the verdict in **code** from six typed judgements.
`internal/app/analysis.go` owns the in-memory queue (2 workers, 256 slots) and the write
transaction that stores — and in `auto` mode applies — the verdict.

- Off unless `AI_REVIEW_ENABLED=true`. Then every upload still goes to a person.
- All extraction and inference happens outside any transaction, like SMTP. The upload
  commit only writes `analysis.status=queued`; enqueueing happens after it returns.
- The model judges, `Decide` decides. A missing answer, a failed service or a
  probability near 0.5 always lands on `review` — never on an automatic approval.
- `saveAnalysis` re-checks everything inside the write transaction: a person who
  reviewed the item while the worker ran always wins, and the late verdict is recorded
  with `applied:false`.
- `domain.PortalRequest` strips the analysis and the reviewer id — use it for every
  portal response, including the upload one. `PublicRequest` is for office responses.
- Thresholds are configuration and are uncalibrated. Do not present `auto` as validated;
  see `docs/06-validacao-da-entrega.md`.

### Account creation is rate-limited and confirmed

Both routes that create an organization without anyone paying or proving
anything are bounded, because an unbounded number of them is an unbounded
amount of state in a store that loads all of it per transaction.

- `register` does **not** return a session. It answers
  `{status:"verification_sent",email}` and mails a single-use 24h link;
  `verify-email` confirms the address and opens the session. `login` returns
  `403 email_unverified`, checked only after the password matched so the
  response never reveals an account's state to someone without its credentials.
  `User.EmailPending` gates this and its zero value means "not pending", so
  accounts stored before the field keep working with no data migration.
- `Config.VerifyEmail` is derived from SMTP being configured, never set by
  hand: production already requires SMTP, and a development machine without it
  must stay usable — there, registration opens the workspace as it did before.
- `/api/auth/demo` has its own hourly rate-limit group and a ceiling on live
  demonstrations. Expiry (`DEMO_TTL_HOURS`, `UNVERIFIED_TTL_HOURS`) is what
  keeps them from accumulating; a confirmed workspace is never collected.

### Config is the safety boundary

`internal/app/config.go` refuses to start when `APP_ENV=production` lacks HTTPS origin,
verified-TLS MySQL, private S3 or a way to send e-mail, or when any development bypass is
set. Mail has two transports and production needs one of them, not a specific one:
`RESEND_API_KEY` sends over HTTPS and wins when set, SMTP stays for the local
Mailpit stack, and `Config.Sender()` resolves `MAIL_FROM` falling back to
`SMTP_FROM`. HTTPS exists because a platform can block the SMTP port outright —
Railway does below Pro, and the dial just times out — so a product whose only
transport is SMTP can go silent without a single error from the provider.
`SMTP_TLS_MODE` picks how the mail connection reaches TLS — `starttls` (port 587)
or `implicit` (port 465, TLS before the greeting, which is what Hostinger recommends);
blank means the port decides. Both verify the certificate against the system roots and
the configured hostname, and an unknown mode is refused rather than treated as
plaintext. Verified TLS means `tls=true` against the system roots, or `tls=docping-ca` with a
PEM in `MYSQL_CA_CERT` — same chain and hostname check, different trust anchor, and
refused outright when the CA is missing or unparsable. `tls=skip-verify` stays rejected. It also refuses to start in production without SMTP, since a new account could
then never confirm its address. It likewise refuses `AI_REVIEW_ENABLED=true` without `TYPESAFE_API_KEY`, without an
extractor URL, with a non-HTTPS TypeSafe endpoint, or with a confidence threshold outside
0.5–1 — a threshold a typo turned into zero would approve every document. There is no
automatic fallback from MySQL to JSON. Preserve this: new capabilities that weaken
production must be rejected in `LoadConfig`, not defaulted.

### Subscriptions are off until a key exists

`internal/billing` is a leaf like `internal/analysis`: a hand-written Stripe
client (three POSTs, one GET, one HMAC) with no persistence and no domain
knowledge. The official SDK is not used — the surface is small and this
repository builds MinIO from source over supply-chain concern — so the API
version is pinned in `APIVersion` by hand and `Verify` is the most tested
function in the package.

- Nothing charges without `STRIPE_SECRET_KEY`: no routes, no gate, and a new
  workspace is born `founder_pilot`, exactly as before money existed. With a
  key, `LoadConfig` demands the webhook secret and all three prices, and
  production refuses a test key.
- The three billing routes bypass `a.handle` for the reason upload and export
  do: each one calls Stripe, and an HTTP call to another company must not hold
  a transaction open. The webhook has no session, no CSRF and no Origin — what
  protects it is the signature, and nothing else.
- Access follows Stripe's own status. `active`, `trialing` and `past_due` are
  open; a failed card is still being retried and is not a reason to lock an
  office out. The gate lives in `authenticate`, so it covers every
  authenticated route at once; `billingExempt` lists what the subscribe screen
  needs. The client portal is never gated — whoever uploads a document is not
  whoever stopped paying.
- The webhook finds the organization by Stripe **customer id**, not by
  metadata, and ignores an event older than `BillingUpdatedAt`: deliveries
  repeat and arrive out of order, and a stale cancellation would close a
  workspace that just paid. An unknown price records the status and does not
  pick a plan.
- What a plan sells is **clients**, and only that: `domain.entitlements` holds
  the same three figures `apps/web/src/lib/plans.ts` prints (20/60/150).
  Change one and change the other in the same commit, or the pricing page
  becomes a brochure again. `domain.EffectiveClientLimit` reads the limit from
  the plan on every check instead of copying it into the organization, so an
  upgrade takes effect without touching stored data; `Organization.ClientLimit`
  still answers for `founder_pilot` and `demo`, which buy nothing.
- Active requests and storage are **ceilings, not a product boundary**. They
  are derived in `sold()` from the client figure — six open requests and two
  gigabytes per client — so they grow with the plan and no single closing can
  reach them. Nothing on the pricing page mentions them, and that is the
  point: a ceiling a paying office can hit is a limit nobody told it about.
  Raise the clients a plan sells and the room follows; never set one by hand
  near the sold figure.

### Domain rules

`internal/domain/models.go` holds the state machine and time rules.

- Item status: `pending` → `uploaded` → `approved` | `correction_required`.
  Request status is **derived**, never set directly: call `domain.Recalculate(&r)` after any
  item change (`completed` only when every required item is approved; `cancelled` is sticky).
- `Organization.AIReview` is `off`/`suggest`/`auto`; an organization stored before the
  feature reads as `suggest` via `domain.NormalizeAIReview` — existing workspaces never
  start deciding on their own. `Review.Source == "jev"` marks an automated decision and
  carries no `UserID`.
- A request snapshots its template items at creation time; editing a template must not
  change existing requests.
- Timestamps persist as UTC RFC3339 (`domain.Now()`). Competence is `YYYY-MM`, due dates are
  civil dates, recurrence runs in `America/Sao_Paulo` via `MonthDay`/`NextExecution`
  (day 31 clamps to the last day of short months — never add 30 days).
- Recurrence is idempotent on schedule+client+competence; reminders reserve a delivery
  before sending.
- `domain.PublicRequest` strips `ObjectKey`/`SHA256` — use it for every portal response.
- Every org-scoped entity carries `OrgID`; look requests up with `c.ownRequest(id)` and
  never trust a client-supplied organization.

### Frontend

`App.tsx` owns routing (React Router) with every page lazy-loaded; `ProtectedLayout`
gates on the `["session"]` TanStack Query. `lib/api.ts` is the only HTTP surface: it
prefixes `/api`, sends `credentials: "same-origin"`, captures `csrfToken` from any
response into a module-level variable and replays it as `X-CSRF-Token`, and throws
`ApiError(code, message, status)`. Uploads use `XMLHttpRequest` (`uploadFile`) for real
progress. TanStack Query owns remote data; React Hook Form + Zod validate auth forms only —
everything else relies on authoritative server validation. Path alias `@/` → `apps/web/src`.

Styling is Tailwind v4 (`@tailwindcss/vite`, no config file) with CSS custom properties
declared in `src/styles.css` and per-page `*.css` files. The chrome is neutral — black,
white and grey — and colour carries meaning only, through the tokens in `:root`:

- `--accent` (indigo) marks automated analysis and the recommended plan. It is the one
  non-status colour, and it never spreads to ordinary buttons, headers or navigation.
- `--ok` / `--warn` / `--danger` / `--info` mark state: approved, correction, rejected or
  error, and under review. Each has a `-soft` background, a `-border`, and an `-on-dark`
  variant for the landing's dark section.

Every token pair was checked at 4.5:1 against both white and its own soft background; keep
new pairs at that bar. Colour is never the only signal — pair it with an icon, a dot, a
label or a written number, as `AnalysisPanel` does by printing the confidence it also
draws. Use the tokens rather than new hexes, and do not give the chrome a brand hue.
`components/ui/` holds shadcn/EasyUI primitives installed via the registry and then
customized — prefer extending them over pulling new dependencies.

### Conventions

- Code, comments and docs-in-code are English; all user-visible strings and `docs/` are
  Portuguese (pt-BR).
- `docs/api-contract.md` and `apps/api/openapi.json` document the implemented contract —
  update them when routes change.
- The README and `docs/06-validacao-da-entrega.md` are deliberately precise about what has
  and has not been validated (Playwright/visual validation and production infra are still
  pending). Do not upgrade those claims without running the checks.
