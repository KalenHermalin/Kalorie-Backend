# Kalorie API Documentation

## Overview
This is an internal API for the Kalorie fitness tracking app. The server is built in Go and deployed on Heroku via a pipeline with separate staging and production apps, each with its own database.

## Connection Information
**Base URL (dev):** `http://192.168.118.216:8080/`
**Base URL (Staging):** `https://staging.kalorie.fit/`
**Base URL (Production):** `https://kalorie.fit/`

All request and response bodies are JSON unless stated otherwise. All timestamps are RFC 3339 strings
(e.g. `2026-10-29T14:03:12.123456Z`). The server stores timestamps with **microsecond** precision, so
anything finer is rounded.

---

## Authentication
Every endpoint under `/api/v1/` and the authenticated `/api/auth/` endpoints require a **JWT access token**:

| Header | Value | Description |
|--------|-------|-------------|
| Authorization | Bearer &lt;access_token&gt; | The token received from `/api/auth/login` or `/api/auth/refresh`. |

Access tokens are valid for **15 minutes** (`expires_in: 900`). Refresh tokens are valid for **30 days**
and are single use: every refresh returns a new refresh token and invalidates the old one.

The following endpoints do **not** require an access token:
- `POST /api/auth/login`
- `POST /api/auth/refresh` (needs a valid refresh token in the body instead)
- `POST /api/waitlist`
- `GET /api/system/health`

### Auth errors (any authenticated endpoint)
| Status | Code | When | What the client should do |
|--------|------|------|---------------------------|
| 401 | `ERR_UNAUTHORIZED` | No `Authorization` header | Send the access token |
| 400 | `ERR_AUTH_HEADER_MALFORMED` | Header isn't `Bearer <token>` | Fix the header format |
| 401 | `ERR_INVALID_TOKEN` | Access token expired or invalid | Call `/api/auth/refresh`, then retry |
| 401 | `ERR_SESSION_OVER` | The session can't continue (e.g. the account was deleted on another device) | Clear local session and sign in again |

---

## Errors
Every error response has the same shape:
```JSON
{
    "code": string,
    "message": string,
    "fields": [
        { "field": string, "problem": string }
    ]
}
```
- `code` is machine-readable and stable. **Clients should branch on `code`, not on `message` or the HTTP status.**
- `message` is human-readable and safe to show to the user.
- `fields` is only present on some `400` responses, to say which part of the request was invalid. It is omitted otherwise.
- Internal details (database errors, provider errors) are never sent to the client; they are only logged on the server.

### Error codes
| Code | Status | Meaning |
|------|--------|---------|
| `ERR_INVALID_REQUEST` | 400 | The request body or a required field is invalid. May include `fields`. |
| `ERR_AUTH_HEADER_MALFORMED` | 400 | The `Authorization` header is not `Bearer <token>`. |
| `ERR_LOGIN_REQUEST` | 400 | The sign-in code was rejected by the provider (expired, reused, or the user cancelled). Start sign-in again. |
| `ERR_INVALID_PROVIDER` | 400 | The `provider` / `platform` combination is not supported. |
| `ERR_MAX_BATCH_SIZE` | 400 | Too many items in one sync request. |
| `ERR_NO_FOOD` | 400 | No food was found in the image. |
| `ERR_NO_LABEL` | 400 | No nutrition label was found in the image. |
| `ERR_UNAUTHORIZED` | 401 | No access token was sent. |
| `ERR_INVALID_TOKEN` | 401 | The access token is expired or invalid. Refresh and retry. |
| `ERR_SESSION_OVER` | 401 | The session has ended (refresh token expired/invalid/used, or the account no longer exists). Sign in again. |
| `ERR_CONFLICT` | 409 | The email is already on the waitlist. |
| `ERR_INTERNAL_SERVER` | 500 | Unexpected server error. Safe to retry later. |
| `ERR_AUTH_UNAVAILABLE` | 503 | The sign-in provider is temporarily down. Retry in a few minutes. |
| `ERR_LLM_PROVIDER` | 503 | The AI provider is temporarily down. Retry later. |

