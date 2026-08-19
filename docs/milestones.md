# Milestones

---

## Phase 1 — Foundation & Admin Tooling

### Overview

Phase 1 establishes the complete platform foundation and delivers a fully functional admin experience. By the end of this phase, university staff can log in with their institutional account, form teams, create richly configured surveys with all question types and AI conversation modes, manage the survey lifecycle with scheduling and response caps, and distribute surveys via shareable links and QR codes.

No AI conversation happens in this phase — the respondent-facing experience is limited to the landing page state checks (active, closed, not found). But every admin-side decision from the design is implemented, tested, and ready for Phase 2 to build on top of.

This phase is also the riskiest from an infrastructure standpoint: OAuth integration with the university identity provider, database migrations, CI/CD setup, and RBAC correctness all need to be solid before anything else can proceed.

### Demo at Completion

An admin logs in with their ITESO email, creates a team with a colleague, and builds a hybrid Mode C survey for the week 12 semester check-in. The survey has 8 questions across 4 types (open-ended, linear scale, single-choice, true/false), with AI follow-up enabled on the open-ended ones and disabled on the structured ones. They write a system prompt in Spanish, set the termination to 14 turns, enable both Spanish and English, schedule the survey to open on week 12 Monday and close on Friday, and set a response cap of 300. They activate it, copy the shareable link, download the QR code as PNG and SVG, and share both with their class. The survey landing page loads on a phone and shows the correct "coming soon" or "survey closed" state depending on the schedule.

### Issues

#### #01 — Project Scaffold
Bootstrap the monorepo: SvelteKit frontend, Go backend, PostgreSQL with a migration runner, `docker-compose.yml` for local dev, a `GET /api/health` endpoint, a single SvelteKit page that renders health status, and a CI pipeline that runs `go vet`, backend tests, and frontend type-check on every push. The goal is that a new developer can clone the repo, run `docker-compose up`, and have a working local environment with zero manual steps beyond copying `.env.example`.

#### #02 — Institutional OAuth
OAuth 2.0 login using the university email provider. On first login, a user record is created with the `admin` role by default. Emails listed in the `SUPER_ADMIN_EMAILS` environment variable receive `super_admin` on first login. The Go backend exposes protected routes under `/api/admin/*` that reject unauthenticated requests. SvelteKit renders a login page and a minimal admin shell (nav + "logged in as X" header). Logout clears the session.

#### #03 — Teams & RBAC
Admins can create teams and invite colleagues by institutional email. Within a team, members can be `admin` (full management) or `viewer` (read-only results). Survey endpoints are scoped to the owning team — a user outside the team gets `403`. Super-admins bypass team scoping and see everything. Viewers get `403` on edit and export endpoints. The admin shell gains a team management UI.

#### #04 — Survey CRUD & Duplicate
Full create/read/update/delete for surveys, plus a duplicate button that deep-copies all config (and questions, once they exist) into a new `draft`, suffixed with "(copia)". The `surveys` table holds all configuration fields: title, description, owner, team, status, anonymity level, allow-revisit toggle, optional-registration toggle. Anonymity level is locked at the DB layer once the first response row exists — a trigger rejects the update with a clear error. Deleting a survey with responses is also rejected. The admin UI has a survey list page, create/edit forms, delete confirmation, and a duplicate button.

#### #05 — Question Editor
All seven question types are available: `open_ended`, `single_choice`, `multi_choice`, `true_false`, `linear_scale`, `ranking`, `matrix`. Each question has a required/optional toggle (required by default) and an AI follow-up toggle (on by default for `open_ended`, off by default for structured types). Questions can be reordered via drag-and-drop. Editing or deleting questions after the first response is blocked with a clear error message. The duplicate operation from #04 deep-copies all questions.

#### #06 — Survey Mode, System Prompt & Termination Config
Mode selector with three options — each rendered as a card with a one-sentence description and a concrete Spanish-language example. Mode C (Hybrid) is the default. The system prompt textarea is required for Mode B and C, optional for Mode A. Switching to Mode B hides the question list with an explanatory note. Termination panel shows all four modes with plain-language explanations; `turn_limit` is the default with 12 exchanges pre-filled. Combination mode reveals all three inputs simultaneously with a note that the first trigger wins.

#### #07 — Language Configuration
A language panel on the survey config page with a checkbox list (Spanish, English) and a default language selector. Spanish is pre-checked and is the default. The admin cannot deselect all languages. When only one language is configured, no language selector will be shown to respondents. When multiple are configured, the respondent picks on the landing page. The selected language is stored on the response record and passed to the AI adapter as a language instruction in Phase 2.

