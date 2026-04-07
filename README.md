# Opportunity Radar

Opportunity Radar is a self-hosted Go app that collects jobs, scores them against your preferences, stores them in PostgreSQL, and sends digest emails when configured.

This repo is designed for single-user deployment:
- one app instance
- one database
- one operator
- self-hosted on your own machine or your own cloud account

There is no shared central server, no multi-user account system, and no SaaS control plane.

## What You Need

- Docker and Docker Compose for the local/self-hosted path
- a Resend account only if you want real digest emails
- your own Fly.io account only if you want the cloud path

You do not need Go installed to run the Docker deployment.

## Runtime Behavior

On startup, the app now:
1. connects to PostgreSQL
2. applies any pending SQL migrations automatically
3. starts the admin UI
4. runs ingest immediately when scheduler/startup config allows it
5. continues on the configured schedule

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

## Fly.io Deployment

Fly.io is the recommended cloud path in this repo instead of Railway.

This repo includes a starter [fly.toml.example](fly.toml.example). Copy it to `fly.toml` and edit the placeholder app name before deploying.

### Important scheduler note

Opportunity Radar is a scheduled app, not a request-only app. Your Fly machine must stay running for the scheduler to fire on time.

The Fly config therefore keeps one machine running:
- `auto_stop_machines = "off"`
- `min_machines_running = 1`

### Basic Fly setup flow

1. Install `flyctl`
2. Sign in to Fly
3. Copy the config:

```bash
cp fly.toml.example fly.toml
```

4. Edit `fly.toml`
5. Launch the app:

```bash
fly launch --no-deploy
```

6. Create a Postgres cluster
7. Attach or otherwise provide a `DATABASE_URL`
8. Set app secrets:

```bash
fly secrets set \
  DATABASE_URL="your_database_url" \
  RESEND_API_KEY="your_resend_key" \
  RESEND_FROM_EMAIL="you@example.com" \
  RESEND_FROM_NAME="Opportunity Radar"
```

9. Deploy:

```bash
fly deploy
```

10. Open the app, complete onboarding, and set digest recipient/settings in the UI

### Fly update flow later

```bash
git pull
fly deploy
```

## Notes

- If `RESEND_API_KEY` or `RESEND_FROM_EMAIL` is missing, digest sends fall back to logging.
- The digest recipient is not configured through env vars in this app. Set it in the UI.
- If you prefer a slower schedule, change `SCHEDULER_INTERVAL` to values like `72h`.
