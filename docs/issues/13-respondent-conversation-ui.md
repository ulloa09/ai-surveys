# 13 — Respondent Conversation UI (Hybrid Chat/Form, Streaming, Progress, Revisit, Partial Submit)

## What to build

The respondent-facing conversation interface. Renders a hybrid UI where open-ended questions use a chat-style layout (message bubbles, streaming AI response) and structured questions render as native form elements (radio buttons, checkboxes, sliders, etc.). Includes a progress indicator, optional answer revisit, and a submit flow that gates on required question coverage.

**Hybrid rendering rules:**
- `open_ended` → chat bubble from AI, freetext input from respondent
- `single_choice` → chat bubble question + radio button group
- `multi_choice` → chat bubble question + checkbox group
- `true_false` → chat bubble question + two buttons (Sí / No)
- `linear_scale` → chat bubble question + slider with min/max labels
- `ranking` → chat bubble question + drag-to-reorder list
- `matrix` → chat bubble question + grid of radio groups (one row per item)

**Streaming:** AI response tokens appear word-by-word as they arrive via SSE. A typing indicator ("...") is shown while the first token is pending.

**Progress indicator:** Shows `X of Y questions` based on required questions covered. Optional questions do not count toward the total.

**Answer revisit:** When `allow_revisit = true` on the survey, previously answered questions appear in the conversation history with an "Edit" button. Clicking Edit scrolls back to that question, allows a new answer, and re-sends the updated context to the engine before the conversation continues.

**Submit flow:**
- "Enviar respuestas" button appears when all required questions are covered (or when termination triggers)
- If optional questions remain, a confirmation dialog notes they will be skipped
- After submit, a thank-you screen is shown with the survey title

Deliver:
- SvelteKit route `/s/[token]/conversation` (public)
- SSE client that connects to `/api/responses/:id/stream` and appends streamed tokens to the active message bubble
- Per-question-type Svelte components for structured answer input
- Progress bar component
- Answer revisit UI (conditional on `allow_revisit`)
- Thank-you screen after submission
- Responsive design — mobile-first

## Acceptance criteria

- [ ] Open-ended AI responses stream token-by-token into the chat bubble; a typing indicator shows while waiting for the first token
- [ ] Each structured question type renders its native form element correctly
- [ ] Submitting a structured answer sends it to the engine and the conversation advances
- [ ] Progress indicator updates after each required question is answered
- [ ] When `allow_revisit = false`, no edit buttons appear on previous answers
- [ ] When `allow_revisit = true`, an "Edit" button appears on each answered question; clicking it allows re-answering
- [ ] Submit button appears when all required questions are covered
- [ ] Submitting with optional questions unanswered shows a confirmation dialog before proceeding
- [ ] After successful submission, the thank-you screen is shown
- [ ] The UI is usable on a 375px-wide mobile screen without horizontal scrolling
- [ ] Network loss during streaming shows a "connection lost, retrying..." indicator and reconnects automatically

## Blocked by

- #10 Landing Page & QR Access
- #12 Survey Engine
