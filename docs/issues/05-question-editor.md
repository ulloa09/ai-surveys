# 05 — Question Editor (All Question Types, Required/Optional, Follow-up Toggle, Order)

## What to build

Admins can add, edit, reorder, and delete questions within a survey. All seven question types are supported. Each question has a required/optional toggle and a per-question AI follow-up toggle. Question order is fixed and admin-controlled via drag-to-reorder. Questions become immutable once the survey has any responses.

Question types:
- `open_ended` — free-text answer
- `single_choice` — radio buttons, admin defines options
- `multi_choice` — checkboxes, admin defines options
- `true_false` — binary yes/no
- `linear_scale` — numeric range, admin sets min/max (e.g. 1–5 or 1–10) and optional labels
- `ranking` — admin defines a list of items, respondent orders them
- `matrix` — admin defines rows (items) and columns (scale labels), respondent rates each row

Deliver:
- `questions` table: id, survey FK, type, text, required (bool, default true), ai_followup (bool, default true for open_ended / false for structured), options (JSONB for structured types), order_index, created_at
- REST endpoints: `POST /api/surveys/:id/questions`, `PATCH /api/surveys/:id/questions/:qid`, `DELETE /api/surveys/:id/questions/:qid`, `PUT /api/surveys/:id/questions/order`
- Question editor UI: type selector with plain-language descriptions, field editor per type, required/optional toggle, AI follow-up toggle, drag-to-reorder handle
- Editing and deleting questions is blocked with a clear error message once the survey has responses

## Acceptance criteria

- [ ] Admin can add a question of each of the 7 types to a survey in draft state
- [ ] Each question type renders the appropriate configuration fields (options list for choice types, min/max for scale, items for ranking and matrix)
- [ ] Admin can reorder questions via drag-and-drop; order persists on save
- [ ] Required toggle defaults to `true`; AI follow-up toggle defaults to `true` for `open_ended` and `false` for all structured types
- [ ] Admin can change the AI follow-up toggle on any question type
- [ ] Attempting to edit or delete a question after the first response returns `409 Conflict`
- [ ] Duplicate survey (from #04) deep-copies all questions with their configuration

## Blocked by

- #04 Survey CRUD & Duplicate
