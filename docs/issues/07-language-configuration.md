# 07 — Language Configuration per Survey

## What to build

Admins select which languages are available for a survey. Respondents choose from the available languages on the landing page before starting. Spanish is the default and is always available unless explicitly removed. The selected language is stored on the response record and passed to the AI as part of its context.

Deliver:
- Schema addition to `surveys`: `available_languages` (text array, default `['es']`), `default_language` (text, default `'es'`)
- Schema addition to `responses`: `language` (text, set at conversation start)
- Admin UI: language configuration panel — checkbox list of supported languages (Spanish, English), default language selector, note explaining that the AI will respond in the respondent's chosen language
- Respondent-facing language selector on the landing page (rendered in #10), shown only when more than one language is available
- Language value passed to the AI Provider Adapter as part of the system prompt context (implemented fully in #11/#12)

## Acceptance criteria

- [ ] Survey defaults to Spanish-only (`available_languages: ['es']`)
- [ ] Admin can enable English, making both languages available
- [ ] Admin can set English as the default language (e.g. for an English-only course survey)
- [ ] Admin cannot remove all languages — at least one must remain selected
- [ ] When only one language is configured, no language selector is shown to the respondent
- [ ] When multiple languages are configured, respondent sees a language selector on the landing page
- [ ] Selected language is stored on the response record

## Blocked by

- #04 Survey CRUD & Duplicate
