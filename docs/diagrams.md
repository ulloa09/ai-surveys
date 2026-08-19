# Architecture & Design Diagrams

## 1. System Architecture

High-level view of all components and their relationships.

```mermaid
graph TB
  subgraph Clients
    Student["Student / Attendee\n(Browser / Phone)"]
    Admin["Admin / Professor\n(Browser)"]
    SuperAdmin["Super Admin\n(Browser)"]
  end

  subgraph Frontend ["SvelteKit Frontend"]
    LandingUI["/s/[token]\nLanding Page"]
    ConvUI["/s/[token]/conversation\nConversation UI"]
    AdminUI["/admin\nAdmin Dashboard"]
    SettingsUI["/admin/settings\nPlatform Settings"]
  end

  subgraph Backend ["Go Backend"]
    API["REST API"]
    SSE["SSE Streaming\nEndpoint"]
    Engine["Survey Engine\n(State Machine)"]
    Adapter["AI Provider Adapter\n(Strategy Pattern)"]
    AnalysisJob["Analysis Engine\n(Background Job)"]
    AuthMW["Auth Middleware"]
    RBAC["RBAC Enforcement"]
  end

  subgraph External
    OAuth["University OAuth\n(Institutional Email)"]
    Claude["Anthropic Claude API"]
    DB[("PostgreSQL")]
  end

  Student -->|"Link / QR scan"| LandingUI
  Student -->|"Conversation"| ConvUI
  Admin -->|"Survey management"| AdminUI
  SuperAdmin -->|"Provider config"| SettingsUI

  LandingUI & ConvUI & AdminUI & SettingsUI -->|"REST"| API
  ConvUI -->|"SSE (token stream)"| SSE

  API --> AuthMW --> RBAC
  SSE --> Engine
  API --> Engine
  Engine --> Adapter
  Adapter -->|"stream=true"| Claude
  Engine --> AnalysisJob
  AnalysisJob --> Adapter

  API -->|"OAuth redirect"| OAuth
  Backend -->|"Read / Write"| DB
```

---

## 2. Entity Relationship Diagram

```mermaid
erDiagram
  users {
    uuid id PK
    string email
    string display_name
    enum role
    timestamp created_at
  }

  teams {
    uuid id PK
    string name
    uuid created_by FK
    timestamp created_at
  }

  team_members {
    uuid team_id FK
    uuid user_id FK
    enum role
  }

  surveys {
    uuid id PK
    string title
    string description
    uuid owner_id FK
    uuid team_id FK
    enum status
    enum mode
    text system_prompt
    enum anonymity_level
    string[] available_languages
    string default_language
    enum termination_mode
    int turn_limit
    int time_estimate_minutes
    int response_cap
    bool allow_revisit
    bool optional_registration
    timestamp opens_at
    timestamp closes_at
    uuid public_token
    string qr_png_url
    string qr_svg_url
    timestamp created_at
    timestamp updated_at
  }

  questions {
    uuid id PK
    uuid survey_id FK
    enum type
    text text
    bool required
    bool ai_followup
    jsonb options
    int order_index
    timestamp created_at
  }

  responses {
    uuid id PK
    uuid survey_id FK
    uuid user_id FK
    string fingerprint_hash
    enum status
    string language
    timestamp started_at
    timestamp submitted_at
    int current_question_index
    int turn_count
    string registered_name
    string registered_email
  }

  turns {
    uuid id PK
    uuid response_id FK
    enum role
    text content
    timestamp created_at
  }

  answers {
    uuid id PK
    uuid response_id FK
    uuid question_id FK
    text raw_value
    float sentiment_score
    string sentiment_label
    string[] topic_tags
    bool is_outlier
  }

  analysis_results {
    uuid id PK
    uuid survey_id FK
    uuid question_id FK
    text summary_text
    jsonb sentiment_distribution
    jsonb topic_clusters
    timestamp analysed_at
  }

  survey_access_tokens {
    uuid id PK
    uuid response_id FK
    uuid survey_id FK
    timestamp expires_at
    timestamp created_at
  }

  platform_settings {
    string key PK
    text value
    timestamp updated_at
  }

  users ||--o{ team_members : "member of"
  teams ||--o{ team_members : "has"
  users ||--o{ surveys : "owns"
  teams ||--o{ surveys : "contains"
  surveys ||--o{ questions : "has"
  surveys ||--o{ responses : "collects"
  users ||--o{ responses : "submits"
  responses ||--o{ turns : "contains"
  responses ||--o{ answers : "records"
  questions ||--o{ answers : "answered by"
  surveys ||--o{ analysis_results : "produces"
  questions ||--o{ analysis_results : "analysed in"
  responses ||--o| survey_access_tokens : "has"
```

