# Opportunity Radar

Opportunity Radar is a self-hosted Go app that collects jobs, scores them against your preferences, stores them in PostgreSQL, and sends digest emails when configured.

For technical architecture, design constraints, and future-direction notes, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

This repo is designed for single-user deployment:
- one app instance
- one database
- one operator
- self-hosted on your own machine or your own cloud account

There is no shared central server, no multi-user account system, and no SaaS control plane.

## What You Need

- Docker and Docker Compose for the local/self-hosted path
- a Resend account only if you want real digest emails
- a Railway account only if you want the hosted cloud path

You do not need Go installed to run the Docker deployment.

## Runtime Behavior

On startup, the app now:
1. connects to PostgreSQL
2. applies any pending SQL migrations automatically
3. starts the admin UI
4. checks whether required onboarding fields are complete before any automatic run
5. runs ingest immediately when scheduler/startup config allows it and setup is complete
6. continues on the configured schedule, but skips automatic runs until setup is complete

Digest recipient, digest lookback, and digest top-N are configured in the UI after first launch.

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

### 1. Create your env file

```bash
cp .env.example .env
```

Edit `.env` and set the values you want.

For Docker Compose, the default `DATABASE_URL` in `.env.example` already points at the internal `postgres` service:

```env
DATABASE_URL=postgres://postgres:postgres@postgres:5432/opportunity_radar?sslmode=disable
```

### 2. Start the stack

```bash
docker compose up --build
```

If you want it detached:

```bash
docker compose up --build -d
```

### 3. Open the app

Visit:

```text
http://localhost:8080
```

### 4. Complete first-run setup

Use the UI to:
- complete onboarding
- set your role preferences
- set digest recipient and digest settings

### 5. Trigger a manual test run

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

### Why GitHub deploy is preferred

For this project, GitHub-connected deployment is the better default than deploying from your local machine because:
- Railway can auto-deploy new commits from your chosen branch
- deployment history stays tied to commits
- updates later are simpler
- you do not need to keep uploading source from your laptop

### What you will create on Railway

On Railway, you will create one project containing:
- one PostgreSQL service
- one app service connected to this GitHub repository

The PostgreSQL service stores your data.
The app service runs Opportunity Radar.

### Railway setup flow

1. Create a Railway account.
2. Connect your GitHub account to Railway.
3. Create a new Railway project.
4. Add a PostgreSQL service to that project.
5. Add a new service from GitHub and select this repository.
6. Let Railway build the app from the committed `Dockerfile`.
7. Set the required app variables in the Railway dashboard.
8. Deploy the app service.
9. Open the Railway-generated app URL.
10. Complete onboarding and digest settings in the UI.

Automatic runs do not begin until the required onboarding fields have been completed in the UI.

### Variables to set on the Railway app service

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

### Railway update flow later

Once the service is connected to GitHub, the normal update path is:

1. commit your changes
2. push to GitHub
3. Railway auto-deploys the connected branch

If you later disable GitHub autodeploys, you can still trigger deploys manually from the Railway dashboard.

### Important runtime note

This app contains a process-local scheduler, so it is intended to stay running continuously in Railway rather than sleep between requests. Automatic runs remain paused until onboarding is complete.

## Notes

- If `RESEND_API_KEY` or `RESEND_FROM_EMAIL` is missing, digest sends fall back to logging.
- The digest recipient is not configured through env vars in this app. Set it in the UI.
- If you prefer a slower schedule, change `SCHEDULER_INTERVAL` to values like `72h`.
