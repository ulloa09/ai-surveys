# PRD: AI-Assisted Survey Platform (ITESO)

## Problem Statement

University staff at ITESO currently collect student feedback and conduct academic surveys using static forms (e.g., the week 12 semester survey). These forms capture surface-level answers but cannot follow up on pain points, probe for root causes, or adapt to what a student actually says. The result is low-signal data that requires significant manual effort to interpret.

Additionally, event organizers at conferences, congresses, and university gatherings have no lightweight, QR-accessible survey tool that works for anonymous attendees and produces AI-interpreted results without requiring IT involvement.

Admins — professors, research staff, and event organizers — need a self-service platform where they can design surveys, configure AI interview behavior, and receive analysis-ready results without writing code or depending on external tools.

---

## Solution

A general-purpose AI-assisted survey and interview platform deployed at ITESO. Admins create surveys with configurable AI conversation modes, question types, and language settings. Respondents access surveys via a shareable link or QR code and complete an AI-guided interview in a hybrid chat/form interface. On close, the platform automatically generates sentiment analysis, topic clusters, and per-question summaries. Admins can export structured data and full transcripts for further analysis.

The platform supports two access patterns: institutional surveys (authenticated via university OAuth) and anonymous surveys (QR-accessible, no login required), each with appropriate identity and privacy guarantees.

---

## User Stories

### Respondent — Institutional Survey
1. As a student, I want to access a survey via a link sent by my professor, so that I can complete it without needing to find it myself.
2. As a student, I want to authenticate with my institutional email, so that my responses are attributed to my account without needing a separate registration.
3. As a student, I want to see how long a survey is estimated to take before I start, so that I can decide whether to begin it now or later.
4. As a student, I want to resume an incomplete survey from any device, so that I do not lose my progress if I am interrupted.
5. As a student, I want to go back and change a previous answer during the survey (when the admin allows it), so that I can correct a mistake without restarting.
6. As a student, I want to submit a survey once all required questions are answered even if optional questions remain, so that I am not blocked from submitting by questions I cannot answer.
7. As a student, I want the AI to ask follow-up questions on my open-ended answers, so that I can elaborate on issues that a fixed form would never surface.
8. As a student, I want structured questions (multiple choice, rating, true/false) to appear as proper UI elements rather than text prompts, so that the interaction is clear and error-free.
9. As a student, I want to see a progress indicator during the survey, so that I know how far along I am.
10. As a student, I want to take the survey in Spanish or English (if the admin enables both), so that I can respond in the language I am most comfortable with.

### Respondent — Anonymous / QR Survey
11. As a conference attendee, I want to scan a QR code and reach the survey in under 3 taps with no login, so that the friction of participation is minimal.
12. As a conference attendee, I want to see what the survey is about and how long it takes before I start, so that I can make an informed decision to participate.
13. As a conference attendee, I want to know clearly whether my response is anonymous before I begin, so that I can trust the platform with my honest opinion.
14. As a conference attendee, I want to resume an incomplete survey on the same device and browser, so that I do not lose progress if I close the tab accidentally.
15. As a conference attendee, I want to optionally provide my name or email for a prize draw (when the organizer enables this), so that I can participate in event incentives while still knowing this breaks full anonymity.

### Admin — Survey Creation
16. As an admin, I want to create a survey from scratch, so that I can design questions tailored to my specific research needs.
17. As an admin, I want to choose between three survey modes (fixed questions with follow-up, system-prompt-only, or hybrid), so that I can match the survey design to my goal.
18. As an admin, I want the platform to explain each survey mode in plain language with examples, so that I can make an informed choice without reading documentation.
19. As an admin, I want to write a system prompt for the AI, so that I can define the interviewer's persona, context, and goals.
20. As an admin, I want to add questions of different types (open-ended, single-choice, multi-choice, true/false, linear scale, ranking, matrix), so that I can collect both qualitative and quantitative data in the same survey.
21. As an admin, I want to mark each question as required or optional, so that I can ensure critical questions are always answered.
22. As an admin, I want to toggle AI follow-up on a per-question basis, so that I can enable probing on qualitative questions while keeping structured questions concise.
23. As an admin, I want to configure the termination model (turn limit, question coverage, time estimate, or a combination), so that I can control survey length and API cost.
24. As an admin, I want the default termination model to be turn limit, so that I have a sensible starting point without needing to read the docs.
25. As an admin, I want to set the survey's anonymity level (truly anonymous, pseudonymous, or identity-verified), so that respondents receive accurate privacy information.
26. As an admin, I want the anonymity level to be immutable once the first response is recorded, so that the trust contract with respondents is never violated retroactively.
27. As an admin, I want to select which languages are available for a survey (Spanish only, English only, or both), with Spanish as the default, so that the survey is accessible to the right audience.
28. As an admin, I want to toggle whether respondents can go back and change previous answers, so that I can balance flexibility against data integrity.
29. As an admin, I want to toggle optional registration (name/email) for anonymous surveys, so that I can collect contact details for event incentives when needed.
30. As an admin, I want to duplicate any existing survey as a starting point for a new one, so that I do not have to rebuild recurring surveys (like the week 12 survey) from scratch each semester.

