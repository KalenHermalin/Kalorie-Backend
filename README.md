# Kalorie Backend

API server for the Kalorie fitness tracking app (Python / FastAPI). See [API.md](API.md) for the endpoint reference.

## Requirements

- Python 3.13+
- Docker + Docker Compose (for Postgres, and for the containerized dev server)

## Setup

```bash
python3 -m venv .venv
./.venv/bin/pip install -r requirements.txt
```

Create a `.env` file in the repo root with the variables below.

### Environment variables

| Variable | Description |
|---|---|
| `PORT` | Port the API listens on |
| `DB_DNS` | Postgres connection string, e.g. `postgresql://user:pass@db:5432/dbname` |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Used by the `db` container in `docker-compose.dev.yml` |
| `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | Secrets used to sign access/refresh tokens |
| `GIT_CLIENT_ID` / `GIT_CLIENT_SECRET` | GitHub OAuth app credentials |
| `GOOGLE_CLIENT_ID_IOS` / `GOOGLE_CLIENT_ID_ANDROID` | Google OAuth client IDs per platform |
| `APPLE_CLIENT_ID` *(optional)* | Apple Services ID / app Bundle ID registered for Sign in with Apple |
| `APPLE_TEAM_ID` *(optional)* | Apple Developer Team ID |
| `APPLE_KEY_ID` *(optional)* | Key ID of the Sign in with Apple private key |
| `APPLE_PRIVATE_KEY` *(optional)* | The `.p8` private key content (PEM). If stored on one line, escape newlines as `\n` |

Unlike the other credentials, the four `APPLE_*` vars are optional - if any are missing, the app still starts, it just won't register Apple as a login provider (`"provider": "apple"` on `/auth/login` returns `ERR_INVALID_PROVIDER` until all four are set).
| `LLM_MODEL` | Gemini model name (e.g. `gemini-2.0-flash`) |
| `LLM_API_KEY` | Gemini API key |

## Running the dev server

```bash
./run-dev-server.sh
```

This runs `docker compose -f docker-compose.dev.yml up`, which starts Postgres, runs migrations on boot, and serves the API at `http://localhost:8080` with live reload (the app restarts automatically on code changes).

To run it outside Docker instead (with your own Postgres reachable via `DB_DNS`):

```bash
./.venv/bin/python -m app.main
```

## Running the integration tests

```bash
./run-int-tests.sh
```

This spins up a throwaway Postgres container (`docker-compose.test.yml`), runs migrations against it, runs the store-layer integration tests in `app/store/test_postgres_user.py`, then tears the container down.

## Running the unit tests

The rest of the test suite doesn't need a database or Docker:

```bash
./.venv/bin/pytest app --ignore=app/store/test_postgres_user.py
```

(or `./.venv/bin/pytest app` to run everything, once a test database is up per the previous section)
