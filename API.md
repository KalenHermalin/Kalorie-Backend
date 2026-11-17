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
Sign-in is handled by **Firebase Authentication** (Google, Apple and GitHub) in the app. The server never sees
provider codes or passwords and issues no tokens of its own.

Every endpoint under `/api/v1/` and `DELETE /api/auth/account` require the user's **Firebase ID token**:

| Header | Value | Description |
|--------|-------|-------------|
| Authorization | Bearer &lt;firebase_id_token&gt; | The current ID token from the Firebase SDK (`getIDToken()`). |

- Firebase ID tokens are valid for **1 hour**. The Firebase SDK refreshes them on its own; always read the current
  token from the SDK right before a request rather than storing it.
- The account must already exist on the server: call [`POST /api/auth/login`](#login--sign-up) once after every
  Firebase sign-in. A valid Firebase token for a user the server doesn't know returns `ERR_SESSION_OVER`.
- **Logging out** is done only in the app (Firebase sign-out). There is no server endpoint for it.

The following endpoints do **not** require the `Authorization` header:
- `POST /api/auth/login` (takes the Firebase ID token in the body instead)
- `POST /api/waitlist`
- `GET /api/system/health`

### Auth errors (any authenticated endpoint)
| Status | Code | When | What the client should do |
|--------|------|------|---------------------------|
| 401 | `ERR_UNAUTHORIZED` | No `Authorization` header | Send the Firebase ID token |
| 400 | `ERR_AUTH_HEADER_MALFORMED` | Header isn't `Bearer <token>` | Fix the header format |
| 401 | `ERR_INVALID_TOKEN` | Firebase ID token expired or invalid | Force-refresh the token in the Firebase SDK, then retry once |
| 401 | `ERR_SESSION_OVER` | No account on the server for this Firebase user (e.g. the account was deleted, or `/api/auth/login` was never called) | Sign out of Firebase and sign in again |

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
| `ERR_MAX_BATCH_SIZE` | 400 | Too many items in one sync request. |
| `ERR_NO_FOOD` | 400 | No food was found in the image. |
| `ERR_NO_LABEL` | 400 | No nutrition label was found in the image. |
| `ERR_UNAUTHORIZED` | 401 | No Firebase ID token was sent. |
| `ERR_INVALID_TOKEN` | 401 | The Firebase ID token is expired or invalid. Force-refresh it in the Firebase SDK and retry. |
| `ERR_SESSION_OVER` | 401 | The server has no account for this Firebase user (deleted, or never signed in). Sign in again. |
| `ERR_CONFLICT` | 409 | The email is already on the waitlist. |
| `ERR_INTERNAL_SERVER` | 500 | Unexpected server error. Safe to retry later. |
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
**Auth:** none (Firebase ID token in body)

Call this once after every Firebase sign-in. It registers the user on the first call and is a no-op login after
that, so it is safe to call repeatedly. Until it has been called, every other authenticated endpoint returns
`ERR_SESSION_OVER` for this user.

**Request Body:**
```JSON
{
    "token_id": string
}
```
- `token_id` is the user's current Firebase ID token.
- The user's email is stored only if Firebase reports it as **verified**. It is updated on every sign-in, so an
  email that is missing or unverified at sign-in clears the stored one.

**Success (200 OK):**
```JSON
{
    "user_id": string (UUID)
}
```
- `user_id` is the server's id for this user. It is **not** the Firebase uid. It is the user id used to build weight
  log ids (see [Weight Log Endpoints](#weight-log-endpoints)), so store it on the device.
- The same `user_id` is returned on every sign-in for the same Firebase user.

**Errors:**
| Status | Code | When |
|--------|------|------|
| 400 | `ERR_INVALID_REQUEST` | Body is not valid JSON (`fields: body`), or `token_id` is empty (`fields: token_id`) |
| 401 | `ERR_INVALID_TOKEN` | The Firebase ID token is expired or invalid. Force-refresh it and retry. |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Logout and token refresh (app only)
There are **no** server endpoints for logging out or refreshing tokens. Both are handled entirely in the app by the
Firebase SDK:

- **Token refresh:** the Firebase SDK keeps its own refresh token on the device and fetches a new ID token before
  the old one expires. Always ask the SDK for the current ID token right before a request (`getIDToken()`); never
  store it yourself. If a request returns `ERR_INVALID_TOKEN`, ask the SDK for a **force-refreshed** token
  (`getIDTokenForcingRefresh(true)` on iOS) and retry once.
- **Logout:** sign out with the Firebase SDK (`Auth.auth().signOut()` on iOS) and clear local data as needed. The
  server keeps no session to end. The user's account and data stay on the server until they sign in again.
- **Signing out of all devices** is not supported by the server. An ID token that was already issued stays valid
  until it expires (up to 1 hour).

#### Delete Account
**Path:** `DELETE /api/auth/account`
**Auth:** Firebase ID token

Permanently deletes the user's account and all of its data (settings, weight logs, food logs, exercise logs) and
their Firebase user. This cannot be undone.

For users who signed in with Apple, the app must also revoke the Apple token (through the Firebase SDK, which needs a
fresh Sign in with Apple) as part of the delete flow; the server does not do this.

**Request Body:** none.

**Success (200 OK):** Plain text.
```text
Success
```
After a successful delete, the same Firebase token gets `ERR_SESSION_OVER` on every endpoint, including this one. A
client retrying a delete that already went through should treat `ERR_SESSION_OVER` as "already deleted".

**Errors:**
| Status | Code | When |
|--------|------|------|
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token, or the account is already gone |
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
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Ingress Sync Settings
**Path:** `GET /api/v1/settings/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
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
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

If the account no longer exists, each log fails and comes back in `failed_logs`.

#### Ingress Sync Weight Logs
**Path:** `GET /api/v1/weight-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
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
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

#### Ingress Sync Food Logs
**Path:** `GET /api/v1/food-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
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
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 500 | `ERR_INTERNAL_SERVER` | Server error not tied to a specific log |

#### Ingress Sync Exercise Logs
**Path:** `GET /api/v1/exercise-logs/sync?last_synced_at=<RFC 3339 timestamp>`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

---

### Analysis Endpoints
Both endpoints send the image to an AI model and can take several seconds. The server gives up after **30 seconds**.

#### Analyze Food
**Path:** `POST /api/v1/analyze/food`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
| 503 | `ERR_LLM_PROVIDER` | The AI provider failed, timed out, or is unavailable |
| 500 | `ERR_INTERNAL_SERVER` | Server error |

#### Analyze Label
**Path:** `POST /api/v1/analyze/label`
**Auth:** Firebase ID token

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
| 4xx | see [Auth errors](#auth-errors-any-authenticated-endpoint) | Missing or invalid token |
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
