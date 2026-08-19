# 11 — AI Provider Adapter (Strategy Interface, Claude Implementation, SSE Streaming, Context Assembly, Provider Settings UI)

## What to build

Implement the provider-agnostic AI adapter using the strategy pattern. Claude (Anthropic) is the default and first implementation. The adapter assembles the full conversation context for each turn, streams the AI response token-by-token to the client via SSE, and runs a separate post-completion extraction call. Super-admins can configure the active provider and API credentials through a platform settings UI.

**Strategy interface (Go):**
```go
type AIProvider interface {
    StreamTurn(ctx context.Context, req TurnRequest) (<-chan TurnChunk, error)
    Extract(ctx context.Context, req ExtractionRequest) (ExtractionResult, error)
}

type TurnRequest struct {
    SystemPrompt    string
    Questions       []Question      // ordered list with types and follow-up flags
    Transcript      []Turn          // full conversation history so far
    QuestionCoverage map[string]bool // which required questions have been answered
    Language        string
}

type ExtractionRequest struct {
    Questions  []Question
    Transcript []Turn
    Language   string
}

type ExtractionResult struct {
    Answers []ExtractedAnswer // one per question: value, sentiment, tags
}
```

**Context assembly rules:**
- System prompt = admin's system prompt + language instruction + question list with types and follow-up flags + current coverage state
- Conversation history = full transcript passed as alternating user/assistant messages
- Prompt injection hardening: student responses are passed as user-role messages only; no student content is interpolated into the system prompt

**Provider settings (platform-level, super-admin only):**
- Active provider selector (Claude / OpenAI / Azure OpenAI) with plain-language description of each
- API key field (write-only — displayed as masked after save)
- Model selector per provider (e.g. `claude-sonnet-4-6`, `gpt-4o`)
- Test connection button that sends a minimal probe request and reports success/failure

Deliver:
- `AIProvider` interface and `TurnRequest`/`ExtractionRequest`/`ExtractionResult` types in Go
- Claude implementation using the Anthropic Go SDK with streaming
- SSE endpoint `GET /api/responses/:id/stream` — accepts a `message` query param (the respondent's latest input), streams the AI response as SSE events
- Platform settings table: `platform_settings` (key/value, encrypted values for API keys)
- Admin UI: platform settings page accessible to super-admins only, with provider selector, API key input, model selector, and test connection button
- A dev-only `POST /api/dev/test-ai` endpoint that sends a one-turn prompt and streams the response — useful for verifying provider configuration without a full survey

## Acceptance criteria

- [ ] `GET /api/responses/:id/stream?message=hello` streams the AI response as SSE `data:` events, one token per event, terminated with `data: [DONE]`
- [ ] The assembled system prompt contains the admin's system prompt, the language instruction, and the question list
- [ ] Student response content is never interpolated into the system prompt (injection hardening)
- [ ] Switching the active provider in platform settings to a second implementation (even a stub) changes which provider handles subsequent requests
- [ ] API key is stored encrypted; the UI shows only masked characters after save
- [ ] Test connection button returns a success or failure message within 5 seconds
- [ ] Super-admin can change the active model for the Claude provider
- [ ] Non-super-admins cannot access the platform settings page or API
- [ ] `/api/dev/test-ai` streams a response and is only available when `APP_ENV=development`

## Blocked by

- #01 Project Scaffold
