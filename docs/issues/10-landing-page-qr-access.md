# 10 — Landing Page & QR Access

## What to build

The public-facing entry point for all survey respondents. When a respondent visits `/s/<public_token>` (via link or QR scan), they see a landing page that sets expectations before the conversation starts. The landing page handles survey state, language selection, anonymity declaration, and optional registration. After the respondent clicks Start, a response record is created and they are taken to the conversation UI (built in #13).

Landing page content:
- Survey title and description
- Estimated duration (if configured in #06)
- Anonymity level declaration (plain language: "Sus respuestas son completamente anónimas" / "Sus respuestas están vinculadas a su cuenta institucional" / etc.)
- Language selector (shown only when more than one language is available, per #07)
- Optional registration form (name + email fields, shown only when `optional_registration = true` per #04) — includes a clear note that providing this information breaks full anonymity
- "Comenzar" / "Start" button

State handling:
- `draft` or `archived` survey → 404 page
- `closed` survey → "Esta encuesta ya cerró" page with survey title
- `active` survey → landing page
- Resume detected (via localStorage token or OAuth session) → landing page with "Continuar donde lo dejaste" option

Deliver:
- SvelteKit route `/s/[token]` (public, no auth required)
- Server-side survey state lookup by `public_token`
- Response creation on Start (calls backend, receives response ID + resume token for unauthenticated users)
- Resume token stored in `localStorage` under `resume_<public_token>`
- Optional registration fields saved to the response record
- Responsive design — optimised for mobile (QR scans happen on phones)

## Acceptance criteria

- [ ] Visiting `/s/<token>` for a `draft` or `archived` survey returns a 404 page
- [ ] Visiting `/s/<token>` for a `closed` survey shows a "survey closed" message with the survey title
- [ ] Visiting `/s/<token>` for an `active` survey renders the landing page with title, description, anonymity declaration, and Start button
- [ ] Estimated duration is shown when configured; hidden when not set
- [ ] Language selector appears only when multiple languages are available; selecting a language persists to the response record
- [ ] Optional registration form appears only when enabled; fields are optional within the form
- [ ] Clicking Start creates a response record and navigates to the conversation UI route
- [ ] A returning pseudonymous respondent with a valid resume token sees a "continue where you left off" option
- [ ] The page is usable on a 375px-wide mobile screen without horizontal scrolling

## Blocked by

- #07 Language Configuration
- #08 Survey Lifecycle & Access Links
- #09 Anonymous Identity & Resume Tokens