---

## 3. Survey Lifecycle State Machine

```mermaid
stateDiagram-v2
  [*] --> draft : Survey created

  draft --> active : Manual activate\nor opens_at reached
  draft --> archived : Super-admin archives

  active --> closed : Manual close\nor closes_at reached\nor response_cap reached
  active --> active : Responses collected

  closed --> active : Manual reopen
  closed --> analysing : survey.closed event triggers\nAnalysis Engine
  closed --> archived : Super-admin archives

  analysing --> complete : Analysis job done

  complete --> archived : Super-admin archives

  archived --> [*]

  note right of draft
    Questions and config
    are fully editable
  end note

  note right of active
    Questions become immutable\nafter first response
  end note

  note right of analysing
    Background job running:\nsentiment, summaries,\ntopic clusters, outliers
  end note
```

---

## 4. Response (Conversation) State Machine

```mermaid
stateDiagram-v2
  [*] --> not_started : Response created\n(Respondent clicks Start)

  not_started --> in_progress : First message sent

  in_progress --> in_progress : Turn processed\n(termination not yet triggered)

  in_progress --> pending_submission : Termination triggered\n(turn limit / question coverage / time)

  pending_submission --> in_progress : Required questions still missing\n(AI continues to cover them)

  pending_submission --> submitted : All required questions answered\nPOST /submit

  submitted --> analysing : survey.submitted event\n(incremental sentiment + tags)

  analysing --> complete : Per-response extraction done

  in_progress --> abandoned : Respondent starts fresh\n(confirmation required)

  note right of pending_submission
    Gate: all required questions
    must have a recorded answer
    before submission is accepted
  end note
```

---

## 5. Respondent Conversation Flow

```mermaid
sequenceDiagram
  actor R as Respondent
  participant L as Landing Page
  participant B as Go Backend
  participant E as Survey Engine
  participant A as AI Provider Adapter
  participant C as Claude API
  participant D as PostgreSQL

  R->>L: Visit /s/<token> (link or QR scan)
  L->>B: GET /api/surveys/by-token/<token>
  B->>D: Lookup survey by public_token
  D-->>B: Survey (status, config, languages)
  B-->>L: Survey metadata
  L-->>R: Landing page (title, anonymity declaration,\nlanguage selector, estimated time)

  R->>L: Select language + click "Comenzar"
  L->>B: POST /api/responses (survey_id, language)
  B->>D: Create response record
  D-->>B: response_id + resume_token
  B-->>L: { response_id, resume_token }
  L->>L: Store resume_token in localStorage
  L-->>R: Navigate to /conversation

  loop Each conversation turn
    R->>B: POST /api/responses/:id/turns { message }
    B->>E: ProcessTurn(response_id, message)
    E->>D: Append human turn
    E->>A: StreamTurn(system_prompt, questions,\ntranscript, coverage, language)
    A->>C: POST /v1/messages (stream: true)
    C-->>A: Token stream
    A-->>E: Token stream
    E-->>B: Token stream
    B-->>R: SSE data events (token by token)
    E->>D: Append assistant turn
    E->>D: Record structured answer (if applicable)
    E->>E: Evaluate termination conditions
  end

  E->>E: Termination triggered → pending_submission
  R->>B: POST /api/responses/:id/submit
  B->>E: ValidateAndSubmit(response_id)
  E->>D: Transition status → submitted
  E->>E: Emit survey.submitted event
  B-->>R: 200 OK
  R-->>R: Thank-you screen
```

---

## 6. Analysis Engine Flow

