# 06 — Survey Mode, System Prompt & Termination Configuration

## What to build

Admins configure how the AI conducts the interview: which mode (A/B/C), what system prompt to use, and when the conversation ends. The UI explains each option in plain language with examples so admins can make an informed choice without reading documentation.

**Survey modes:**
- **Mode A — Fixed questions with follow-up:** AI asks required questions in order and may probe 1–2 levels deeper per question before moving on.
- **Mode B — System prompt only:** Admin writes a system prompt; AI runs a free-form conversation and decides what to ask and when to stop.
- **Mode C — Hybrid (recommended default):** Admin defines required questions AND a system prompt. AI covers all required questions but has latitude to probe, adapt tone, and transition naturally.

**Termination modes (default: turn limit):**
- `turn_limit` — max number of AI+human exchanges (default: 12)
- `question_coverage` — ends when all required questions are covered
- `time_estimate` — admin sets an expected duration shown to respondents; AI calibrates pace
- `combination` — all three active; first trigger wins

Deliver:
- Schema additions to `surveys`: `mode` (enum A/B/C, default C), `system_prompt` (text), `termination_mode` (enum, default `turn_limit`), `turn_limit` (int, default 12), `time_estimate_minutes` (int, nullable)
- REST: `PATCH /api/surveys/:id` already handles these fields (extend from #04)
- Admin UI: mode selector with explanatory cards (each card has a one-sentence description + a concrete example), system prompt textarea, termination configuration panel with per-mode inputs and a plain-language explanation for each option

## Acceptance criteria

- [ ] Survey creation defaults to Mode C and `turn_limit` termination with a limit of 12
- [ ] Each mode card displays a title, plain-language description, and a concrete example in Spanish
- [ ] Admin can switch modes; switching to Mode B hides the questions section with an explanatory note
- [ ] Admin can write and save a system prompt (required for Mode B and C; optional for Mode A)
- [ ] Admin can select any termination mode; relevant inputs appear/hide based on selection
- [ ] `combination` mode shows all three inputs simultaneously with a note that the first trigger wins
- [ ] All fields persist correctly on save and reload

## Blocked by

- #04 Survey CRUD & Duplicate
