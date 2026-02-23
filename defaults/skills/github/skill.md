# Skill: github

## Description
GitHub operations via the gh CLI: issues, PRs, CI status, code review, API queries, and repository management. Use when the user asks about pull requests, issues, CI/CD, GitHub repos, or code review workflows. Requires gh CLI to be installed and authenticated.

## Instructions
Use the `shell` tool to run `gh` commands. Always specify `--repo owner/repo` when not inside a git directory.

### Pull Requests

```bash
# List open PRs
gh pr list --repo owner/repo

# View PR details
gh pr view 55 --repo owner/repo

# Check CI status on a PR
gh pr checks 55 --repo owner/repo

# Create a PR
gh pr create --title "feat: add feature" --body "Description" --repo owner/repo

# Merge with squash
gh pr merge 55 --squash --repo owner/repo

# View PR diff
gh pr diff 55 --repo owner/repo
```

### Issues

```bash
# List open issues
gh issue list --repo owner/repo --state open

# Create issue
gh issue create --title "Bug: description" --body "Details" --repo owner/repo

# Close issue
gh issue close 42 --repo owner/repo

# View issue
gh issue view 42 --repo owner/repo
```

### CI/Workflow Runs

```bash
# List recent runs
gh run list --repo owner/repo --limit 10

# View run details
gh run view <run-id> --repo owner/repo

# View failed logs only
gh run view <run-id> --repo owner/repo --log-failed

# Re-run failed jobs
gh run rerun <run-id> --failed --repo owner/repo
```

### JSON Output + Filtering

```bash
# List PRs as structured data
gh pr list --repo owner/repo --json number,title,state --jq '.[] | "\(.number): \(.title)"'

# Get repo stats
gh api repos/owner/repo --jq '{stars: .stargazers_count, forks: .forks_count}'

# List labels
gh api repos/owner/repo/labels --jq '.[].name'
```

### PR Review Summary Template

```bash
PR=55 REPO=owner/repo
gh pr view $PR --repo $REPO --json title,body,author,additions,deletions,changedFiles \
  --jq '"**\(.title)** by @\(.author.login)\n+\(.additions) -\(.deletions) across \(.changedFiles) files"'
gh pr checks $PR --repo $REPO
```

### Tips
- Use URLs directly: `gh pr view https://github.com/owner/repo/pull/55`
- Add `--json` with `--jq` for structured, parseable output
- Use `gh api` for any endpoint not covered by shortcuts
- Check auth status: `gh auth status`

## Tools
- name: gh_pr_list
  description: List open pull requests
  command: gh pr list --repo {{repo}}
- name: gh_pr_view
  description: View pull request details
  command: gh pr view {{pr_number}} --repo {{repo}}
- name: gh_pr_checks
  description: Check CI status on a pull request
  command: gh pr checks {{pr_number}} --repo {{repo}}
- name: gh_issue_list
  description: List open issues
  command: gh issue list --repo {{repo}} --state open
- name: gh_issue_create
  description: Create a new issue
  command: gh issue create --title "{{title}}" --body "{{body}}" --repo {{repo}}
- name: gh_run_list
  description: List recent CI workflow runs
  command: gh run list --repo {{repo}} --limit 10
