# Jenkins Flow CLI

A robust Go CLI tools to orchestrate Jenkins jobs linearly across multiple instances from your local machine.

## Features

- **Multi-Instance Support**: Orchestrate jobs across different Jenkins servers.
- **Sequential & Parallel Workflows**: Run steps in sequence or parallel (e.g., deploy to multiple regions simultaneously).
- **Robust Polling**: Waits for Queue items to start and Builds to finish.
- **Fail Fast**: Stops immediately if a job fails (including cancellation of parallel siblings).
- **Notifications**: macOS desktop notifications via `terminal-notifier`, with optional Slack integration.
- **Secure Auth**: Separation of concerns with `instances.yaml` (ignored) and `workflow.yaml`.

## Installation

### Prerequisites
- Go 1.20+
- Access to Jenkins instances

### Generating a Persistent Jenkins API Token

Jenkins API tokens are persistent and do not expire unless manually revoked. To generate one:

1. **Log in to Jenkins** with your user account.

2. **Navigate to your user settings**:
   - Click your username in the top-right corner, or
   - Go to `https://<your-jenkins-url>/me/configure`

3. **Generate a new API Token**:
   - Scroll to the **API Token** section
   - Click **Add new Token**
   - Give it a descriptive name (e.g., `jenkins-flow-cli`)
   - Click **Generate**

4. **Copy the token immediately** — it will only be displayed once!

5. **Use the token** in the format `username:token`:
   ```bash
   # As environment variable (recommended)
   export JENKINS_AUTH_US="your-username:11xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
   
   # Or directly in instances.yaml (local use only)
   token: "your-username:11xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
   ```

> **Note**: API tokens are tied to your user account and inherit your permissions. Keep them secure and never commit them to version control.

### Build

```bash
git clone https://github.com/treaz/jenkins-flow.git
cd jenkins-flow
make deps   # Download dependencies
make build  # Build the binary
```

> **Tip**: Run `make help` to see all available commands (build, test, run, clean, lint, etc.).

## Usage

1. **Configure Instances**:
   Copy `instances.yaml.template` to `instances.yaml` and configure your servers.
   **Note**: `instances.yaml` is gitignored by default.

```yaml
instances:
  prod-us:
    url: "https://jenkins-us.example.com"
    auth_env: "JENKINS_AUTH_US"
  prod-eu:
    url: "https://jenkins-eu.example.com"
    # Or use direct token (local only)
    token: "username:11xxxxxxxxxxxxxxxxxxxx"

# Optional: GitHub Authentication (for wait_for_pr)
github:
  auth_env: "GITHUB_TOKEN"
  # Or use direct token
  # token: "ghp_xxxxxxxxxxxxxxxxxxxx"
```

2. **Define Workflow**:
   Create a workflow file (e.g., `workflow.yaml`). You can commit this file.

```yaml
workflow:
  - wait_for_pr:
      name: "Wait for Release PR"
      owner: "treaz"
      repo: "my-app"
      # Provide either pr_number or head_branch (but not both)
      head_branch: "release/my-app"
      wait_for: "merged" # or "closed"
  - name: "Build US Backend"
    instance: prod-us
    job: "/job/backend/job/build"
    params:
      BRANCH: "main"
      DEPLOY_ENV: "staging"
  - name: "Deploy EU Replica"
    instance: prod-eu
    job: "/job/deploy/job/replica"
```

### Parallel Execution

To run steps in parallel (e.g., deploying to multiple regions simultaneously), use the `parallel` block:

```yaml
workflow:
  # Sequential: Build first
  - name: "Build Backend"
    instance: prod-us
    job: "/job/backend/job/build"
    params:
      BRANCH: "main"

  # Parallel: Deploy to all regions at once
  - parallel:
      name: "Deploy to All Regions"  # Optional group name
      steps:
        - name: "Deploy US"
          instance: prod-us
          job: "/job/deploy"
          params:
            REGION: "us-east-1"
        - name: "Deploy EU"
          instance: prod-eu
          job: "/job/deploy"
          params:
            REGION: "eu-west-1"
        - name: "Deploy APAC"
          instance: prod-apac
          job: "/job/deploy"
          params:
            REGION: "ap-southeast-1"

  # Sequential: Run after all parallel steps succeed
  - name: "Integration Tests"
    instance: prod-us
    job: "/job/integration-tests"
```

**Parallel Behavior:**

- All steps within a `parallel` block run concurrently
- The workflow waits for **all** parallel steps to complete **successfully** before proceeding
- If any step fails, remaining parallel steps are cancelled (fail-fast)
- Parallel groups can be mixed with sequential steps

1. **Set Environment Variables** (if using `auth_env`):

```bash
export JENKINS_AUTH_US="username:11xxxxxxxxxxxxxxxxxxxx"
```

1. **Run the Flow**:

```bash
./jenkins-flow -instances instances.yaml -workflow workflow.yaml
```

You can create multiple workflow files (e.g., `deploy-staging.yaml`, `deploy-prod.yaml`) and specify which one to run using the `-workflow` flag.

### Waiting for Branch-Based PRs

The `wait_for_pr` step can resolve a PR dynamically using `head_branch`. The branch comparison is case-insensitive. If multiple open PRs exist for the same branch, the step fails fast so the workflow does not continue with ambiguous state.

## Notifications

The CLI uses [`terminal-notifier`](https://github.com/julienXX/terminal-notifier) to display macOS desktop notifications when workflows complete (both success and failure).

Install it with Homebrew if it is not already available on your machine:

```bash
brew install terminal-notifier
```

You can verify notifications are working with:

```bash
terminal-notifier -title "Jenkins Flow" -message "Workflow finished"
```

### Slack Integration (Optional)

To also receive Slack notifications, set the following environment variables:

```bash
# Required: Slack incoming webhook URL
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/XXXX/YYYY/ZZZZ"

# Optional: Override channel and username
export SLACK_CHANNEL="#deployments"
export SLACK_USERNAME="Jenkins Flow"
```

To create a Slack webhook:

1. Go to [Slack Apps](https://api.slack.com/apps)
2. Create a new app (or use existing)
3. Enable **Incoming Webhooks**
4. Add a new webhook to a channel
5. Copy the webhook URL