#### #08 — Survey Lifecycle & Access Links
Surveys move through `draft → active → closed → archived`. Manual transitions via buttons with confirmation dialogs. A background job (polling or pg_cron) auto-activates surveys when `opens_at` is in the past and auto-closes when `closes_at` is in the past or `response_cap` is reached — whichever fires first. On first activation, a permanent `public_token` UUID is generated and never changes. A QR code (PNG + SVG) is generated server-side pointing to `/s/<public_token>`. The lifecycle panel in the admin UI shows the current status badge, transition buttons, date-time pickers, and the response cap input. QR code is downloadable from the UI. The public route `/s/<token>` returns a 404 for draft/archived surveys and a "survey closed" page for closed ones — no conversation UI yet.

### Phase 1 Acceptance Criteria

- [ ] Developer can run the full stack locally with `docker-compose up` and zero manual steps
- [ ] CI pipeline passes on a clean checkout and blocks merge on failure
- [ ] Admin can log in with an institutional email and is assigned the correct role
- [ ] Super-admin bootstrap works via `SUPER_ADMIN_EMAILS` env variable
- [ ] Admin can create a team, invite a colleague, and assign them admin or viewer role
- [ ] Team scoping is enforced: cross-team survey access returns `403`
- [ ] Super-admin can access and manage all surveys across all teams
- [ ] Admin can create a survey with all 7 question types, all survey modes, all termination modes, language config, and lifecycle settings
- [ ] Anonymity level is immutable after the first response at the DB layer
- [ ] Survey lifecycle transitions work manually and automatically (schedule + cap)
- [ ] Shareable URL and QR code (PNG + SVG) are generated and downloadable
- [ ] Public survey URL shows the correct state page (not found / survey closed)
- [ ] All admin endpoints return `403` for unauthenticated requests
- [ ] Viewer role cannot access edit or export endpoints

### Risks & Notes

- The OAuth integration depends on the university identity provider supporting standard OAuth 2.0. Confirm the provider (Azure AD, Google Workspace, custom SAML) with IT before starting #02.
- The DB migration runner must support rollbacks — choose a Go migration library (e.g. `golang-migrate`) that supports both up and down migrations before writing any schema.
- The `public_token` immutability must be enforced at both the application and DB layer. A DB-level trigger or generated column is safer than relying on application code alone.

---

## Phase 2 — AI Conversation Experience

### Overview

Phase 2 delivers the core value proposition: an AI-guided interview that feels like a real conversation. A respondent scans a QR code or follows a link, lands on a branded page, selects their language, and completes a hybrid chat/form interview — open-ended questions as chat bubbles with streaming AI responses, structured questions as native form elements (sliders, radio buttons, checkboxes). The AI probes deeper on flagged questions, respects the termination configuration, and ensures all required questions are covered before allowing submission.

This phase also introduces anonymous identity and resume logic for both authenticated and unauthenticated users, and gives super-admins a provider settings UI to configure which AI provider is active, its API key, and the model in use.

Phase 2 is the phase where the product either works or it doesn't. The streaming experience, the quality of the AI's follow-up questions, and the smoothness of the hybrid UI on mobile are the things that will determine whether this gets adopted. Build carefully and test on real devices early.

### Demo at Completion

A student receives a link to the week 12 survey in WhatsApp. They open it on their phone, see the ITESO-branded landing page with the survey title, estimated time (10 min), "Tus respuestas son confidenciales" anonymity declaration, and a "Comenzar" button. They start the interview. The AI greets them in Spanish and asks the first open-ended question about the semester. The student types a short answer; the AI probes with a follow-up. The student then rates their workload on a 1–5 slider — it renders as a proper slider, not a chat prompt. They answer 6 more questions, some probed by the AI, some not. Halfway through they close the browser. An hour later they return on a different device, log in with their institutional email, and see "Continuar donde lo dejaste." They resume exactly where they left off. They complete the survey and submit. A super-admin, meanwhile, opens the platform settings page, sees Claude is the active provider with a masked API key, and clicks "Test connection" — it succeeds.

### Issues

#### #09 — Anonymous Identity & Resume Tokens
Two non-OAuth identity models. Pseudonymous: an HMAC-SHA256 hash of a stable device fingerprint (user-agent + accept-language + server-side salt), non-reversible, stored on the response record, used to detect and prevent duplicate submissions on `pseudonymous` surveys. Truly anonymous: no fingerprint stored at all, every visit creates a new response. Resume tokens: UUID v4 issued by the server when a pseudonymous respondent starts a survey, returned in the response body, stored in `localStorage` under `resume_<public_token>`, expire when the survey closes. The fingerprint hash must be deterministic (same inputs → same hash always) and non-reversible (no endpoint or code path maps it back to device details).