### Admin — Survey Lifecycle
31. As an admin, I want to save a survey as a draft and continue editing it before publishing, so that I can refine it before respondents see it.
32. As an admin, I want to activate a survey manually, so that I control exactly when it opens.
33. As an admin, I want to set a scheduled open and close date for a survey, so that the week 12 survey opens and closes automatically without manual intervention.
34. As an admin, I want to set a response cap so that the survey closes automatically after N responses, so that I can limit participation for event surveys without monitoring it manually.
35. As an admin, I want whichever trigger fires first (schedule or response cap) to close the survey, so that I do not need to manage both separately.
36. As an admin, I want to manually close or reopen a survey at any time, so that I can handle edge cases (e.g., a technical incident during a conference).
37. As an admin, I want survey questions to become immutable once the first response is recorded, so that my dataset remains internally consistent.
38. As an admin, I want to archive a survey to hide it from default views without deleting the data, so that I can keep the dashboard clean while retaining historical records.

### Admin — Access and Distribution
39. As an admin, I want the platform to generate a shareable link for each survey, so that I can distribute it via email, LMS, or WhatsApp using my existing channels.
40. As an admin, I want the platform to generate a QR code for each survey, so that I can display it at events for attendees to scan.
41. As an admin, I want the QR code to always map to exactly one survey, so that scanning it never leads to ambiguity.

### Admin — Results and Analysis
42. As an admin, I want the platform to automatically run analysis when a survey closes, so that results are ready without me having to trigger it manually.
43. As an admin, I want to see a per-question AI-generated summary of all responses, so that I can understand the key themes without reading every transcript.
44. As an admin, I want to see sentiment distribution across respondents per question, so that I can identify whether the overall tone is positive, negative, or mixed.
45. As an admin, I want to see automatically identified topic clusters per question, so that I can understand what themes emerged across respondents.
46. As an admin, I want outlier responses flagged automatically, so that I can quickly find unusual or exceptional answers that deserve attention.
47. As an admin, I want to export structured answers as a CSV, so that I can load them into Excel, R, or Python for further analysis.
48. As an admin, I want to export full conversation transcripts as JSON, so that I or a researcher can do qualitative analysis on the raw dialogue.

### Admin — Permissions and Teams
49. As a super-admin, I want to manage all surveys across all teams, so that I have platform-wide oversight and governance.
50. As a super-admin, I want to archive any survey, so that I can clean up the platform even for surveys I do not own.
51. As an admin, I want to create a team and invite other admins, so that multiple people in my department can co-manage surveys.
52. As an admin, I want to assign viewers to a survey so that they can see results but not edit the survey or export raw data, so that I can share results with stakeholders without giving them full access.
53. As an admin, I want surveys I create to be visible only to my team and super-admins by default, so that other departments cannot see my data without my permission.

---

## Implementation Decisions

### Module Overview

#### 1. AI Provider Adapter
A provider-agnostic interface implemented using the strategy pattern. Wraps the LLM call, supports streaming responses via SSE, and handles conversation context assembly. The Claude (Anthropic) implementation is the default. Other providers (Azure OpenAI, OpenAI) can be added by implementing the same interface.

Responsibilities:
- Assemble the conversation payload: system prompt + required question list + full transcript so far
- Stream the AI response token-by-token to the client via SSE
- Post-completion extraction: run a separate LLM call to extract structured answers, sentiment, and topic tags from a completed conversation

#### 2. Survey Engine (Conversation State Machine)
The core orchestration module. Tracks which questions have been asked, which have been answered, whether required questions are satisfied, and when the termination condition is met.

States: `not_started → in_progress → pending_submission → submitted → analysing → complete`

Responsibilities:
- Enforce fixed question order
- Track required vs optional question coverage
- Evaluate termination conditions (turn limit, question coverage, time, combination)
- Determine whether a partial submission is valid (all required questions answered)
- Emit state transition events consumed by the Analysis Engine