```mermaid
sequenceDiagram
  participant E as Survey Engine
  participant J as Analysis Job
  participant A as AI Provider Adapter
  participant C as Claude API
  participant D as PostgreSQL

  Note over E,D: Incremental — runs after each submission
  E->>J: survey.submitted(response_id)
  J->>D: Fetch response transcript + answers
  loop Each answer in response
    J->>A: Extract(transcript, question, language)
    A->>C: Sentiment + topic tags prompt
    C-->>A: { sentiment_label, score, tags }
    A-->>J: ExtractedAnswer
    J->>D: UPDATE answers SET sentiment_score,\nsentiment_label, topic_tags
  end

  Note over E,D: Aggregation — runs when survey closes
  E->>J: survey.closed(survey_id)
  J->>D: UPDATE surveys SET status = analysing
  J->>D: Fetch all submitted responses + answers

  loop Each question in survey
    J->>D: Fetch all answers for this question
    J->>A: Aggregate(question, all_answers, language)
    A->>C: Summary + clusters + outliers prompt
    C-->>A: { summary, clusters, outlier_ids }
    A-->>J: AggregationResult
    J->>D: INSERT analysis_results
    J->>D: UPDATE answers SET is_outlier = true\nWHERE id IN outlier_ids
  end

  J->>D: UPDATE surveys SET status = complete
```

---

## 7. Module Dependency Map

```mermaid
graph LR
  subgraph BE ["Backend Modules"]
    Auth["Auth Module\nOAuth + Anonymous\nIdentity + Resume Tokens"]
    RBAC["RBAC Module\nRoles + Teams +\nPermission Enforcement"]
    Builder["Survey Builder\nCRUD + Question Editor\n+ Config"]
    Engine["Survey Engine\nConversation State Machine\n+ Storage"]
    Adapter["AI Provider Adapter\nStrategy Pattern\nClaude / OpenAI / Azure"]
    Analysis["Analysis Engine\nBackground Job\nSentiment + Clusters"]
    Export["Export Module\nCSV + JSON"]
    QRAccess["QR & Access Module\nLanding Page +\nQR Generation"]
  end

  subgraph FE ["Frontend (SvelteKit)"]
    AdminUI["Admin Dashboard\nSurvey Management\n+ Results"]
    ConvUI["Conversation UI\nHybrid Chat/Form\n+ Streaming"]
    LandingUI["Landing Page\nAnonymity Declaration\n+ Language Selector"]
    SettingsUI["Platform Settings\nProvider Selection\n+ API Keys"]
  end

  subgraph Ext ["External"]
    DB[("PostgreSQL")]
    OAuthProv["University OAuth"]
    ClaudeAPI["Claude API\n(+ other providers)"]
  end

  Auth --> OAuthProv
  Auth --> DB
  RBAC --> Auth
  RBAC --> DB
  Builder --> RBAC
  Builder --> DB
  Engine --> Adapter
  Engine --> DB
  Adapter --> ClaudeAPI
  Analysis --> Adapter
  Analysis --> DB
  Export --> RBAC
  Export --> DB
  QRAccess --> DB

  AdminUI --> Builder
  AdminUI --> RBAC
  AdminUI --> Analysis
  AdminUI --> Export
  SettingsUI --> Adapter
  ConvUI --> Engine
  ConvUI --> Auth
  LandingUI --> QRAccess
  LandingUI --> Auth
```

---

## 8. AI Provider Adapter — Strategy Pattern

```mermaid
classDiagram
  class AIProvider {
    <<interface>>
    +StreamTurn(ctx, TurnRequest) chan TurnChunk, error
    +Extract(ctx, ExtractionRequest) ExtractionResult, error
  }

  class ClaudeProvider {
    -client AnthropicClient
    -model string
    +StreamTurn(ctx, TurnRequest) chan TurnChunk, error
    +Extract(ctx, ExtractionRequest) ExtractionResult, error
    -assembleMessages(TurnRequest) []Message
  }

  class OpenAIProvider {
    -client OpenAIClient
    -model string
    +StreamTurn(ctx, TurnRequest) chan TurnChunk, error
    +Extract(ctx, ExtractionRequest) ExtractionResult, error
  }

  class AzureOpenAIProvider {
    -client AzureClient
    -deployment string
    +StreamTurn(ctx, TurnRequest) chan TurnChunk, error
    +Extract(ctx, ExtractionRequest) ExtractionResult, error
  }

  class ProviderRegistry {
    -providers map_string_AIProvider
    -active string
    +Get() AIProvider
    +Register(name string, p AIProvider)
    +SetActive(name string) error
  }

  class TurnRequest {
    +SystemPrompt string
    +Questions []Question
    +Transcript []Turn
    +QuestionCoverage map_string_bool
    +Language string
  }

  class ExtractionRequest {
    +Questions []Question
    +Transcript []Turn
    +Language string
  }

  class ExtractionResult {
    +Answers []ExtractedAnswer
  }

  class ExtractedAnswer {
    +QuestionID string
    +SentimentLabel string
    +SentimentScore float64
    +TopicTags []string
  }

  AIProvider <|.. ClaudeProvider : implements
  AIProvider <|.. OpenAIProvider : implements
  AIProvider <|.. AzureOpenAIProvider : implements
  ProviderRegistry --> AIProvider : manages active
  ClaudeProvider ..> TurnRequest : consumes
  ClaudeProvider ..> ExtractionRequest : consumes
  ClaudeProvider ..> ExtractionResult : produces
```

