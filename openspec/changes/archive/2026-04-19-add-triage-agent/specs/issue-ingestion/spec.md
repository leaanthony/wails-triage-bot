## ADDED Requirements

### Requirement: Single-issue fetch
`internal/github` SHALL expose a `GetIssue(ctx, number)` method returning the same `Issue` shape as `ListIssues` for a single issue number, used by the `get_issue` agent tool to resolve issues not yet in the store.

#### Scenario: Known issue
- **WHEN** `GetIssue` is called with a valid issue number on the configured repo
- **THEN** method returns that issue's metadata

#### Scenario: Unknown issue
- **WHEN** the issue number does not exist
- **THEN** method returns an error naming the missing number

#### Scenario: Pull request number
- **WHEN** the number belongs to a pull request
- **THEN** method returns an error identifying it as a pull request, not an issue
