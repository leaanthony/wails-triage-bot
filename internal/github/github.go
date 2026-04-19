package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gh "github.com/google/go-github/v69/github"
)

// ZeroTime is the conventional empty time for issues with no date info.

type Issue struct {
	Number    int
	Title     string
	Body      string
	Labels    []string
	State     string
	URL       string
	Author    string    // GitHub login; "" if unknown.
	CreatedAt time.Time // zero if unknown (pre-migration rows).
	UpdatedAt time.Time
	ClosedAt  time.Time
}

type Client struct {
	gh    *gh.Client
	owner string
	repo  string
}

func New(token, ownerRepo string) (*Client, error) {
	owner, repo, ok := strings.Cut(ownerRepo, "/")
	if !ok || owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repo %q, want owner/repo", ownerRepo)
	}
	if token == "" {
		return nil, errors.New("github token is empty")
	}
	c := gh.NewClient(nil).WithAuthToken(token)
	return &Client{gh: c, owner: owner, repo: repo}, nil
}

// ListIssues walks every page of issues (open + closed), filters out pull
// requests, and returns the collected set.
func (c *Client) ListIssues(ctx context.Context) ([]Issue, error) {
	opts := &gh.IssueListByRepoOptions{
		State:       "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	var out []Issue
	for {
		page, resp, err := c.gh.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			if handled := handleRateLimit(err, resp); handled {
				continue
			}
			return nil, fmt.Errorf("list issues page %d: %w", opts.Page, err)
		}
		for _, it := range page {
			if it.IsPullRequest() {
				continue
			}
			out = append(out, convert(it))
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// SearchIssues runs GitHub's Issues Search API against the configured repo.
// The repo qualifier is injected automatically; callers may add others
// (e.g. "label:bug is:open").
func (c *Client) SearchIssues(ctx context.Context, query string, limit int) ([]Issue, error) {
	q := fmt.Sprintf("repo:%s/%s is:issue %s", c.owner, c.repo, query)
	opts := &gh.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: limit},
	}
	for {
		res, resp, err := c.gh.Search.Issues(ctx, q, opts)
		if err != nil {
			if handleRateLimit(err, resp) {
				continue
			}
			return nil, fmt.Errorf("search issues: %w", err)
		}
		out := make([]Issue, 0, len(res.Issues))
		for _, it := range res.Issues {
			if it.IsPullRequest() {
				continue
			}
			out = append(out, convert(it))
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	}
}

// GetIssue fetches a single issue by number. Returns an error for pull
// requests or missing numbers.
func (c *Client) GetIssue(ctx context.Context, number int) (Issue, error) {
	for {
		it, resp, err := c.gh.Issues.Get(ctx, c.owner, c.repo, number)
		if err != nil {
			if handleRateLimit(err, resp) {
				continue
			}
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return Issue{}, fmt.Errorf("issue #%d not found in %s/%s", number, c.owner, c.repo)
			}
			return Issue{}, fmt.Errorf("get issue #%d: %w", number, err)
		}
		if it.IsPullRequest() {
			return Issue{}, fmt.Errorf("#%d is a pull request, not an issue", number)
		}
		return convert(it), nil
	}
}

func convert(it *gh.Issue) Issue {
	labels := make([]string, 0, len(it.Labels))
	for _, l := range it.Labels {
		labels = append(labels, l.GetName())
	}
	return Issue{
		Number:    it.GetNumber(),
		Title:     it.GetTitle(),
		Body:      it.GetBody(),
		Labels:    labels,
		State:     it.GetState(),
		URL:       it.GetHTMLURL(),
		Author:    it.GetUser().GetLogin(),
		CreatedAt: it.GetCreatedAt().Time,
		UpdatedAt: it.GetUpdatedAt().Time,
		ClosedAt:  it.GetClosedAt().Time,
	}
}

// handleRateLimit sleeps on secondary/abuse or primary rate-limit errors and
// signals the caller to retry. Returns false for other errors.
func handleRateLimit(err error, resp *gh.Response) bool {
	var rlErr *gh.RateLimitError
	if errors.As(err, &rlErr) {
		wait := time.Until(rlErr.Rate.Reset.Time)
		if wait < 0 {
			wait = 10 * time.Second
		}
		time.Sleep(wait + time.Second)
		return true
	}
	var abuseErr *gh.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		wait := 30 * time.Second
		if abuseErr.RetryAfter != nil {
			wait = *abuseErr.RetryAfter
		}
		time.Sleep(wait)
		return true
	}
	if resp != nil && resp.StatusCode == http.StatusForbidden {
		time.Sleep(30 * time.Second)
		return true
	}
	return false
}