#### #10 — Landing Page & QR Access
The public entry point at `/s/[token]`. Renders the survey title, description, estimated duration (if set), anonymity declaration in plain language, language selector (when multiple languages are available), and a "Comenzar" / "Start" button. Optional registration form (name + email) when enabled by the admin, with a clear note that this breaks full anonymity. Handles all survey states: 404 for draft/archived, "survey closed" for closed, landing page for active. Detects resume: pseudonymous users with a valid localStorage token see "Continuar donde lo dejaste"; authenticated users are handled in #14. Clicking Start creates the response record on the backend and navigates to the conversation route. Fully responsive — optimised for mobile since QR scans happen on phones.

#### #11 — AI Provider Adapter
The strategy-pattern core of the AI layer. Defines the `AIProvider` Go interface with `StreamTurn` and `Extract` methods. The Claude (Anthropic) implementation uses the Anthropic Go SDK with streaming enabled. Context assembly injects the admin's system prompt, language instruction, ordered question list with types and follow-up flags, full conversation transcript, and current question coverage state into every request. Prompt injection hardening: student responses are passed only as `user`-role messages and never interpolated into the system prompt. SSE endpoint `GET /api/responses/:id/stream` streams tokens to the client. A `ProviderRegistry` manages active provider selection. Platform settings UI (super-admin only) at `/admin/settings`: provider selector with descriptions, masked API key input, model selector per provider, test-connection button. A dev-only `POST /api/dev/test-ai` endpoint allows quick verification without a full survey. API keys stored encrypted in `platform_settings`.

#### #12 — Survey Engine (Conversation State Machine)
The orchestration brain. Manages the full response lifecycle (`not_started → in_progress → pending_submission → submitted → analysing → complete`). On each turn: appends the human message to the transcript, determines the active question, decides whether to probe or advance (based on `ai_followup` flag and whether a structured answer has been recorded), calls the AI Provider Adapter, appends the AI response, evaluates all active termination conditions, and transitions to `pending_submission` when any trigger fires. For structured question types, the engine parses and validates the respondent's answer before storing it. Partial submission gate: if termination fires before all required questions are covered, the AI is instructed to cover the remaining required questions before the session ends. `POST /api/responses/:id/submit` transitions to `submitted` only when all required questions have answers. Emits a `survey.submitted` event consumed by the Analysis Engine in Phase 3. Fully unit-tested with no external LLM calls — the AI adapter is mocked in tests.

#### #13 — Respondent Conversation UI
The hybrid chat/form interface at `/s/[token]/conversation`. Open-ended questions render as chat bubbles — AI message on the left, respondent input at the bottom, AI responses stream token-by-token via SSE. Structured questions render as native components: radio buttons for `single_choice`, checkboxes for `multi_choice`, two large buttons for `true_false`, a labelled slider for `linear_scale`, drag-to-reorder for `ranking`, a grid for `matrix`. A typing indicator ("...") shows while waiting for the first streaming token. Progress bar shows required questions covered out of total required. When `allow_revisit` is enabled, previously answered questions show an "Editar" button that scrolls back, allows a new answer, and re-sends the full updated context to the engine before continuing. Submit button appears when all required questions are covered. Confirmation dialog when optional questions remain unanswered. Thank-you screen after successful submission. Network loss shows a "conexión perdida, reintentando..." indicator and auto-reconnects to the SSE stream.

#### #14 — Server-side Resume (Authenticated Users)
When an authenticated user returns to the landing page of an active survey they have already started, the backend detects their in-progress response by user identity and returns it via `GET /api/surveys/:id/my-response`. The landing page shows a "Continuar donde lo dejaste" button alongside a "Comenzar de nuevo" option (starting fresh requires confirmation and marks the previous response as `abandoned`). `GET /api/responses/:id` returns the full transcript, answered questions, current question index, and response status — used to restore the conversation UI to its exact prior state. This endpoint enforces ownership: only the response owner, team admins, and super-admins can fetch a response record.

### Phase 2 Acceptance Criteria

