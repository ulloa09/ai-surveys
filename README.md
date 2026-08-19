# AI-Assisted Survey Platform

A conversational survey platform built as an applied research project at ITESO. Replaces static feedback forms with AI-guided interviews that probe deeper, surface themes, and auto-generate analysis for professors and event organizers.

---

## The Problem

University staff collect student feedback using static forms (e.g. the week-12 semester survey). These capture surface-level answers but cannot follow up on pain points or adapt to what a student actually says — resulting in low-signal data that requires significant manual effort to interpret.

Event organizers at congresses and university gatherings also lack a lightweight, QR-accessible survey tool that produces AI-interpreted results without IT involvement.

## The Solution

Admins (professors, event organizers) create surveys with configurable AI conversation modes and question types. Respondents access them via a shareable link or QR code and complete an AI-guided interview in a hybrid chat/form interface. On close, the platform automatically generates per-question summaries, sentiment analysis, and topic clusters.

Two access patterns are supported:

- **Institutional surveys** — authenticated, responses tied to user identity
- **Anonymous surveys** — QR-accessible, no login required, with configurable anonymity guarantees

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | SvelteKit + TypeScript + Skeleton UI (Tailwind CSS) |
| Backend | Go + Chi router |
| DB access | sqlc-style hand-written SQL — no ORM |
| Migrations | goose |
| Database | PostgreSQL |
| AI | Anthropic Claude (Sonnet) via a strategy-pattern adapter |
| Sessions | Server-side encrypted cookies |
| Local dev | Docker Compose |
| CI | GitHub Actions |

---

## Project Structure

```
ai-surveys/
├── frontend/          # SvelteKit application
├── backend/           # Go API server
├── docs/
│   ├── PRD.md         # Full product requirements document
│   ├── milestones.md  # Phase plan with acceptance criteria
│   ├── diagrams.md     # Architecture, ERD, and state machine diagrams
│   └── issues/        # One file per implementation issue (#01–#17)
└── docker-compose.yml
```

---

## Roadmap

The project is organized into three phases. Each phase has a detailed issue breakdown in `docs/issues/`.

### Phase 1 — Foundation & Admin Tooling (Issues #01–#08)

Establishes the monorepo, database, CI pipeline, authentication, role-based access control, and the complete admin experience for creating and publishing surveys.

| Issue | Title |
|---|---|
| [#01](docs/issues/01-project-scaffold.md) | Project Scaffold |
| [#02](docs/issues/02-email-password-auth.md) | Email & Password Authentication |
| [#03](docs/issues/03-teams-rbac.md) | Teams & RBAC |
| [#04](docs/issues/04-survey-crud-duplicate.md) | Survey CRUD & Duplicate |
| [#05](docs/issues/05-question-editor.md) | Question Editor |
| [#06](docs/issues/06-survey-mode-prompt-termination.md) | Survey Mode, System Prompt & Termination Config |
| [#07](docs/issues/07-language-configuration.md) | Language Configuration |
| [#08](docs/issues/08-survey-lifecycle-access-links.md) | Survey Lifecycle & Access Links |

**Demo at completion:** An admin logs in, creates a hybrid survey for the week-12 check-in with 8 questions, sets a schedule and response cap, activates it, and shares the QR code with their class.

### Phase 2 — AI Conversation Experience (Issues #09–#14)

Delivers the core value: a streaming AI-guided interview on any device, with anonymous identity handling and server-side resume for authenticated users.

| Issue | Title |
|---|---|
| [#09](docs/issues/09-anonymous-identity-resume-tokens.md) | Anonymous Identity & Resume Tokens |
| [#10](docs/issues/10-landing-page-qr-access.md) | Landing Page & QR Access |
| [#11](docs/issues/11-ai-provider-adapter.md) | AI Provider Adapter |
| [#12](docs/issues/12-survey-engine.md) | Survey Engine (Conversation State Machine) |
| [#13](docs/issues/13-respondent-conversation-ui.md) | Respondent Conversation UI |
| [#14](docs/issues/14-server-side-resume.md) | Server-side Resume (Authenticated Users) |

**Demo at completion:** A student scans a QR code, completes an AI-guided interview in Spanish on their phone, closes the browser halfway through, and resumes from where they left off after logging in on a different device.

### Phase 3 — Analysis & Export (Issues #15–#17)

Closes the loop: automatic analysis when a survey closes, an admin results dashboard with per-question summaries and sentiment charts, and CSV/JSON export.

| Issue | Title |
|---|---|
| [#15](docs/issues/15-analysis-engine.md) | Analysis Engine |
| [#16](docs/issues/16-admin-results-dashboard.md) | Admin Results Dashboard |
| [#17](docs/issues/17-data-export.md) | Data Export |

**Demo at completion:** A professor or coordinator opens the dashboard the morning after the week-12 survey closes and sees AI-generated summaries, sentiment bars, and topic clusters for each question — without reading a single transcript.

---

## Key Design Decisions

**Survey modes**

- **Mode A — Fixed questions + AI follow-up:** Admin defines questions; AI asks them in order and probes 1–2 levels deeper.
- **Mode B — System prompt only:** Admin writes a system prompt; AI runs a free-form conversation.
- **Mode C — Hybrid (default):** Admin defines required questions *and* a system prompt; AI covers all questions with latitude to probe.

**Anonymity levels**

| Level | Identity | Duplicate prevention |
|---|---|---|
| `identity_verified` | Login required | Server-side by user identity |
| `pseudonymous` | HMAC-SHA256 device fingerprint (non-reversible) | Fingerprint hash on response record |
| `truly_anonymous` | Nothing stored | None — every visit creates a new response |

The anonymity level is **immutable** once the first response exists — enforced at the database layer via trigger, not just the application layer.

**Prompt injection hardening**

Student responses are passed as `user`-role messages only. No student content is ever interpolated into the system prompt. This is a hard constraint enforced in the AI Provider Adapter.

**AI provider strategy pattern**

The backend defines an `AIProvider` interface. Claude (Anthropic) is the default implementation. Other providers can be added by implementing the same interface. The active provider and API key are configurable by super-admins at runtime via a platform settings UI.

---

## Getting Started

> Prerequisites: Docker and Docker Compose installed.

```bash
git clone <repo-url>
cd ai-surveys
cp .env.example .env        # fill in required values
docker-compose up
```

The app will be available at `http://localhost:5173` (frontend) and `http://localhost:8080` (backend API).

Database migrations run automatically on backend startup.

### Seeded users (local development only)

Migration `backend/migrations/003_default_users.sql` seeds one account per role so you can log in without registering — this runs automatically on backend startup, so there's nothing to set up after pulling the repo. They share the password **`12345678`**:

| Email                  | Role          |
| ----------------------- | ------------- |
| `superadmin@test.com`  | `super_admin` |
| `admin@test.com`       | `admin`       |
| `member@test.com`      | `viewer`      |

> ⚠️ These are development-only credentials. Do **not** ship them to any shared or production environment.

---

## Documentation

| Document | Contents |
|---|---|
| [`docs/PRD.md`](docs/PRD.md) | Full product requirements, user stories, module overview, schema decisions, API design, testing strategy |
| [`docs/milestones.md`](docs/milestones.md) | Phase plan with demo scenarios and acceptance criteria per phase |
| [`docs/diagrams.md`](docs/diagrams.md) | System architecture, ERD, survey and response state machines, conversation sequence diagram, module dependency map, AI provider class diagram |
| [`docs/issues/`](docs/issues/) | Implementation issues #01–#17, each with a what-to-build spec, acceptance criteria, and dependency list |