---

## Sync model
Settings, weight logs, food logs and exercise logs are stored on the device and synced with two endpoints each:

- **Egress** (`PUT .../sync`): the client **pushes** its local changes. The response only reports what happened
  to the items that were sent (conflicts and failures). It never includes changes made on other devices.
- **Ingress** (`GET .../sync?last_synced_at=...`): the client **pulls** everything that changed on the server
  since its last successful sync.

### Conflict resolution: newest `updated_at` wins
- Every item carries an `updated_at` set by the client when it was last changed.
- On egress, the server only saves an item if its `updated_at` is **newer** than the server's copy. If the server's
  copy is the same age or newer, the client's version is discarded and the server's current copy is sent back in
  `new_logs` (or `new` for settings) so the client can replace its local copy.
- Deletes are **soft deletes**: set `deleted_at` (and bump `updated_at`) and sync the item like any other change.
  Items are never removed from the server by sync, so ingress also returns soft-deleted items.

### Food and exercise logs sync as one unit
A food log with its entries, and an exercise log with its sets, are treated as **one item versioned by the parent
log's `updated_at`**. The whole log wins or loses together:
- If the log is newer than the server's copy, the log and every entry/set in the request are saved.
- If it is not newer, **nothing** in that log is saved (including entries/sets), and the server's full copy of the
  log is returned in `new_logs`.

This puts the following rules on the client:
1. **Any change to an entry or set must also bump the parent log's `updated_at`.** Entry/set `updated_at` values are
   stored but are not used to decide conflicts, and ingress only checks the parent log's `updated_at`.
2. **Always send the full entry/set list** for a log, including soft-deleted ones (with `deleted_at` set). An entry/set
   that is left out is not deleted on the server; it is simply left unchanged.
3. **Always send the list itself**, even if it is empty (`[]`). A log sent with a missing or `null` list fails and is
   returned in `failed_logs`.
4. **Entry/set ids belong to one log forever.** Generate a new id when copying or moving an entry/set to another log.
   An entry/set whose id already exists under a different log is ignored.
5. The `log_id` field on entries/sets is ignored on egress; the server always uses the id of the parent log.

### Partial failures
Each log in an egress request is saved in its own transaction, so one failing log never affects the others.
- **200 OK**: every log was either saved or resolved as a conflict.
- **207 Multi-Status**: one or more logs hit an actual error. Their **ids** are listed in `failed_logs`; the client
  should keep them and resend them on the next sync. Conflicts are not failures and still go in `new_logs`.
- An error status (`4xx`/`5xx`) means the request as a whole was rejected and nothing should be assumed saved.

### `last_synced_at`
- Send the time of the last **successful** ingress sync, in RFC 3339 format.
- If it is omitted **or cannot be parsed**, the server treats it as missing and returns **everything** (use this
  for the first sync on a new device).
- `304 Not Modified` (with no body) means there is nothing newer than `last_synced_at`. The client keeps its data and
  should not update `last_synced_at`.

---

## Endpoints

### Authentication Endpoints

#### Login / Sign up
**Path:** `POST /api/auth/login`
**Auth:** none

Handles both registration and login. If no account exists for the provider's user, one is created.

**Request Body:**
```JSON
{
    "code": string,
    "provider": "github" | "google" | "apple",
    "verifier": string,
    "platform": "ios" | "android" (optional)
}
```
- `code` and `verifier` are the OAuth authorization code and its PKCE code verifier. Both are required.
- `platform` selects the platform-specific client for providers that have one (e.g. `google`). `github` and
  `apple` are single providers: omit `platform` for them.

