# 1klas-identity

Identity & Access bounded context for 1klas. Walking skeleton.

## Scope (this skeleton)

- `POST /api/v1/crm/public/identity/sign-up` — create user, set session cookie.
- `POST /api/v1/crm/public/identity/sessions` — sign-in, set session cookie.
- `DELETE /api/v1/crm/public/identity/sessions/current` — sign-out.
- `GET /api/v1/crm/public/identity/profile/me` — current user.
- `GET /healthz` — health probe.
- `GET /openapi.json` — embedded OpenAPI spec.

## Stack

Go 1.22 / net/http / pgx/v5 / squirrel / goose / argon2id / slog / OTel SDK
(no-op exporter) / testcontainers-go.

## Architectural deviations vs CTO mandate (documented per SPEC §"deviations")

- **net/http instead of fasthttp.** Follows the canonical sibling `1klas-gateway`
  which has shipped on net/http. Stack pivot ADR favours fasthttp; this is a
  conscious carry-over from gateway. To be reconciled in a follow-up ADR.
- **No Redis session cache** — Postgres-only lookup at skeleton.
- **No CSRF token** — SameSite=Lax + `__Host-` prefix stand in.
- **No Kafka outbox drainer** — rows accumulate in `identity.outbox_events`.
- **No SumSub applicant create on signup.** User immediately `active`.
- **No email verification.**
- **No PII encryption.** Email is the only stored identifier; not in the
  PII-encrypted set per `ARCHITECTURE.md` §14.3.

## Local development

```bash
cp .env.example .env
docker compose up -d postgres
make build
make run
```

Then smoke-test:

```bash
# Sign up — creates user, sets cookie
curl -i -X POST http://localhost:8080/api/v1/crm/public/identity/sign-up \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"correct horse battery"}' \
  --cookie-jar /tmp/cj

# Profile
curl -i http://localhost:8080/api/v1/crm/public/identity/profile/me \
  --cookie /tmp/cj

# Sign out
curl -i -X DELETE http://localhost:8080/api/v1/crm/public/identity/sessions/current \
  --cookie /tmp/cj --cookie-jar /tmp/cj

# Sign in
curl -i -X POST http://localhost:8080/api/v1/crm/public/identity/sessions \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"correct horse battery"}' \
  --cookie-jar /tmp/cj
```

For local HTTP (non-TLS) testing, set `COOKIE_SECURE=false` in `.env` and use
`Set-Cookie` cookies named `__Host-session` are otherwise rejected by clients
without HTTPS. In that mode the cookie name falls back to `session`.

## Tests

```bash
make test            # unit + integration + e2e (requires Docker)
make test-unit       # unit only, no Docker
```

Integration / e2e tests use `testcontainers-go` to spin up a fresh Postgres
container per test, apply all `migrations/`, and exercise the HTTP surface
end-to-end.

## Migrations

Goose. Files in `migrations/`. Applied automatically at service start.

The standalone `cmd/migrate` binary (built as `bin/migrate` in the Docker
image) is the same code path, used as a DO App Platform pre-deploy job so
each deployment applies pending migrations before the server starts.

```bash
./bin/migrate           # equivalent to up
./bin/migrate up
./bin/migrate up-to <ver>
./bin/migrate status
./bin/migrate -h        # usage
```

The binary logs JSON to stderr (slog) and exits non-zero on any failure,
so the pre-deploy job blocks the release on a migration error. A 5-minute
deadline is applied to the whole operation; a hung migration tears the
pre-deploy job down rather than blocking the deployment forever.

## First-time deploy steps

DO App Platform pre-deploy jobs clone from the configured branch (`main`).
That means on the **very first deploy**, the branch this README lives on
must already be merged to `main` before `doctl apps create --spec .do/app.yaml`
is run — otherwise the pre-deploy job will pull a `main` tip that has no
`bin/migrate` binary in its Dockerfile output, and the deploy will fail.

Order of operations:

1. Merge `feat/identity-deploy-prep` to `main`.
2. Confirm `main` builds locally: `docker build -t identity:firstdeploy .`.
3. `doctl apps create --spec .do/app.yaml` — DO will then clone `main`,
   build the image, run the `migrate` pre-deploy job, and finally start
   the `identity` service.
4. For every subsequent deploy, `deploy_on_push: true` handles the rest.

## Configuration

See `.env.example`. All env vars required at runtime are documented there.
