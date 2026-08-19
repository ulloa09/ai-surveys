# 12 — Survey Engine (Conversation State Machine, Question Flow, Termination, Storage)

## What to build

The core orchestration module. The Survey Engine tracks conversation state for each in-progress response, enforces question flow, evaluates termination conditions, gates submission on required question coverage, and persists every turn and structured answer to the database.

**Response lifecycle states:**
```
not_started → in_progress → pending_submission → submitted → analysing → complete
```

**Engine responsibilities per turn:**
1. Receive the respondent's message
2. Append it to the transcript as a `human` turn
3. Determine which question is currently active (fixed order)
4. Decide whether the AI should probe deeper (question has `ai_followup = true` and the engine hasn't yet recorded a structured answer for it) or advance to the next question
5. Assemble the context via the AI Provider Adapter and stream the response
6. Append the AI response as an `assistant` turn
7. After each AI turn, evaluate all active termination conditions — if any trigger, transition to `pending_submission`
8. On submission: verify all required questions have a recorded answer; if yes, transition to `submitted` and emit a `survey.submitted` event consumed by the Analysis Engine (#15)

**Structured answer recording:**
- For structured question types (single_choice, multi_choice, true_false, linear_scale, ranking, matrix): the answer is captured from the respondent's message at the turn when the active question is a structured type. The engine parses and validates the value before storing it.
- For open_ended questions: the raw text is stored as the answer. Sentiment and topic tags are populated later by the Analysis Engine.

**Partial submission:**
- If termination triggers before all questions are covered, the engine checks whether all *required* questions are answered. If yes, submission proceeds. If no, the AI is instructed to attempt to cover the remaining required questions before the session ends.

Deliver:
- `responses` table: id, survey FK, respondent identity (user FK or fingerprint hash), status (enum above), language, started_at, submitted_at, current_question_index, turn_count
- `turns` table: id, response FK, role (human/assistant), content, created_at (append-only)
- `answers` table: id, response FK, question FK, raw_value (text), sentiment_score (float, nullable), sentiment_label (text, nullable), topic_tags (text array, nullable), is_outlier (bool, nullable)
- Engine service in Go: `ProcessTurn(responseID, message string) error` — handles steps 1–8 above
- `POST /api/responses/:id/turns` endpoint — accepts respondent message, invokes engine, streams AI response via SSE (delegates to adapter from #11)
- `POST /api/responses/:id/submit` endpoint — validates required coverage, transitions status
- Unit tests for the state machine: all transitions, all termination condition combinations, required-question gate, partial submission edge cases

## Acceptance criteria

- [ ] Sending a message to an `in_progress` response appends a `human` turn, calls the AI, appends an `assistant` turn, and returns the streamed response
- [ ] Questions are presented in fixed order; the engine does not skip or reorder questions
- [ ] When `turn_limit` is reached, the response transitions to `pending_submission`
- [ ] When all required questions are covered and `question_coverage` termination is active, the response transitions to `pending_submission`
- [ ] `POST /api/responses/:id/submit` returns `422` if any required question has no recorded answer
- [ ] `POST /api/responses/:id/submit` returns `200` and transitions to `submitted` if all required questions are answered (optional questions may be unanswered)
- [ ] Submitting emits a `survey.submitted` event (log entry or queue message) consumed by the Analysis Engine
- [ ] Structured question answers are parsed and validated before storage (e.g. a `linear_scale` answer outside the configured range is rejected)
- [ ] All state machine transitions are covered by unit tests with no external dependencies mocked beyond the AI provider

## Blocked by

- #05 Question Editor
- #06 Survey Mode, System Prompt & Termination Config
- #11 AI Provider Adapter