**Success (200 OK):**
```JSON
{
    "access_token": string,
    "refresh_token": string,
    "expires_in": number,
    "user": {
        "id": string,
        "email": string,
        "created_at": string (RFC 3339 timestamp)
    }
}
```
`user.created_at` is always the account's real creation time.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON (`fields: body`), or `code`, `provider` or `verifier` is empty (`fields` names which one) |
| 400 | `ERR_INVALID_PROVIDER` | Unknown `provider`, or a `provider`/`platform` combination that isn't configured |
| 400 | `ERR_LOGIN_REQUEST` | The provider rejected the code (expired, already used, or access denied) |
| 503 | `ERR_AUTH_UNAVAILABLE` | The provider is temporarily unavailable |
| 500 | `ERR_INTERNAL_SERVER` | Provider misconfiguration, network failure, or a server error |

#### Refresh
**Path:** `POST /api/auth/refresh`
**Auth:** none (refresh token in body)

Exchanges a valid refresh token for a new access token and a new refresh token. The old refresh token stops working
immediately, so always store the new one.

**Request Body:**
```JSON
{
    "refresh": string
}
```
**Success (200 OK):** Same response as `/api/auth/login`.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON (`fields: body`) or `refresh` is empty (`fields: refresh`) |
| 401 | `ERR_SESSION_OVER` | Refresh token is expired, invalid, already used, or the account no longer exists. Sign in again. |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Logout
**Path:** `POST /api/auth/logout`
**Auth:** access token

Ends the session by deleting the given refresh token. Logging out with a token that is already gone still succeeds,
so this is safe to retry.

**Request Body:**
```JSON
{
    "refresh": string
}
```
**Success (200 OK):** Plain text.
```text
Success
```
**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON (`fields: body`) or `refresh` is empty (`fields: refresh`) |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Delete Account
**Path:** `DELETE /api/auth/account`
**Auth:** access token

Permanently deletes the user's account and all of its data (settings, weight logs, food logs, exercise logs, refresh
tokens). This cannot be undone. Deleting an account that is already gone still succeeds, so this is safe to retry.

**Request Body:** none.

**Success (200 OK):** Plain text.
```text
Success
```
**Errors:**
| Status | Code | When |
|--------|------|------|
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Settings Endpoints

**Settings object:**
```JSON
{
    "units": string,
    "calories_target": number,
    "protein_target": number,
    "carbs_target": number,
    "fat_target": number,
    "theme": string,
    "updated_at": string (RFC 3339 timestamp),
    "deleted_at": string | null
}
```

#### Egress Sync Settings
**Path:** `PUT /api/v1/settings/sync`
**Auth:** access token

Pushes the device's settings. They are saved only if `updated_at` is newer than the server's copy.

**Request Body:**
```JSON
{
    "settings": <Settings object>
}
```
**Success (200 OK):** Always `200`; check `is_conflict`.
```JSON
{
    "new": <Settings object> | null,
    "is_conflict": boolean
}
```
- `is_conflict: false`, `new: null`: the settings were saved.
- `is_conflict: true`: the server already had newer settings, so the request was discarded. `new` holds the server's
  current settings; replace the local copy with them.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON |
| 401 | `ERR_SESSION_OVER` | The account no longer exists |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Ingress Sync Settings
**Path:** `GET /api/v1/settings/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** access token

Returns the server's settings if they changed after `last_synced_at` (or always, if `last_synced_at` is omitted).

**Success (200 OK):** The settings object.
```JSON
<Settings object>
```
**Not Modified (304):** The settings haven't changed since `last_synced_at`. No body.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 401 | `ERR_SESSION_OVER` | The account no longer exists |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Weight Log Endpoints

**Weight log object:**
```JSON
{
    "id": string (UUID),
    "weight_kg": number,
    "log_date": string (RFC 3339 timestamp),
    "updated_at": string (RFC 3339 timestamp),
    "deleted_at": string | null
}
```
There is **at most one weight log per user per `log_date`**. A log for a date that already has one updates that
date's log (if newer) rather than creating a second one.

> **Known issue:** if the client sends a log for an existing date with a *different* `id` than the server's log and
> the server's copy is newer, that log is currently returned in `failed_logs` instead of `new_logs`.

#### Egress Sync Weight Logs
**Path:** `PUT /api/v1/weight-logs/sync`
**Auth:** access token

Pushes weight log changes. Each log is judged on its own (see [Sync model](#sync-model)).
**Limited to 200 logs per request**; split larger batches into multiple requests.

**Request Body:**
```JSON
{
    "weight_logs": [ <Weight log object>, ... ]
}
```
**Success (200 OK)** / **Partial Success (207 Multi-Status):**
```JSON
{
    "new_logs": [ <Weight log object>, ... ],
    "failed_logs": [ string (log id), ... ]
}
```
- `new_logs`: the server's current copy of every log that lost a conflict. Replace the local copies with them.
- `failed_logs`: ids of logs that hit an error and should be resent. Non-empty only on `207`.
- Both are always arrays (`[]` when empty), never `null`.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON, or `weight_logs` is missing/`null` |
| 400 | `ERR_MAX_BATCH_SIZE` | More than 200 logs in one request |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

If the account no longer exists, each log fails and comes back in `failed_logs`.

#### Ingress Sync Weight Logs
**Path:** `GET /api/v1/weight-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** access token

