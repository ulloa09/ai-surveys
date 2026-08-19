# 14 — Server-side Resume (Authenticated Users, Any-device Resume)

## What to build

Authenticated (OAuth) respondents can leave a survey mid-conversation and resume from any device. When they return to the survey landing page, their in-progress response is detected via their session identity, and they are offered the option to continue where they left off. The conversation UI is restored with the full transcript and current question state.

Deliver:
- On landing page load (for an authenticated user): `GET /api/surveys/:id/my-response` — returns the user's existing `in_progress` or `pending_submission` response for this survey, if any
- Resume flow in SvelteKit: landing page detects an in-progress response and shows "Continuar donde lo dejaste" button alongside the normal "Comenzar" option (starting fresh creates a new response, abandoning the previous one — with a confirmation dialog)
- Conversation UI (from #13) accepts a `responseId` parameter and pre-loads the transcript and current question state from `GET /api/responses/:id`
- `GET /api/responses/:id` returns the full transcript, answered questions, current question index, and response status — used both for resume and for the revisit feature (#13)
- The endpoint enforces ownership: only the response owner, team admins, and super-admins can fetch a response record

## Acceptance criteria

- [ ] An authenticated user who starts a survey and closes the browser mid-way sees "Continuar donde lo dejaste" on their next visit to the landing page
- [ ] Clicking "Continuar" loads the conversation UI with the full prior transcript visible and the AI ready to continue from the current question
- [ ] Clicking "Comenzar de nuevo" shows a confirmation dialog, then creates a fresh response (the previous one is marked `abandoned`)
- [ ] `GET /api/responses/:id` returns `403` when called by a user who does not own the response and is not a team admin or super-admin
- [ ] A user with two in-progress responses for the same survey (edge case) is shown the most recent one for resume
- [ ] Resume works correctly after a survey question edit would have been blocked — i.e. the transcript reflects the questions as they were when the response was created

## Blocked by

- #02 Email & Password Authentication
- #13 Respondent Conversation UI
