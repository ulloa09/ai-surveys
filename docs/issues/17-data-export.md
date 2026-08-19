# 17 — Data Export (CSV + JSON, RBAC Enforcement)

## What to build

On-demand data export for completed surveys. Admins (owners, team members) and super-admins can download two formats: a CSV of structured answers and a JSON archive of full conversation transcripts. Viewers cannot export.

**CSV format:**
- One row per response
- Columns: `response_id`, `submitted_at`, `language`, `completion_status`, then one column per question (using the question text as the header)
- Structured answers are serialized to their natural string representation (e.g. selected option label, numeric score, comma-separated selections)
- Open-ended answers are the raw response text

**JSON format:**
- Array of response objects
- Each object: `response_id`, `submitted_at`, `language`, `completion_status`, `turns` (array of `{role, content, created_at}`)
- One file containing all responses

**Export behaviour:**
- Exports are generated on-demand (no pre-generation)
- For large surveys (>500 responses), generation is async: admin requests export, receives a job ID, polls for completion, then downloads
- Generated files are served as file downloads (not stored permanently)

Deliver:
- `GET /api/surveys/:id/export/csv` — streams CSV response
- `GET /api/surveys/:id/export/json` — streams JSON response
- Both endpoints enforce RBAC: `403` for viewers and non-team-members
- Export buttons in the admin survey detail UI (from #16)
- For surveys with >500 responses: async export with a progress indicator in the UI

## Acceptance criteria

- [ ] Admin can download a CSV from the survey detail page; the file opens correctly in Excel and Google Sheets
- [ ] CSV has one row per response and one column per question with the question text as the header
- [ ] Admin can download a JSON file; it contains the full transcript for every submitted response
- [ ] Viewer role receives `403` on both export endpoints
- [ ] Non-team-member receives `403` on both export endpoints
- [ ] Super-admin can export any survey regardless of team
- [ ] Surveys with ≤500 responses download immediately
- [ ] Surveys with >500 responses show a loading indicator while the file is being generated, then trigger a download on completion

## Blocked by

- #03 Teams & RBAC
- #12 Survey Engine
