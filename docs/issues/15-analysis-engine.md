# 15 — Analysis Engine (Background Job: Summary, Sentiment, Topic Clusters, Outlier Flags)

## What to build

A background job triggered by the `survey.submitted` event emitted by the Survey Engine when a response transitions to `submitted`, and by the survey `closed` transition. The job runs one LLM call per question across all responses to produce per-question summaries, sentiment scores, topic tags, and outlier flags. Results are stored back into the `answers` table and a survey-level `analysis` record.

**Job behaviour:**
- Triggered when a survey transitions to `closed` (processes all submitted responses at once)
- Also triggered incrementally per response on `survey.submitted` to populate per-response sentiment and tags as responses arrive (so the dashboard is not empty until close)
- Idempotent: re-running on already-analysed responses overwrites with fresh results (safe to retry)
- Survey transitions: `closed → analysing → complete` (the `analysing` state indicates the job is running)

**Per-response extraction (incremental, runs after each submission):**
- Sentiment label (positive / neutral / negative) + score (0.0–1.0) per answer
- Topic tags (2–5 short phrases) per open-ended answer

**Per-question aggregation (runs after survey closes):**
- AI-generated written summary of all responses to that question (~150 words)
- Dominant sentiment distribution (% positive / neutral / negative)
- Top topic clusters (most frequent tags across all responses, grouped)
- Outlier flags: responses whose content deviates significantly from the majority (flagged via a prompt that asks the LLM to identify unusual responses given the full set)

Deliver:
- `analysis_results` table: survey FK, question FK (nullable for survey-level), summary_text, sentiment_distribution (JSONB), topic_clusters (JSONB), analysed_at
- Per-response sentiment/tag update to the `answers` table (populates `sentiment_score`, `sentiment_label`, `topic_tags`, `is_outlier`)
- Background job runner in Go (goroutine + channel, or simple polling loop)
- Job triggered by status transitions in the Survey Engine
- Unit tests: job is idempotent, correct number of LLM calls per survey, results written to correct rows, job handles empty response set gracefully

## Acceptance criteria

- [ ] When a response is submitted, per-answer sentiment and topic tags are populated within 30 seconds
- [ ] When a survey closes, the analysis job starts and the survey transitions to `analysing`
- [ ] After the job completes, the survey transitions to `complete` and `analysis_results` rows exist for each question
- [ ] Re-running the job on an already-analysed survey overwrites results without errors (idempotent)
- [ ] A survey with zero responses completes analysis immediately with empty results (no crash)
- [ ] The job makes exactly one LLM call per question for aggregation (not one per response × question)
- [ ] `is_outlier` is set to `true` on answers flagged by the outlier-detection prompt

## Blocked by

- #12 Survey Engine
