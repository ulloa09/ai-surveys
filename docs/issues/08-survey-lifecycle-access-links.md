# 08 — Survey Lifecycle & Access Links (States, Schedule, Cap, URL + QR Generation)

## What to build

Surveys move through a defined lifecycle: `draft → active → closed → archived`. Admins can trigger transitions manually, set a scheduled open/close window, and set a response cap. The first trigger to fire (manual, schedule, or cap) closes the survey. Each survey gets a permanent shareable URL and a QR code (PNG + SVG) generated at activation time.

Deliver:
- Schema additions to `surveys`: `status` (enum: draft/active/closed/archived, default draft), `opens_at` (timestamptz, nullable), `closes_at` (timestamptz, nullable), `response_cap` (int, nullable), `public_token` (UUID, generated on first activation, immutable thereafter), `qr_png_url`, `qr_svg_url`
- Background job that polls (or uses pg_cron / scheduled task) every minute to auto-activate and auto-close surveys based on `opens_at` / `closes_at`
- Response cap enforced on the response-creation endpoint: reject new responses when count ≥ cap
- Admin UI: lifecycle panel on the survey detail page — current status badge, manual activate/close/reopen/archive buttons with confirmation dialogs, date-time pickers for schedule, response cap input
- QR code generated server-side (Go) on first activation; PNG and SVG URLs stored and displayed in the admin UI with a download button
- Shareable URL format: `https://<host>/s/<public_token>`
- Archived surveys are hidden from the default survey list but accessible via a "show archived" toggle

## Acceptance criteria

- [ ] New survey starts in `draft` status
- [ ] Admin can manually activate a draft survey; status changes to `active`
- [ ] Admin can manually close an active survey; status changes to `closed`
- [ ] Admin can reopen a closed survey back to `active`
- [ ] Only super-admins can archive a survey
- [ ] Survey with `opens_at` in the past automatically activates within 1 minute
- [ ] Survey with `closes_at` in the past automatically closes within 1 minute
- [ ] Survey with a response cap closes automatically when the cap is reached
- [ ] `public_token` is generated on first activation and never changes
- [ ] Shareable URL `/s/<public_token>` returns a not-found page for draft/archived surveys and a "survey closed" page for closed surveys
- [ ] QR code PNG and SVG are generated and downloadable from the admin UI
- [ ] Schedule and response cap can be combined; the first trigger wins

## Blocked by

- #04 Survey CRUD & Duplicate
