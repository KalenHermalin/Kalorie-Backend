# ADR 001: Error handling and request logging

- **Status:** Accepted, in progress
- **Date:** 2026-10-13

## Context

Error handling grew one handler at a time, and it shows:

- **Errors are logged and translated in every layer.** The same `errors.As` / "fall back to 500"
  block is copied into ~10 handlers. Some failures are logged twice, some never.
- **`AppError` can't carry the original error.** The real cause is either lost (logs show
  `ERR_INTERNAL_SERVER` and nothing else) or concatenated into `Message`, where it reaches the
  client (`AuthService.LogOut`).
- **Shared `Err...` values are definitions and instances at the same time.** Attaching a cause
  to one modifies a package-level pointer shared by every in-flight request (a data race), and
  copying it instead breaks `errors.Is`, which compares pointers.
- **Codes are scattered string literals.** That's how we ended up with typos (`ERR_NO_:LABEL`,
  `ERR_DELETEING_USER`), one code used for two meanings (`ERR_GENERATING_ACCESS_TOKEN`), and the
  same bug returning 401 from one endpoint and 500 from another.
- **Logs can't be traced to a request.** chi's `RequestID` middleware generates an ID that
  nothing reads, and `middleware.Logger` writes plain text while the app writes JSON.
- **Bad requests can't say what was wrong.** The client only ever gets "the request is invalid."

## Decision

The response format stays exactly the same: `{"code": "...", "message": "..."}`, plus an
optional `fields` array. Existing clients keep working.

### 1. Separate error definitions (`Kind`) from instances (`Error`)

New package `internal/apperr`:

```go
// Kind is a definition. Plain value, so it can't be modified through a shared pointer.
type Kind struct {
	Code    string
	Status  int
	Message string // user-facing
}

// All kinds live in one block: one place to check for typos and duplicates.
var (
	Internal     = Kind{Code: "ERR_INTERNAL_SERVER", Status: 500, Message: "An internal server error occurred"}
	BadRequest   = Kind{Code: "ERR_INVALID_REQUEST", Status: 400, Message: "The request is invalid"}
	Unauthorized = Kind{Code: "ERR_UNAUTHORIZED", Status: 401, Message: "Please sign in again"}
	// ...
)

type FieldError struct {
	Field   string `json:"field"`
	Problem string `json:"problem"`
}

// Error is a single failure, created fresh each time.
type Error struct {
	Kind   Kind
	Fields []FieldError // PUBLIC: sent to the client; only for 4xx the caller can fix
	cause  error        // PRIVATE: logged, never sent
}

func New(k Kind, cause error) *Error
func Invalid(fields ...FieldError) *Error // BadRequest with field details
func (e *Error) Error() string            // "CODE: cause" (for logs only)
func (e *Error) Unwrap() error            // exposes cause to errors.Is/As
func Is(err error, k Kind) bool           // errors.As + compare Kind; ignores cause
func Write(w http.ResponseWriter, e *Error)
```

Call sites: `return apperr.New(apperr.NoFood, nil)` and `if apperr.Is(err, apperr.NoFood)`.

**Cause vs. fields:** the cause is what went wrong inside (`pq: connection refused`) and only
goes to the logs. Fields are what the client can fix (`goal_calories: must be positive`) and go
in the response. Never put a raw decoder or driver error into `Fields`: `json` decode errors
include Go type names. Use `*json.UnmarshalTypeError.Field` to build a clean `FieldError` and keep
the raw error as the cause.

### 2. A logger for each request

New package `internal/logx`: `With(ctx, *slog.Logger) context.Context` and
`L(ctx) *slog.Logger` (falls back to `slog.Default()`).

- A request-logging middleware replaces `middleware.Logger`. It stores
  `base.With("request_id", ..., "method", ..., "path", ...)` in the context and logs one JSON
  line per request with the status and duration.
- The `Auth` middleware adds `user_id` to the context logger.
- Informational logs anywhere: `logx.L(ctx).Info("food analyzed", "provider", name)`.

### 3. One place handles errors

```go
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error
func Handle(fn HandlerFunc) http.HandlerFunc
```

`Handle` is the **only** place that logs a failure and writes an error response:
- an error that isn't an `*apperr.Error` becomes `apperr.New(apperr.Internal, err)`, keeping the cause
- status ≥ 500 → `log.Error`, 4xx → `log.Warn`, always with `code` and the full `err` chain
- `apperr.Write` sends `code`, `message`, and `fields` if there are any

### 4. What each layer does

| Layer | Does | Never does |
|---|---|---|
| store | Wraps errors with context (`fmt.Errorf("...: %w", err)`). Turns `sql.ErrNoRows` into `store.ErrNotFound`. | Logs, or knows about HTTP |
| service | Chooses a `Kind` when the client should see a specific error. Wraps and returns anything unexpected. | Logs errors, or builds 500s by hand |
| handler | Decodes the request, calls the service, writes success, `return err` | Logs errors, or checks error types |
| `Handle` | Logs each failure once and writes the error response | — |

### 5. Responses that aren't errors

- **207 partial sync** isn't an error. The service returns a result (`Saved`, `FailedIDs`). The
  handler writes 207 when `FailedIDs` isn't empty and returns `nil`.
- **409 conflict with the server's copy:** the service returns `(serverCopy, apperr.New(SyncConflict, nil))`.
  The handler checks `apperr.Is`, writes 409 with the body, and returns `nil`.

## Consequences

**Good**
- No data race: kinds are values; each failure creates its own error.
- Internal errors can't leak; the cause field is unexported and `Write` never sends it.
- Every log line can be traced to a request and a user.
- Removes ~10 copies of the error-checking block from the handlers.
- Clients get actionable field-level detail on 4xx errors.

**Costs**
- Every handler's signature changes (it now returns `error`), so the migration touches every route.
- Two error packages exist side by side during the migration.

**Possible status changes:** `NoFood`/`NoLabel` 400 → 422, `LLMProvider` 500 → 502. Only make
these if the mobile client doesn't already check for the old values.

## Alternatives considered

- **Keep the shared `*AppError` values and copy them before attaching a cause.** It works, but it
  depends on remembering to copy every time, and needs a custom `Is` method. The `Kind`/`Error`
  split makes the mistake impossible to write.
- **Change everything in one go.** Rejected: ~21 call sites and ~10 handlers would have to change
  before anything compiles, so a design flaw would only show up at the end.

## Migration plan

1. Revert the `NewAppError` signature change so `main` builds.
2. Add `apperr`, `logx`, and `Handle` next to the old `apperrors` package, with tests that
   `apperr.Is` ignores the cause and that `Write` never includes it.
3. Pilot: convert the LLM handlers and `LLMService`. Bad JSON from a provider returns
   `apperr.New(apperr.LLMProvider, err)`, so the fallback loop moves on to the next provider.
4. Convert `auth.go`, then `user.go`, then `docs.go`, one commit each.
5. Delete `internal/apperrors`. Clean up codes (typos, one meaning per code), coordinating with
   the mobile client if it already checks for them.
