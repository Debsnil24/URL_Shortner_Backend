# URL_Shortner_Backend

Go backend for the Sniply URL shortener. It exposes authenticated APIs for creating and managing short links, plus a public redirect endpoint that resolves short codes.

## Prerequisites

- Go 1.22+
- PostgreSQL 13+
- Environment variables described below

## Environment Variables

Set the following variables (for local development you can export them in your shell):

```
DATABASE_URL=postgres://<user>:<password>@localhost:5432/sniply?sslmode=disable
JWT_SECRET=<replace-with-long-random-secret>
JWT_EXPIRY_HOURS=24
GOOGLE_CLIENT_ID=<optional-for-oauth>
GOOGLE_CLIENT_SECRET=<optional-for-oauth>
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
FRONTEND_URL=http://localhost:3000
SHORT_URL_BASE=http://localhost:8080  # Base URL for short links (defaults to https://www.sniply.co.in)
ENV=development
COOKIE_DOMAIN=
```

## Running Locally

```bash
export $(cat .env | xargs)  # or export the variables manually
go run main.go
```

The server listens on `:8080`, runs database migrations automatically, and registers all routes.

## Key Endpoints

- `POST /auth/register` – create an account; returns user payload and sets `auth_token` cookie.
- `POST /auth/login` – email/password login; issues `auth_token` cookie.
- `GET /auth/me` – requires valid JWT cookie; returns current user.
- `GET /api/urls` – **requires authentication**; lists the caller’s short links. Responds with `{"success": true, "message": "OK", "data": [...]}` where each entry includes the short code, original URL, click count, timestamps, expiry (if any), aggregated visit totals, and the most recent visit metadata.
- `POST /api/shorten` – **requires authentication**; creates a short code owned by the authenticated user.
- `DELETE /api/delete/:code` – **requires authentication**; deletes the short code if the requester owns it.
- `GET /api/urls/:code/stats` – **requires authentication**; returns click totals, visit counts, and the most recent visit metadata for the caller’s short code.
- `GET /:code` – public redirect; returns `302` with `Location` header when the short code is valid, `404` when it does not exist, and `410` when expired. Redirects increment `click_count` and persist a visit record (IP, user-agent, timestamp).

## Testing

Manual testing can be performed with tools such as Postman by exercising the auth flow (`/auth/register`, `/auth/login`), creating short links (`POST /api/shorten`), listing the current user’s links (`GET /api/urls`), invoking the public redirect (`GET /:code` against the backend host on port `8080`), pulling stats (`GET /api/urls/:code/stats`), and deleting links (`DELETE /api/delete/:code`).

## Migrations

Migrations run automatically on startup via `gormigrate`. If you add columns (e.g., the `updated_at` column on `urls`), ensure a new migration exists in `config/migrations.go` so deployments and local runs stay in sync.

## Notes

- `POST /api/shorten`, `GET /api/urls`, and `DELETE /api/delete/:code` enforce ownership using JWT claims.
- Redirect logging uses structured log messages (`event=...`) to simplify operations tracing.
- Redirects increment `click_count` and create an entry in `url_visits` capturing IP and user agent data for analytics.