- [ ] Respondent can scan a QR code on a phone and reach the landing page in under 3 taps with no login
- [ ] Landing page shows the correct anonymity declaration based on the survey's anonymity level
- [ ] Language selector appears only when multiple languages are configured; selected language is stored on the response
- [ ] Optional registration form appears when enabled; the note about breaking anonymity is clearly visible
- [ ] AI responses stream token-by-token in the chat UI with a typing indicator before the first token arrives
- [ ] Structured questions render as native form elements (slider, radio, checkbox, etc.), not as chat prompts
- [ ] Progress bar updates after each required question is answered
- [ ] AI follow-up is applied only to questions with `ai_followup = true`
- [ ] Termination fires correctly for all four modes (turn limit, coverage, time, combination)
- [ ] All required questions must be answered before submission is accepted; attempting to submit early returns `422`
- [ ] Answer revisit works when enabled: editing a prior answer re-sends context and continues correctly
- [ ] Thank-you screen shown after successful submission
- [ ] Pseudonymous respondent who returns to the same survey on the same device resumes rather than starting fresh
- [ ] Authenticated respondent who returns on a different device sees "Continuar donde lo dejaste" and resumes correctly
- [ ] Network loss during streaming shows a reconnection indicator and resumes the stream
- [ ] Super-admin can configure AI provider, API key (stored encrypted), and model from the platform settings UI
- [ ] Test-connection button in settings returns pass/fail within 5 seconds
- [ ] Switching the active provider in settings takes effect on the next conversation turn
- [ ] Student responses are never interpolated into the system prompt (verified by code review)
- [ ] The conversation UI is fully usable on a 375px-wide mobile screen

### Risks & Notes

- Streaming via SSE on Go requires careful handling of connection lifecycle — ensure the SSE handler closes the connection cleanly when the client disconnects to avoid goroutine leaks.
- The context assembly for each AI turn grows with every exchange. For long conversations, the full transcript may approach token limits. The adapter should track approximate token usage and truncate older turns (preserving the system prompt and most recent N turns) when approaching the model's context window.
- Test the conversation experience on real university network conditions (campus WiFi, mobile data) before Phase 2 sign-off — streaming latency on constrained networks can make the experience feel broken.
- The `ai_followup` decision logic in the Survey Engine is the most complex piece of state management in the system. Invest heavily in unit tests for this module before moving to integration testing.
- Claude handles Spanish natively but the system prompt must explicitly instruct the AI to respond in the respondent's selected language. Test this instruction with a bilingual survey before Phase 2 sign-off.

---

## Phase 3 — Analysis & Export

### Overview

Phase 3 closes the loop for admins. When a survey closes, the platform automatically analyses all collected responses and surfaces the results in the admin dashboard: a per-question AI-generated summary, sentiment distribution, topic clusters, and flagged outlier responses. Admins can also export the full dataset as a structured CSV and a JSON transcript archive.

This phase turns the platform from a data collection tool into an insight-generation tool — the thing that justifies the AI investment to university stakeholders. The week 12 survey, for example, should let a professor understand in 5 minutes what 120 students experienced during the semester, without reading a single transcript.

Phase 3 is also the phase where the quality of the AI prompts used for analysis matters most. The summaries and cluster labels need to be concise, accurate, and in the correct language. Budget time for prompt tuning against real (or realistic synthetic) response data before sign-off.

### Demo at Completion

The week 12 survey closes automatically on Friday evening. Within 2 minutes, the professor opens the admin dashboard and sees the survey status has changed from "Cerrada" to "Análisis completo." They navigate to the analysis view. At the top: 118 responses, 94% completion rate, 82% in Spanish / 18% in English. Below, per-question breakdowns. The workload rating question (linear scale) shows a sentiment bar: 12% positive, 54% neutral, 34% negative — with the summary "La mayoría de los estudiantes reportó una carga de trabajo manejable durante la mayor parte del semestre, con un pico de estrés notable en la semana 10 relacionado con entregas simultáneas." The open-ended question about improvements shows 6 topic clusters: "rúbricas más claras", "más retroalimentación intermedia", "coordinación entre materias", and 3 others. Three responses are flagged as outliers — one is a student who reported a very different experience from the group. The professor exports a CSV, opens it in Excel, and filters by sentiment to find all the negative workload responses. They also export the JSON transcript file for a qualitative researcher.

### Issues

#### #15 — Analysis Engine
A background job triggered in two phases. **Incremental phase** (per submission): after each `survey.submitted` event, extract sentiment label, sentiment score, and 2–5 topic tags for each answer in that response using a single LLM call per response. Results written to the `answers` table. **Aggregation phase** (on survey close): triggered by the `closed → analysing` status transition. Fetches all submitted responses and runs one LLM call per question to generate: a ~150-word written summary in the survey's default language, a dominant sentiment distribution (% positive / neutral / negative), the top topic clusters (most frequent tags grouped and labelled), and outlier flags (responses whose content deviates significantly from the majority). Results written to `analysis_results` table; outlier flags written back to `answers.is_outlier`. Survey transitions to `complete` when the job finishes. The job must be idempotent (safe to re-run), handle empty response sets gracefully, and make exactly one aggregation LLM call per question (not one per response × question). Unit-tested with a mock AI provider.