#### 3. Response Store
Persistence layer for both the full conversation transcript and structured per-question answers. Two distinct storage concerns kept separate:
- **Transcript store:** append-only log of all turns (role + content + timestamp)
- **Answer store:** one row per (respondent, question), storing the raw answer value plus extracted sentiment and tags (populated post-completion)
- **Resume state:** server-side for authenticated/registered users; client-side (localStorage token) for unauthenticated users

#### 4. Survey Builder (Admin API + UI)
The admin-facing module for creating and configuring surveys. Exposes a form-driven wizard that walks admins through:
- Survey mode selection (with plain-language explanations)
- System prompt authoring
- Question editor (type, required/optional, AI follow-up toggle, answer options for structured types)
- Language configuration (available languages, default)
- Termination configuration (mode + parameters, default: turn limit)
- Anonymity level selection (locked after first response)
- Lifecycle configuration (schedule, response cap)
- Access settings (allow revisit, optional registration)

Survey configuration is stored as a versioned JSON document. Questions are stored as typed rows referencing the survey. Once a response exists, the question schema is frozen.

#### 5. Auth Module
Handles two identity flows:
- **Institutional OAuth:** university email via OAuth 2.0. Session tied to user identity for server-side resume and role enforcement.
- **Anonymous/pseudonymous identity:** one-way HMAC hash of device fingerprint stored in a session cookie. Non-reversible, prevents duplicate submissions without identifying the respondent. For truly anonymous surveys, no fingerprint is stored at all.
- **Resume tokens:** UUID tokens issued on survey start for unauthenticated users, stored in localStorage, expire on survey close.

#### 6. Analysis Engine
Background job triggered by the `closed → analysing` state transition. Runs one LLM call per question (batched) to produce:
- A written summary of all responses to that question
- Sentiment classification per response (positive / neutral / negative + score)
- Topic tags per response
- Outlier flags (responses that deviate significantly from the cluster)

Results are stored back into the answer rows and exposed through the Admin Dashboard.

#### 7. Export Module
On-demand generation of:
- **CSV:** one row per respondent, columns for each question's structured answer + metadata (timestamp, language, completion status)
- **JSON:** array of full conversation transcripts with metadata

Access gated to owner, team members, and super-admins. Viewers cannot export.

#### 8. QR & Access Module
- Generates a QR code (PNG + SVG) pointing to the survey's public URL at survey creation time
- Serves the landing page: survey title, estimated duration, anonymity declaration, language selector, Start button
- Handles optional registration form (name/email) when enabled by admin
- Enforces survey state (shows "survey closed" if not active)

#### 9. Admin Dashboard
Results and survey management UI:
- Survey list (filtered by team/owned, lifecycle state)
- Survey detail: response count, completion rate, live/closed status
- Analysis view: per-question summaries, sentiment charts, topic cluster visualization, outlier list
- Export controls
- Team and permission management

#### 10. Role & Permission Module
RBAC enforced at the API layer:
- Roles: super-admin, admin, viewer, respondent
- Resources scoped to teams/departments
- Survey ownership tracked; team members inherit co-management rights
- Viewers can read analysis dashboard, cannot export or edit

### Schema Decisions
- `surveys` — configuration, mode, status, owner, team, language settings, anonymity level, lifecycle config
- `questions` — type, required flag, follow-up flag, options (for structured types), order index, survey FK
- `responses` — respondent identity (hashed or user FK), survey FK, start/end timestamps, completion status, language used
- `turns` — role (ai/human), content, timestamp, response FK (append-only transcript)
- `answers` — question FK, response FK, raw value, sentiment score, sentiment label, topic tags, outlier flag
- `teams` — name, members (with roles)
- `survey_access_tokens` — UUID, survey FK, expiry (for unauthenticated resume)

### API Design
- REST for admin operations (CRUD on surveys, questions, teams, exports)
- SSE endpoint for streaming AI turns during an active conversation
- WebSocket not required — SSE is sufficient for server-to-client streaming; client sends responses via POST

### Tech Stack

#### Frontend
- **Framework:** SvelteKit (TypeScript)
- **Design system:** Skeleton UI (built on SvelteKit + Tailwind CSS)
- **Streaming:** SSE client (native `EventSource`)

