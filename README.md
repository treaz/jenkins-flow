# Jenkins Flow CLI

A robust Go CLI tools to orchestrate Jenkins jobs linearly across multiple instances from your local machine.

## Features

- **Multi-Instance Support**: Orchestrate jobs across different Jenkins servers.
- **Sequential & Parallel Workflows**: Run steps in sequence or parallel (e.g., deploy to multiple regions simultaneously).
- **Robust Polling**: Waits for Queue items to start and Builds to finish.
- **Fail Fast**: Stops immediately if a job fails (including cancellation of parallel siblings).
- **Notifications**: macOS desktop notifications via `terminal-notifier`, with optional Slack integration.
- **Secure Auth**: Separation of concerns with `instances.yaml` (ignored) and `workflow.yaml`.
- **Workflow History**: SQLite database persists all workflow runs with inputs, status, and configuration snapshots.

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

Optionally set a workflow-scoped Slack webhook alongside the workflow name to control where completion notifications are delivered:

```yaml
name: "Deploy Payments API"
slack_webhook: "https://hooks.slack.com/services/T000/B000/XXXX"
workflow:
  - name: "Deploy US"
    instance: prod-us
    job: "/job/deploy"
```

If you omit `slack_webhook`, Jenkins Flow logs a warning and skips Slack delivery (macOS notifications still fire).

1. **Start the Server**:

```bash
# Using Makefile
make serve

# Or directly
./jenkins-flow -port 32567
```

1. **Open the Dashboard**:

Open your browser to `http://localhost:32567`.

From the dashboard you can:

- See all available workflows in the `workflows/` and `examples/` directories.
- Trigger workflows.
- View real-time logs and step status.

### Parallel Execution

To run steps in parallel (e.g., deploying to multiple regions simultaneously), use the `parallel` block in your workflow files:

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

### Waiting for Branch-Based PRs

The `wait_for_pr` step can resolve a PR dynamically using `head_branch`. The branch comparison is case-insensitive. If multiple open PRs exist for the same branch, the step fails fast so the workflow does not continue with ambiguous state.

### Configurable Workflow Inputs

You can define variables in your workflow YAML file that can be modified via the UI before each run. These inputs are substituted into your job parameters.

**1. Define Inputs in `workflow.yaml`:**

Add an `inputs` section at the top level of your workflow file.

```yaml
name: "Deploy Service"
inputs:
  git_branch: main
  region: us-west-1
  env: staging

workflow:
  - name: Deploy
    instance: ci
    job: /job/deploy
    params:
      BRANCH: ${git_branch}  # Variable substitution
      REGION: ${region}
      ENV: ${env}
```

**2. Edit in Dashboard:**
When you select the workflow in the UI, input fields will automatically appear for each defined variable. You can change `git_branch` from `main` to `feature/xyz` and click **Run**.

**3. Persistence:**
Any changes you make in the UI are **saved back to the `workflow.yaml` file**. This ensures that the next time you (or someone else) runs the workflow, it defaults to the last used configuration. The system preserves comments and formatting when updating the file.

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

Slack notifications are powered by the `slack_webhook` property inside each workflow file. Define it alongside the workflow name to opt in to Slack delivery; omit it to disable Slack for that workflow.

To create a Slack webhook:

1. Go to [Slack Apps](https://api.slack.com/apps)
2. Create a new app (or use existing)
3. Enable **Incoming Webhooks**
4. Add a new webhook to a channel
5. Copy the webhook URL


## Workflow History

Jenkins Flow automatically persists all workflow runs to a SQLite database for historical tracking and auditing.

### Database Location

**Default**: `~/.config/jenkins-flow/jenkins-flow.db`

**Custom path via CLI**:
```bash
./jenkins-flow -db-path /custom/path/jenkins-flow.db
```

**Custom path via UI**: Use the Settings API endpoint or update `~/.config/jenkins-flow/settings.json`:
```json
{
  "db_path": "/custom/path/jenkins-flow.db"
}
```

### What's Stored

Each workflow run captures:
- Workflow name and file path
- Start and end timestamps
- Final status (running, success, failed, stopped)
- Input parameters (as JSON)
- Complete workflow YAML configuration snapshot
- Whether PR checks were skipped

### API Endpoints

**List workflow runs** (with pagination and filtering):
```
GET /api/history?limit=50&offset=0&workflow_path=workflows/deploy.yaml&status=success
```

**Get specific run**:
```
GET /api/history/{id}
```

**Get current database path**:
```
GET /api/settings/db-path
```

**Update database path** (requires restart):
```
PUT /api/settings/db-path
Content-Type: application/json

{
  "path": "/new/path/jenkins-flow.db"
}
```

### Database Migrations

The database schema is managed using [golang-migrate](https://github.com/golang-migrate/migrate), a popular database migration library. Migration files are located in `pkg/database/migrations/`. This approach provides:

- **Version control**: Migrations use numbered files (e.g., `000001_initial_schema.up.sql` and `000001_initial_schema.down.sql`)
- **Automatic application**: Migrations run automatically on startup, applying only pending migrations
- **Tracking**: Applied migrations are recorded in the `schema_migrations` table
- **Safety**: Migrations run in transactions with automatic rollback on failure
- **Reversibility**: Both up and down migrations supported

To add a new migration:
1. Create two files in `pkg/database/migrations/`:
   - `000002_description.up.sql` - Forward migration
   - `000002_description.down.sql` - Rollback migration
2. Write your SQL migration code in both files
3. Rebuild and restart - the migration applies automatically

The library handles version tracking, dirty state detection, and ensures migrations are idempotent.

### Error Handling

Database operations are designed to be non-blocking. If database writes fail, errors are logged but workflow execution continues normally.

## Development

### API First Development

This project follows an API-first approach using OpenAPI 3.0.

1. **Modify API Spec**: Edit `api/openapi.yaml`.
2. **Generate Code**: Run `make generate-api`.
3. **Implement**: Update `pkg/server/server.go` to implement the generated interface.

The Make target installs `oapi-codegen` and regenerates the server stubs in `pkg/api/server.gen.go`.

### Swagger UI

A Swagger UI is available at `http://localhost:PORT/swagger` for interactive API documentation and testing.

