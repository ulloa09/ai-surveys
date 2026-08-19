# 09 — Anonymous Identity & Resume Tokens

## What to build

Implement the two non-OAuth identity models: pseudonymous identity (for anonymous surveys that still need duplicate-submission prevention) and truly anonymous (no identity stored at all). Also introduce resume tokens — UUID tokens issued when an unauthenticated respondent starts a survey, stored in `localStorage`, used to resume a conversation on the same device.

**Identity rules by anonymity level:**
- `identity_verified` — requires OAuth session (handled by #02); not applicable here
- `pseudonymous` — HMAC-SHA256 hash of a stable device fingerprint (user-agent + accept-language header + a server-side salt). Non-reversible. Stored on the response record. Prevents duplicate submissions per device.
- `truly_anonymous` — no fingerprint, no token stored server-side. A new response is created on every survey start. Duplicate prevention is not attempted.

**Resume tokens (unauthenticated only):**
- UUID v4 issued by the server when a pseudonymous respondent starts a survey
- Returned in the response body and stored in `localStorage` by the client under key `resume_<survey_public_token>`
- On subsequent visits to the survey landing page, the client sends the token; the server looks it up and resumes the existing response
- Tokens expire when the survey closes or is archived

Deliver:
- `survey_access_tokens` table: id (UUID), response FK, survey FK, expires_at, created_at
- Fingerprint hash utility (Go): deterministic, takes request headers + salt, returns SHA-256 hex
- Duplicate-submission check on response creation for `pseudonymous` surveys
- Resume token issuance and lookup endpoints
- Client-side localStorage token storage/retrieval in SvelteKit (implemented in the landing page in #10)

## Acceptance criteria

- [ ] Starting a `pseudonymous` survey from the same device twice reuses the existing response (resume) rather than creating a new one
- [ ] Starting a `truly_anonymous` survey always creates a new response regardless of device
- [ ] The fingerprint hash is deterministic: same headers + salt always produce the same hash
- [ ] The fingerprint hash is non-reversible: there is no endpoint or code path that maps a hash back to device details
- [ ] Resume token is returned on response creation and stored in `localStorage`
- [ ] Presenting a valid resume token resumes the correct in-progress response
- [ ] Resume token is invalidated when the survey closes or is archived
- [ ] Presenting an expired or unknown token starts a new response (for truly_anonymous) or falls back to fingerprint lookup (for pseudonymous)

## Blocked by

- #01 Project Scaffold
