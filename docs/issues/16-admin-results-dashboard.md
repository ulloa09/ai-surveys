# 16 — Admin Results Dashboard (Survey List, Detail, Analysis View)

## What to build

The admin-facing results and survey management UI. Admins see a list of their team's surveys with key stats, can drill into a survey's detail view, and access the full analysis output produced by the Analysis Engine. Super-admins see all surveys across all teams.

**Survey list view:**
- Title, status badge (draft/open/closed/analysing/complete/failed/archived), response count, completion rate, created date, owner
- Team badges per survey (a survey can be deployed to several teams — see below)
- Filter by status, search by title (both client-side over the loaded list)
- "Show archived" toggle (hidden by default; server-side, re-fetches with `include_archived=true`)
- Quick actions: activate, close, reopen, duplicate, archive, retry analysis (role-gated)

**Survey detail view:**
- Survey metadata + lifecycle controls (from #08)
- Response count, completion rate, average duration, expected/missing responses, coverage rate
- Per-team breakdown (one row per assignment)
- Download QR code button (PNG + SVG) — **one QR per team assignment**, not one per survey
- Link to analysis view (shown only when status is `analysing`, `complete`, or `failed`)
- Link to the raw responses view (available as soon as responses exist)

**Analysis view:**
- Per-question section, in survey question order
- Each section shows:
  - Question text and type
  - AI-generated written summary
  - Sentiment distribution bar (% positive / neutral / negative)
  - **Open-ended questions:** top topic clusters as a tag cloud, plus outlier responses listed separately with a flag icon
  - **Structured questions** (linear_scale, single_choice, …): answer distribution (count + % per option, by option *label*). No topic clusters and no outliers — a rating of 1 is a low rating, not an unusual response
- Survey-level stats at the top: total responses, completed, completion rate, average duration, language distribution
- `failed` renders any partial results plus an error notice and a retry button

Delivered:
- SvelteKit routes under `/dashboard/...` (not `/admin/...`): `/dashboard/surveys`, `/dashboard/surveys/[id]`, `/dashboard/surveys/[id]/analysis`, `/dashboard/surveys/[id]/responses`
- Backend endpoints:
  - `GET /api/surveys/stats` — aggregates for every visible survey (one batched call; avoids an N+1 from the list)
  - `GET /api/surveys/:id/stats` — aggregates for one survey (available in **any** status)
  - `GET /api/surveys/:id/analysis` — `analysis_results` + per-question answer aggregates
  - `GET /api/surveys/:id/responses` — raw per-respondent answers
  - `POST /api/surveys/:id/analysis/retry` — re-enqueue the Analysis Engine
- All views enforce team scoping and RBAC from #03

## Deviations from the original issue

These are intentional, and the acceptance criteria below reflect them:

- **Routes are `/dashboard/*`, not `/admin/*`.** The admin surface was built under `/dashboard`; `/admin` is only an API prefix.
- **There is no `viewer` role.** Migration `012_collapse_team_roles` collapsed the team roles to `profesor` and `alumno`. `profesor` is the read-only role the original criterion meant; `alumno` has no dashboard access at all.
- **A survey belongs to many teams, not one.** Phase 5 (`014_survey_assignments`) moved the team, public token, QR and response cap onto `survey_assignments`. So there is no single "team name column" — the list shows a team badge per assignment, and the detail view breaks stats down per team.
- **`analysing` is not the only pre-`complete` state.** `017_analysis_failed_status` added `failed`, so the analysis view is reachable in three states, not two.
- **Stats are team-scoped per viewer.** An `admin`/`super_admin` sees every team a survey is deployed to; a `profesor` sees only the assignments of teams they belong to. Known limitation: the *per-question* AI output (summary/sentiment/topics) is computed once per survey across all teams, so it is not team-scoped — only the counts are.

## Acceptance criteria

- [x] Survey list shows all surveys for the admin's teams, sorted by created date descending
- [x] Super-admin sees all surveys across all teams, with a team badge per survey
- [x] Status filter correctly limits the list to the selected status
- [x] Search by title filters the list
- [x] "Show archived" toggle reveals archived surveys (hidden by default)
- [x] Survey detail view shows response count, completion rate, average duration, and a per-team breakdown
- [x] Each team assignment exposes its own QR code, downloadable as PNG and SVG
- [x] Analysis view is inaccessible (link hidden + endpoint returns `404`) until status is `analysing`, `complete`, or `failed`
- [x] `GET /api/surveys/:id/stats` works in any status, including before the analysis has run
- [x] Each open-ended question section displays the AI summary, sentiment bar, topic clusters, and outlier list
- [x] Each structured question section displays the AI summary, sentiment bar, and the answer distribution by option label
- [x] Structured questions are never given outlier flags or topic clusters
- [x] Outlier responses are visually distinguished from regular responses (amber card, left border, flag icon)
- [x] Outliers and distributions display option *labels* ("Muy lento"), never internal values (`muy_lento`)
- [x] `profesor` (read-only role) can see the analysis view but cannot trigger lifecycle actions
- [x] `alumno` receives `403` on all survey list, detail and analysis endpoints
- [x] Non-team-member receives `403` on all survey detail and analysis endpoints
- [x] A `complete` survey can be re-analysed from the UI; the engine is idempotent and overwrites results
- [x] A `failed` survey shows partial results, an error notice, and a retry button

## Verification

Driven end-to-end against a live stack (Postgres + Go + SvelteKit) with a real LLM provider:
3 teams × 3 students each, 9 submitted responses to a 5-question survey (3 open-ended, 1 linear_scale,
1 single_choice), then closed. The engine ran `closed → analysing → complete`, wrote one
`analysis_results` row per question, and correctly flagged the two deliberately-planted off-topic
responses as outliers while leaving the low ratings unflagged.

## Blocked by

- #03 Teams & RBAC
- #15 Analysis Engine