#### #16 — Admin Results Dashboard
The admin-facing analysis and management UI. **Survey list** (`/admin/surveys`): title, status badge, response count, completion rate, created date, owner, quick-action buttons (activate, close, duplicate, archive — role-gated). Filter by status. Search by title. "Mostrar archivadas" toggle hidden by default. **Survey detail** (`/admin/surveys/[id]`): survey metadata, lifecycle controls from Phase 1, response count, completion rate, average duration, download QR button, and a link to the analysis view shown only when status is `analysing` or `complete`. **Analysis view** (`/admin/surveys/[id]/analysis`): survey-level stats header (total responses, completion rate, language distribution). Per-question sections in survey order, each showing the AI-generated summary, a sentiment distribution bar, topic clusters as a labelled tag list, and outlier responses in a distinct visual container with a flag icon. All views enforce team scoping and RBAC from Phase 1.

#### #17 — Data Export
On-demand export from the survey detail page. **CSV**: one row per response, columns for `response_id`, `submitted_at`, `language`, `completion_status`, then one column per question (question text as header). Structured answers serialised to natural strings (selected option label, numeric score, comma-separated multi-choice selections). Open-ended answers as raw text. **JSON**: an array of response objects, each with `response_id`, `submitted_at`, `language`, `completion_status`, and a `turns` array of `{role, content, created_at}`. For surveys with ≤500 responses, downloads are synchronous (streamed directly). For surveys with >500 responses, export is async: admin requests the export, sees a progress indicator, and the download triggers when ready. Both endpoints return `403` for viewers and non-team members. Super-admins can export any survey.

### Phase 3 Acceptance Criteria

- [ ] Per-response sentiment and topic tags are populated within 30 seconds of submission
- [ ] Survey transitions to `analysing` within 1 minute of closing
- [ ] Survey transitions to `complete` after all questions have analysis results
- [ ] Analysis view is not accessible (link hidden, endpoint returns 404) until status is `analysing` or `complete`
- [ ] Each question section in the analysis view shows: AI summary, sentiment bar, topic clusters, and outlier list
- [ ] Outlier responses are visually distinct from regular responses in the UI
- [ ] Survey-level stats (response count, completion rate, language distribution) are correct
- [ ] The analysis job is idempotent: re-running on an already-analysed survey overwrites results without errors or crashes
- [ ] A survey with zero responses completes analysis immediately with empty results (no crash, no hanging job)
- [ ] Analysis job makes exactly one aggregation LLM call per question, not one per response
- [ ] CSV export opens correctly in Excel and Google Sheets with correct column headers
- [ ] JSON export contains the full conversation transcript for every submitted response
- [ ] Surveys with ≤500 responses export synchronously as a direct file download
- [ ] Surveys with >500 responses show a progress indicator and trigger the download on completion
- [ ] Viewer role receives `403` on both export endpoints
- [ ] Non-team-member receives `403` on both export endpoints
- [ ] Super-admin can export any survey regardless of team membership

### Risks & Notes

- The analysis prompts (summary, clustering, outlier detection) need to produce consistent, high-quality output in both Spanish and English. Treat prompt engineering for these as a first-class task — test against real or realistic synthetic response data before sign-off. Budget 1–2 days for prompt iteration.
- The aggregation job sends one LLM call per question. For a survey with 10 questions and many concurrent surveys closing at the same time, this could create API rate-limit pressure. Implement a simple rate-limiting wrapper around the analysis calls from the start.
- Outlier detection via LLM prompt is inherently fuzzy. The prompt should instruct the model to flag responses that are factually unusual, express extreme sentiment, or are off-topic — not merely responses that are shorter or longer than average. Review the flagging quality on real data before presenting it to faculty.
- The CSV column-header encoding of question text may produce headers that are too long for some spreadsheet tools if questions are verbose. Consider truncating headers to 60 characters with the full text available in a separate "questions" sheet or metadata row.
- For Phase 3 sign-off, run the full end-to-end cycle at least once with a realistic dataset: 50+ responses on a hybrid survey with 8 questions. Verify that the analysis output is coherent and that the export files open cleanly before declaring Phase 3 complete.

---

## Summary

| Phase | Issues | Key Deliverable |
|---|---|---|
| 1 — Foundation & Admin Tooling | #01 – #08 | Admin can create, configure, and publish surveys with QR codes |
| 2 — AI Conversation Experience | #09 – #14 | Respondents complete AI-guided interviews end-to-end on any device |
| 3 — Analysis & Export | #15 – #17 | Admins receive automatic analysis and can export full datasets |
