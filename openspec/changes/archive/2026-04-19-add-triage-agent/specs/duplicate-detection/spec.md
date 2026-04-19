## ADDED Requirements

### Requirement: Two-stage duplicate check
System SHALL implement `check_duplicate` as a two-stage pipeline: (1) embed the target issue and retrieve top-5 KNN candidates from the in-memory store; (2) call the LLM to compare target against candidates and return structured reasoning.

#### Scenario: Input by issue number
- **WHEN** `check_duplicate` is called with `number`
- **THEN** target text is resolved from the store or fetched from GitHub, embedded, and Stage 1 retrieves top-5 candidates

#### Scenario: Input by free text
- **WHEN** `check_duplicate` is called with `text` instead of a number
- **THEN** the text is embedded directly and Stage 1 retrieves top-5 candidates

#### Scenario: Stage 2 LLM reasoning
- **WHEN** Stage 1 candidates are available
- **THEN** system prompts the LLM with target + 5 candidates and receives a JSON object containing `is_duplicate`, `confidence` (0–1), `reasoning`, and per-candidate verdicts

### Requirement: Confidence thresholds
System SHALL classify the duplicate result into tiers based on confidence.

#### Scenario: Auto-close recommendation
- **WHEN** confidence is ≥ 0.85
- **THEN** result is tagged `recommend_auto_close`

#### Scenario: Human review
- **WHEN** confidence is between 0.60 and 0.84 inclusive
- **THEN** result is tagged `human_review`

#### Scenario: Not a duplicate
- **WHEN** confidence is < 0.60
- **THEN** result is tagged `not_duplicate`

### Requirement: No writes to GitHub
System SHALL NOT close, label, or otherwise modify GitHub issues; it SHALL only return recommendations.

#### Scenario: Auto-close tier
- **WHEN** duplicate is classified as recommend_auto_close
- **THEN** no GitHub write API call is made

### Requirement: Malformed LLM response handling
System SHALL handle malformed JSON from Stage 2 by retrying once; if the retry also fails, return a `not_duplicate` tier with `reasoning` explaining the parse failure.

#### Scenario: Invalid JSON then valid retry
- **WHEN** first Stage 2 response is not valid JSON
- **THEN** system retries once with a reminder to return valid JSON

#### Scenario: Two invalid responses
- **WHEN** both attempts fail to parse
- **THEN** system returns `not_duplicate` with a reasoning field naming the parse failure
