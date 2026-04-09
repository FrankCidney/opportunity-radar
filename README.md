# Opportunity Radar

Opportunity Radar is a self-hosted Go app that collects jobs, scores them against your preferences, stores them in PostgreSQL, and sends digest emails when configured.

For technical architecture, design constraints, and future-direction notes, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## The Idea

From a user's perspective, the app is meant to work like this:

1. Open the app and fill in your job preferences.
2. Optionally add your email address and turn on email updates.
3. The app uses those preferences to decide which jobs are a better fit for you.
4. Starting with the next scheduled run, which is usually the next day, the app collects jobs and prepares your recommendations.
5. If email updates are enabled, you receive the recommended jobs by email.

If you want to see it work immediately instead of waiting for the next scheduled run, you can use `Run Once` in the UI after setup.

This repo is designed for single-user deployment:
- one app instance
- one database
- one operator
- self-hosted on your own machine or your own cloud account

There is no shared central server, no multi-user account system, and no SaaS control plane.

## Getting Started

To use this app for yourself, start by getting your own copy of the code.

### Option 1: Clone it locally

If you want to run it on your own machine with Docker:

```bash
git clone https://github.com/<your-account>/opportunity-radar.git
cd opportunity-radar
```

### Option 2: Put it on your own GitHub first

If you want to deploy on Railway, the cleanest path is:

1. Fork this repo to your own GitHub account, or create a new repo in your GitHub account and push this code there.
2. Clone your copy locally if you also want to edit it on your machine.
3. Connect Railway to your GitHub copy of the repo.

That way:

- Railway deploys from your own repository
- future updates are just commits and pushes
- your deployment history stays tied to your own branch

## What You Need

- Docker and Docker Compose for the local/self-hosted path
- a Resend account only if you want real digest emails
- a Railway account only if you want the hosted cloud path

You do not need Go installed to run the Docker deployment.

## Environment Variables

Copy `.env.example` to `.env` and edit it.

These are the supported runtime variables:

- `DATABASE_URL` (required)
- `ENV` (optional, default `development`)
- `PORT` (optional, default `8080`)
- `SCHEDULER_ENABLED` (optional, default `true`)
- `SCHEDULER_INTERVAL` (optional, default `24h`)
- `SCHEDULER_RUN_ON_START` (optional, default `true`)
- `SCHEDULER_RUN_TIMEOUT` (optional, default `30m`)
- `RESEND_API_KEY` (optional)
- `RESEND_FROM_EMAIL` (optional)
- `RESEND_FROM_NAME` (optional, default `Opportunity Radar`)

## Local Docker Deployment

This path is for running Opportunity Radar on your own machine.

### 1. Clone the repo

```bash
git clone https://github.com/<your-account>/opportunity-radar.git
cd opportunity-radar
```

### 2. Create your env file

```bash
cp .env.example .env
```

Edit `.env` and set the values you want.

For Docker Compose, the default `DATABASE_URL` in `.env.example` already points at the internal `postgres` service:

```env
DATABASE_URL=postgres://postgres:postgres@postgres:5432/opportunity_radar?sslmode=disable
```

### 3. Start the stack

```bash
docker compose up --build
```

If you want it detached:

```bash
docker compose up --build -d
```

### 4. Open the app

Visit:

```text
http://localhost:8080
```

### 5. Complete first-run setup

Use the UI to:
- complete onboarding
- set your role preferences
- set digest recipient and digest settings

### 6. Trigger a manual test run

Use the `Run Once` button in the UI and watch the logs:

```bash
docker compose logs -f app
```

## What To Expect In Docker

- PostgreSQL is internal-only by default
- the app is exposed on `localhost:8080`
- database data persists in the named Docker volume `postgres-data`
- migrations run automatically during app startup

## Updating Later

When you add features later, the normal update flow is:

```bash
git pull
docker compose up --build -d
```

Because migrations run automatically at startup, you should not need a separate migration step for normal updates.

If you want to see logs after an update:

```bash
docker compose logs -f app
```

## Manual Local Validation Checklist

After your first Docker run, you can verify things manually like this:

1. `docker compose up --build`
2. confirm the app logs show startup and migration activity
3. open `http://localhost:8080`
4. complete onboarding
5. save digest settings in the UI
6. click `Run Once`
7. confirm jobs are ingested and the app remains up
8. restart with `docker compose down` then `docker compose up -d`
9. confirm your saved settings are still there

## Railway Deployment

Railway is the recommended hosted deployment path for this repo.

This repo includes [railway.json](railway.json), which tells Railway to:
- build from the root `Dockerfile`
- run one app replica
- keep application sleep disabled
- restart on failure

### 1. Put the code in your own GitHub account

Fork this repo, or push it into a new repository in your own GitHub account.

GitHub-connected deployment is the better default than deploying from your local machine because:

- Railway can auto-deploy new commits from your chosen branch
- deployment history stays tied to commits
- updates later are simpler
- you do not need to keep uploading source from your laptop

### 2. What you will create on Railway

On Railway, you will create one project containing:
- one PostgreSQL service
- one app service connected to this GitHub repository

The PostgreSQL service stores your data.
The app service runs Opportunity Radar.

### 3. Railway setup flow

1. Create a Railway account.
2. Connect your GitHub account to Railway.
3. Create a new Railway project.
4. Add a PostgreSQL service to that project.
5. Add a new service from GitHub and select your repository copy of Opportunity Radar.
6. Let Railway build the app from the committed `Dockerfile`.
7. Set the required app variables in the Railway dashboard.
8. Deploy the app service.
9. Open the Railway-generated app URL.
10. Complete onboarding and digest settings in the UI.

Automatic runs do not begin until the required onboarding fields have been completed in the UI.

### 4. Variables to set on the Railway app service

Add these variables in the app service `Variables` tab:

- `DATABASE_URL=${{Postgres.DATABASE_URL}}`
- `ENV=production`
- `PORT=8080`
- `SCHEDULER_ENABLED=true`
- `SCHEDULER_INTERVAL=24h`
- `SCHEDULER_RUN_ON_START=true`
- `SCHEDULER_RUN_TIMEOUT=30m`
- `RESEND_API_KEY=...`
- `RESEND_FROM_EMAIL=you@example.com`
- `RESEND_FROM_NAME=Opportunity Radar`

`DATABASE_URL=${{Postgres.DATABASE_URL}}` tells Railway to inject the database URL from the PostgreSQL service into the app service.

### 5. Railway update flow later

Once the service is connected to GitHub, the normal update path is:

1. commit your changes
2. push to your tracked branch on GitHub
3. Railway auto-deploys the connected branch

If you later disable GitHub autodeploys, you can still trigger deploys manually from the Railway dashboard.

### 6. Important runtime note

This app contains a process-local scheduler, so it is intended to stay running continuously in Railway rather than sleep between requests. Automatic runs remain paused until onboarding is complete.

## Runtime Behavior

On startup, the app now:
1. connects to PostgreSQL
2. applies any pending SQL migrations automatically
3. starts the admin UI
4. checks whether required onboarding fields are complete before any automatic run
5. runs ingest immediately when scheduler/startup config allows it and setup is complete
6. continues on the configured schedule, but skips automatic runs until setup is complete

Digest recipient, digest lookback, and digest top-N are configured in the UI after first launch.

## Notes

- If `RESEND_API_KEY` or `RESEND_FROM_EMAIL` is missing, digest sends fall back to logging.
- The digest recipient is not configured through env vars in this app. Set it in the UI.
- If you prefer a slower schedule, change `SCHEDULER_INTERVAL` to values like `72h`.