#### Backend
- **Language:** Go
- **Web framework:** Chi (stdlib-compatible, lightweight)
- **DB access:** sqlc (type-safe Go generated from SQL queries — no ORM)
- **Migrations:** golang-migrate
- **Sessions:** gorilla/sessions (server-side, encrypted cookie — required for OAuth resume)
- **Background jobs:** In-process goroutine pool + event channel (no Redis/queue needed at MVP scale)
- **Config:** envconfig (struct-tagged env vars, validated on startup)
- **Logging:** slog (Go stdlib, 1.21+)
- **QR generation:** go-qrcode (skip2/go-qrcode, PNG + SVG output)
- **API key encryption:** AES-256-GCM with `ENCRYPTION_KEY` from env

#### AI
- **SDK:** Anthropic Go SDK (Claude Sonnet as default)
- **Pattern:** Strategy — `AIProvider` interface with deferred concrete OAuth/IDP binding; Claude is the default implementation
- **Streaming:** SSE (server-to-client token stream)

#### Data
- **Database:** PostgreSQL (managed instance on cloud provider)
- **Auth:** OAuth 2.0 via `AuthProvider` interface — concrete IDP (Microsoft Entra ID / Google Workspace / SAML) to be wired when ITESO IDP is confirmed

#### Infra & CI
- **Local dev:** Docker Compose (app + postgres)
- **Production:** Single Docker image, cloud-provider managed PostgreSQL (provider TBD)
- **CI:** GitHub Actions (backend tests, frontend type-check, lint on every push)
- **Testing:** Go stdlib `testing` + testify; Playwright for E2E (frontend)

---

## Testing Decisions

### What makes a good test
Tests should verify observable behavior from the outside — what the module produces given an input — not how it produces it. Avoid testing internal state or private functions. Tests should remain valid if the implementation is refactored, as long as the behavior contract holds.

### Modules to test

**Survey Engine (Conversation State Machine)** — Highest priority. The state machine's transitions, termination conditions, required-question gate for submission, and partial submission logic are pure functions with no external dependencies. Test all state transitions, edge cases (all required answered but optional skipped, turn limit hit before required coverage, etc.), and the termination condition evaluator for each mode.

**AI Provider Adapter** — Test the conversation payload assembly: given a survey config, a transcript, and the current question state, does the adapter produce the correct system message and conversation array? Mock the LLM call to avoid API costs in tests. Test the streaming handler by feeding a mock SSE stream and asserting the client receives the correct sequence.

**Analysis Engine** — Test the job orchestration: given a set of completed responses, does it issue the correct number of LLM calls and write results back to the correct rows? Use a mock LLM returning fixed responses. Test that the job is only triggered by the correct state transition and is idempotent.

**Export Module** — Test CSV and JSON output shape: given a fixed set of answers and transcripts, does the export match the expected schema? Pure transformation logic, no external dependencies.

**Role & Permission Module** — Test every permission boundary: viewer cannot export, non-team-member cannot view, admin cannot archive, super-admin can do everything. Table-driven tests covering all role/action combinations.

**Auth Module** — Test the pseudonymous fingerprint: same device input always produces the same hash; different inputs produce different hashes; the hash is not reversible. Test resume token expiry logic.

---

## Out of Scope (MVP)

- Built-in email distribution — admins use their own channels
- LMS integration (Canvas, Moodle)
- Admin-facing chat to query survey data ("what did engineering students say about group work?")
- Cross-question insights and cohort comparison analysis
- System-provided curated survey templates
- PDF report export
- Mobile native apps (web-only, responsive)
- Real-time response monitoring during an active survey (post-close analysis only)
- Multi-tenancy across institutions (single ITESO deployment)

---

## Further Notes

- The week 12 semester survey is the primary validated use case. The platform should ship with this use case working end-to-end before expanding to other survey types.
- The AI's system prompt receives: the survey's admin-authored system prompt, the full list of required questions with their types and follow-up flags, the conversation transcript so far, and the current question coverage state. This context assembly is the responsibility of the AI Provider Adapter and is the most security-sensitive part of the system — prompt injection from student responses must be considered.
- Anonymity level immutability is a hard constraint enforced at the database layer (not just the application layer) — once a response row exists for a survey, the anonymity field must not be updateable.
- The duplicate-survey feature (MVP templates) must deep-copy all questions and configuration but reset lifecycle state to Draft and clear all response data.
- Language selection by the respondent happens on the landing page before the conversation starts. The selected language is stored on the response row and passed to the AI provider adapter as part of the system prompt context.
- Claude handles Spanish natively. The system prompt should explicitly instruct the AI to respond in the language selected by the respondent.