---

## 9. Survey Mode Decision Tree (Admin UX)

```mermaid
flowchart TD
  Start(["Admin creates a survey"]) --> ModeQ{"Which survey mode?"}

  ModeQ -->|"Mode A"| ModeA["Fixed Questions\n+ AI Follow-up\n\nAdmin defines explicit questions.\nAI asks them in order and probes\n1–2 levels deeper per question."]
  ModeQ -->|"Mode B"| ModeB["System Prompt Only\n\nAdmin writes a system prompt.\nAI runs a free-form conversation\nand decides what to ask."]
  ModeQ -->|"Mode C ★ Default"| ModeC["Hybrid\n\nAdmin defines required questions\nAND a system prompt. AI covers\nall questions with latitude to probe."]

  ModeA --> TermQ
  ModeB --> TermQ
  ModeC --> TermQ

  TermQ{"Termination mode?"} -->|"Default"| TurnLimit["Turn Limit\n\nMax exchanges (default: 12).\nPredictable length and cost."]
  TermQ -->|"Coverage"| Coverage["Question Coverage\n\nEnds when all required\nquestions are answered."]
  TermQ -->|"Time"| TimeEst["Time Estimate\n\nAdmin sets expected duration.\nShown to respondents before start."]
  TermQ -->|"Combination"| Combo["Combination\n\nAll three active.\nFirst trigger wins."]

  TurnLimit & Coverage & TimeEst & Combo --> AnonQ

  AnonQ{"Anonymity level?"} -->|"Institutional"| Inst["Identity Verified\n\nRequires OAuth login.\nResponses linked to user account."]
  AnonQ -->|"Internal anonymous"| Pseudo["Pseudonymous\n\nNo login. Device fingerprint hash\nprevents duplicate submissions."]
  AnonQ -->|"Public / Event"| TrueAnon["Truly Anonymous\n\nNo fingerprint. No duplicate\nprevention. QR-ready."]

  Inst & Pseudo & TrueAnon --> Done(["Survey configured ✓"])
```

---

## 10. Authentication & Identity Decision Tree

```mermaid
flowchart TD
  Visit(["Respondent visits /s/token"]) --> SurveyAnon{"Survey\nanonymity level?"}

  SurveyAnon -->|"identity_verified"| OAuthFlow["Redirect to\nUniversity OAuth"]
  OAuthFlow --> OAuthSuccess{"Auth\nsuccessful?"}
  OAuthSuccess -->|"Yes"| ServerResume["Check for existing\nin-progress response\nby user identity"]
  OAuthSuccess -->|"No"| Blocked["Show login\nerror page"]

  SurveyAnon -->|"pseudonymous"| Fingerprint["Compute HMAC-SHA256\ndevice fingerprint\n(UA + Accept-Language + salt)"]
  Fingerprint --> FingerprintLookup{"Existing response\nfor this fingerprint?"}
  FingerprintLookup -->|"Yes"| TokenCheck{"Valid resume\ntoken in localStorage?"}
  TokenCheck -->|"Yes"| ResumeAnon["Resume existing\nresponse"]
  TokenCheck -->|"No"| OfferResume["Show 'Continue where\nyou left off' option"]
  FingerprintLookup -->|"No"| NewResponsePseudo["Create new response\nIssue resume token\nStore in localStorage"]

  SurveyAnon -->|"truly_anonymous"| NewResponseAnon["Create new response\nNo fingerprint stored\nNo token issued"]

  ServerResume --> OfferResumeAuth["Show 'Continue where\nyou left off' option\n(any device)"]
  OfferResumeAuth & OfferResume & ResumeAnon & NewResponsePseudo & NewResponseAnon --> ConvUI(["Enter conversation UI"])
```