Returns every weight log (including soft-deleted ones) with `updated_at` after `last_synced_at`, or all of them if
`last_synced_at` is omitted.

**Success (200 OK):**
```JSON
[ <Weight log object>, ... ]
```
**Not Modified (304):** Nothing changed since `last_synced_at` (or the user has no weight logs). No body.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Food Log Endpoints

**Full food log object** (a food log plus its entries):
```JSON
{
    "food_log": {
        "id": string (UUID),
        "name": string,
        "Time": string (RFC 3339 timestamp),
        "created_at": string (RFC 3339 timestamp),
        "updated_at": string (RFC 3339 timestamp),
        "deleted_at": string | null
    },
    "food-log_entries": [
        {
            "id": string (UUID),
            "log_id": string (UUID),
            "food_id": string,
            "food_name": string,
            "serving_id": string,
            "unit": string,
            "quantity": number,
            "cal": number (integer),
            "fat": number,
            "carbs": number,
            "protein": number,
            "updated_at": string (RFC 3339 timestamp),
            "deleted_at": string | null
        }
    ]
}
```
Note the exact key spellings: `Time` (capital T) and `food-log_entries` (hyphen, then underscore).

#### Egress Sync Food Logs
**Path:** `PUT /api/v1/food-logs/sync`
**Auth:** access token

