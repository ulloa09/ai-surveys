# 04 — Survey CRUD & Duplicate

## What to build

Admins can create, read, update, delete, and duplicate surveys. This slice covers all survey-level configuration fields but does not yet include questions, modes, or lifecycle management — those come in later slices. A duplicate operation deep-copies all survey config and questions (once questions exist), resets lifecycle state to `draft`, and clears all response data.

Deliver:
- `surveys` table: id, title, description, owner (user FK), team FK, status (`draft` by default), anonymity_level (`pseudonymous` by default), allow_revisit (bool), optional_registration (bool), created_at, updated_at
- REST endpoints: `POST /api/surveys`, `GET /api/surveys`, `GET /api/surveys/:id`, `PATCH /api/surveys/:id`, `DELETE /api/surveys/:id`, `POST /api/surveys/:id/duplicate`
- Admin UI: survey list page, create survey form, edit survey form, delete confirmation, duplicate button
- Anonymity level field is locked to edit once the first response row exists for the survey (enforced at DB layer via trigger or application check)
- Survey list shows title, status, anonymity level, created date, owner

## Acceptance criteria

- [ ] Admin can create a survey with title, description, and anonymity level
- [ ] Survey list shows all surveys belonging to the admin's team
- [ ] Admin can edit title, description, allow_revisit, optional_registration, and anonymity_level (when no responses exist)
- [ ] Attempting to change anonymity_level after the first response returns `409 Conflict`
- [ ] Admin can delete a draft survey; deletion is rejected with `409` if responses exist
- [ ] Duplicate creates a new survey in `draft` state with all config fields copied, suffixed with "(copia)" in the title
- [ ] Duplicated survey has no responses and its anonymity_level is editable
- [ ] All endpoints enforce team scoping from #03

## Blocked by

- #02 Email & Password Authentication
