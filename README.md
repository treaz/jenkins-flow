# Jenkins Flow CLI

A robust Go CLI tools to orchestrate Jenkins jobs linearly across multiple instances from your local machine.

## Features

- **Multi-Instance Support**: Orchestrate jobs across different Jenkins servers.
- **Linear Workflow**: Trigger jobs in a defined sequence.
- **Robust Polling**: Waits for Queue items to start and Builds to finish.
- **Fail Fast**: Stops immediately if a job fails.
- **Notifications**: macOS desktop notifications on completion, with optional Slack integration.
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
go mod tidy
go build -o jenkins-flow cmd/jenkins-flow/main.go
```

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
```

2. **Define Workflow**:
   Create a workflow file (e.g., `workflow.yaml`). You can commit this file.

```yaml
workflow:
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

3. **Set Environment Variables** (if using `auth_env`):

```bash
export JENKINS_AUTH_US="username:11xxxxxxxxxxxxxxxxxxxx"
```

4. **Run the Flow**:

```bash
./jenkins-flow -instances instances.yaml -workflow workflow.yaml
```

You can create multiple workflow files (e.g., `deploy-staging.yaml`, `deploy-prod.yaml`) and specify which one to run using the `-workflow` flag.

## Notifications

The CLI sends macOS desktop notifications when workflows complete (both success and failure). These work out of the box with no configuration.

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