Pushes food log changes. Each food log and its entries are saved as one unit; read
[Food and exercise logs sync as one unit](#food-and-exercise-logs-sync-as-one-unit) for the rules the client must follow.

**Request Body:**
```JSON
{
    "food_logs": [ <Full food log object>, ... ]
}
```
**Success (200 OK)** / **Partial Success (207 Multi-Status):**
```JSON
{
    "new_logs": [ <Full food log object>, ... ],
    "failed_logs": [ string (food log id), ... ]
}
```
- `new_logs`: the server's full copy (log and all entries) of every log that lost a conflict. Replace the local
  copies with them.
- `failed_logs`: ids of logs that hit an error and should be resent. Non-empty only on `207`.
- Both are always arrays (`[]` when empty), never `null`.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON, or `food_logs` is missing/`null` |
| 401 | `ERR_SESSION_OVER` | The account no longer exists |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

#### Ingress Sync Food Logs
**Path:** `GET /api/v1/food-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** access token

Returns every food log (including soft-deleted ones), with all of its entries, whose `food_log.updated_at` is after
`last_synced_at`, or all of them if `last_synced_at` is omitted.

**Success (200 OK):**
```JSON
[ <Full food log object>, ... ]
```
**Not Modified (304):** Nothing changed since `last_synced_at` (or the user has no food logs). No body.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Exercise Log Endpoints

**Full exercise log object** (an exercise log plus its sets):
```JSON
{
    "exercise_log": {
        "id": string (UUID),
        "exercise_id": string,
        "created_at": string (RFC 3339 timestamp),
        "updated_at": string (RFC 3339 timestamp),
        "deleted_at": string | null
    },
    "exercise_sets": [
        {
            "id": string (UUID),
            "log_id": string (UUID),
            "set_number": number (integer),
            "weight": number,
            "reps": number (integer),
            "updated_at": string (RFC 3339 timestamp),
            "deleted_at": string | null
        }
    ]
}
```

#### Egress Sync Exercise Logs
**Path:** `PUT /api/v1/exercise-logs/sync`
**Auth:** access token

Pushes exercise log changes. Each exercise log and its sets are saved as one unit; read
[Food and exercise logs sync as one unit](#food-and-exercise-logs-sync-as-one-unit) for the rules the client must follow.

**Request Body:**
```JSON
{
    "exercise_logs": [ <Full exercise log object>, ... ]
}
```
**Success (200 OK)** / **Partial Success (207 Multi-Status):**
```JSON
{
    "new_logs": [ <Full exercise log object>, ... ],
    "failed_logs": [ string (exercise log id), ... ]
}
```
- `new_logs`: the server's full copy (log and all sets) of every log that lost a conflict. Replace the local copies
  with them.
- `failed_logs`: ids of logs that hit an error and should be resent. Non-empty only on `207`.
- Both are always arrays (`[]` when empty), never `null`.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON, or `exercise_logs` is missing/`null` |
| 401 | `ERR_SESSION_OVER` | The account no longer exists |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

#### Ingress Sync Exercise Logs
**Path:** `GET /api/v1/exercise-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** access token

Returns every exercise log (including soft-deleted ones), with all of its sets, whose `exercise_log.updated_at` is
after `last_synced_at`, or all of them if `last_synced_at` is omitted.

**Success (200 OK):**
```JSON
[ <Full exercise log object>, ... ]
```
**Not Modified (304):** Nothing changed since `last_synced_at` (or the user has no exercise logs). No body.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Analysis Endpoints
Both endpoints send the image to an AI model and can take several seconds. The server gives up after **30 seconds**.

#### Analyze Food
**Path:** `POST /api/v1/analyze/food`
**Auth:** access token

Estimates the nutrition of the food in a photo.

**Request Body:**
```JSON
{
    "picture": string (base64-encoded image bytes),
    "description": string (optional)
}
```
`description` is optional free text passed to the model as extra context (e.g. "grilled, no oil" or a portion
estimate). Omit it or send an empty string if there's nothing to add.

**Success (200 OK):**
```JSON
{
    "food_name": string,
    "cal": number (integer),
    "fat": number (integer),
    "protein": number (integer),
    "carbs": number (integer)
}
```
**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON, `picture` is not valid base64, or no picture was sent |
| 400 | `ERR_NO_FOOD` | No food was found in the image |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 503 | `ERR_LLM_PROVIDER` | The AI provider failed, timed out, or is unavailable |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Analyze Label
**Path:** `POST /api/v1/analyze/label`
**Auth:** access token

Reads the nutrition values from a photo of a nutrition label.

**Request Body:**
```JSON
{
    "picture": string (base64-encoded image bytes)
}
```
**Success (200 OK):**
```JSON
{
    "units": string,
    "base_quantity": number (integer),
    "macros": {
        "cal": number (integer),
        "fat": number (integer),
        "protein": number (integer),
        "carbs": number (integer)
    }
}
```
**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON, `picture` is not valid base64, or no picture was sent |
| 400 | `ERR_NO_LABEL` | No nutrition label was found in the image |
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid access token |
| 503 | `ERR_LLM_PROVIDER` | The AI provider failed, timed out, or is unavailable |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Waitlist Endpoints

#### Remind Me
**Path:** `POST /api/waitlist`
**Auth:** none

Adds an email address to the release waitlist. No email is sent automatically yet; addresses are exported and
emailed manually later.

**Request Body:**
```JSON
{
    "email": string
}
```
**Success (200 OK):** Empty body.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON |
| 409 | `ERR_CONFLICT` | The email is already on the waitlist |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### System Endpoints

#### Health
**Path:** `GET /api/system/health`
**Auth:** none

Simple health check.

**Success (200 OK):** Plain text.
```text
Ok!
```
