# Kalorie API Documentation
## Overview
This is an internal API for the Kalorie fitness tracking app. The server is built in Go and deployed on Heroku via a pipeline with separate staging and production apps, each with its own database.

## Connection Information
**Base URL (dev:)** `http://192.168.118.216:8080/`
**Base URL (Staging):** `https://staging.kalorie.fit/`
**Base URL (Production):** `https://kalorie.fit/`

## Authentication
Most endpoints require a **JWT Access Token** to be sent in the header

| Header | Value | Description |
|--------|-------|-------------|
|Authorization| Bearer <access_token>| Use the token received from the `/api/auth/login` or `/api/auth/refresh` endpoint.|

The following endpoints do not require authorization:  
`/api/auth/login`  
`/api/system/health`  
`/api/auth/refresh` -> note it needs a valid refresh token in the request body  
`/api/waitlist`

## Error Codes
All endpoints will return a standardized `AppError` object when something goes wrong. The `AppError` object has the form of: 
```JSON 
{
    "code": string,
    "message": string
}
```

## Endpoints

### Authentication Endpoints
#### Login / Sign up
**Path:** `POST /api/auth/login`

This endpoint handles both registration and login. If the user doesn't exist, they are created with this request.  
**Request Body:** 
```JSON
 {
    "code": string,
     "provider": string,
    "verifier": string,
    "platform": "ios" | "android" (optional)
 }
 ```
Note that provider can only be one of the setup providers. This includes `github`, `google`, and `apple`. Like `github`, `apple` is a single provider (not split per-platform) - omit `platform` when using it.

**Success (200 OK):** Returns a new auth response object containing the JWT access token and refresh token to be stored on client securely. 
```JSON
{
    "access_token": string,
    "refresh_token": string,
    "expires_in": number,
    "user": {
        "id": string,
        "email": string,
        "created_at": string (RFC3339 timestamp)
    }
}
```
`user.created_at` is always the account's real creation timestamp on this endpoint (it comes from the row that was just inserted/updated).

**Errors:**
- **400 Bad Request:** If the body is invalid json, or if the code or provider is empty
- **500 Internal Server Error:** Indicates an internal error with the auth service or auth providers

#### Refresh
**Path:** `POST /api/auth/refresh` 

This endpoints takes in a valid refresh token and generates a new set of access token and refresh token for the user. Making the old refresh token invalid.  
**Request Body:**
```JSON
{
    "refresh": string
}
```
**Success (200 OK):** Returns a new auth response object containing the JWT access token and refresh token to be stored on client securely.  
```JSON
{
    "access_token": string,
    "refresh_token": string,
    "expires_in": number,
    "user": {
        "id": string,
        "email": string,
        "created_at": string (RFC3339 timestamp)
    }
}
```
**Note:** unlike `/api/auth/login`, this endpoint does not re-fetch the account's creation timestamp from the database - `user.created_at` on a refresh response is always the zero-value Go timestamp (`"0001-01-01T00:00:00Z"`), not the account's real creation date. `user.id` and `user.email` are always accurate. Don't rely on `created_at` here if you need the real value; re-fetch it another way, or only trust it from `/api/auth/login`.

**Errors:**
- **400 Bad Request:** If the request body could not be decoded
- **401 Unauthorized:** If the refresh token is expired, invalid or not found in database

#### Logout
**Path:** `POST /api/auth/logout`

This endpoint invalidates the user's session by deleting the provided refresh token.  
**Request Body:**
```JSON
{
    "refresh": string
}
```
**Success (200 OK):** Returns a simple success message.
```text
Success
```
**Errors:**
- **400 Bad Request:** If the request body is invalid.
- **401 Unauthorized:** If the user's token is invalid.
- **500 Internal Server Error:** If there is an error deleting the token.

#### Delete Account
**Path:** `DELETE /api/auth/account`

Permanently deletes the authenticated user's account and all associated data (settings, weight logs, food logs, exercise logs, refresh tokens). This cannot be undone.

**Request Body:** None.

**Success (200 OK):** Returns a simple success message.
```text
Success
```
**Errors:**
- **401 Unauthorized:** If the access token is missing or invalid.
- **404 Not Found:** If the user no longer exists.
- **500 Internal Server Error:** If there is an error deleting the account.

### Settings Endpoints

#### Egress Sync Settings (esync)
**Path:** `PUT /api/v1/settings/sync`

Pushes updated settings to the server on application close or pause. If the server already has a newer version, a conflict is returned with the latest settings.  

**Request Body:**
```JSON
{
    "settings": {
        "units": string,
        "calories_target": number,
        "protein_target": number,
        "carbs_target": number,
        "fat_target": number,
        "updated_at": string (ISO 8601 (RFC3339) timestamp),
        "deleted_at": string | null,
        "theme": string
    }
}
```
**Success (200 OK):** Returns a simple success status.
```JSON
{
    "status": "success"
}
```
**Conflict (409):** Returns the latest settings when a newer version already exists.
```JSON
{
    "units": string,
    "calories_target": number,
    "protein_target": number,
    "carbs_target": number,
    "fat_target": number,
    "updated_at": string (RFC3339 timestamp),
    "deleted_at": string | null,
    "theme": string
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded.
- **401 Unauthorized:** If the access token is missing or invalid.
- **409 Conflict:** If a newer version of settings already exists.
- **500 Internal Server Error:** If there is an error updating or fetching settings.

#### Ingress Sync Settings (isync)
**Path:** `GET /api/v1/settings/sync?last_synced_at=<RFC3339 timestamp>`

Fetches the most up-to-date settings based on the last synced date. If the server has a newer version, it is returned. If not, a 304 is returned.  

**Success (200 OK):** Returns the latest settings payload.
```JSON
{
    "units": string,
    "calories_target": number,
    "protein_target": number,
    "carbs_target": number,
    "fat_target": number,
    "updated_at": string (RFC3339 timestamp),
    "deleted_at": string | null,
    "theme": string
}
```
**Not Modified (304):** Returned when the server does not have a newer version.
```JSON
{
    "code": string,
    "message": string
}
```
**Errors:**
- **400 Bad Request:** If the last_synced_at query param is missing or invalid.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error fetching settings.

### Weight Log Endpoints

#### Egress Sync Weight Logs
**Path:** `PUT /api/v1/weight-logs/sync`

Pushes weight log updates to the server. Each log is upserted and judged independently (one log
failing to sync has no effect on the others) - the response always reports exactly what happened
to every log in the request, whether the overall result was a full or partial success.

Limited to 200 logs per request - split larger batches into multiple requests.

**Request Body:**
```JSON
{
    "weight_logs": [
        {
            "id": string,
            "weight_kg": number,
            "log_date": string (RFC3339 timestamp),
            "updated_at": string (RFC3339 timestamp),
            "deleted_at": string | null
        }
    ]
}
```
**Success (200 OK):** Every log synced. Returns any logs the server had a newer version of than
what the client sent (same conflict-resolution rule as ingress: whichever `updated_at` is newer
wins) - these are the server's current copies, sent back so the client can update its own local
state to match.
```JSON
{
    "new_logs": [
        {
            "id": string,
            "weight_kg": number,
            "log_date": string (RFC3339 timestamp),
            "updated_at": string (RFC3339 timestamp),
            "deleted_at": string | null
        }
    ],
    "failed_logs": []
}
```
**Partial Success (207 Multi-Status):** One or more logs could not be synced at all (a real
error, not just a newer-version conflict - those still go in `new_logs` as above). `failed_logs`
is a list of the **ids** of the logs that need to be retried (not full log objects) - the client
already has the full data for whatever it sent, so only the id is needed to know what to resend.
```JSON
{
    "new_logs": [ ... ],
    "failed_logs": ["<id>", "<id>"]
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded, or if more than 200 logs were
  submitted in one request (`ERR_BATCH_TOO_LARGE`).
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an internal error unrelated to any specific log
  (e.g. the request itself is invalid) - contrast with per-log failures, which are reported in
  `failed_logs` on a `207` rather than failing the whole request.

#### Ingress Sync Weight Logs
**Path:** `GET /api/v1/weight-logs/sync?last_synced_at=<RFC3339 timestamp>`

Fetches weight logs updated since the last synced date.

**Success (200 OK):** Returns a list of weight logs updated after the provided timestamp.
```JSON
[
    {
        "id": string,
        "weight_kg": number,
        "log_date": string (RFC3339 timestamp),
        "updated_at": string (RFC3339 timestamp),
        "deleted_at": string | null
    }
]
```
**Not Modified (304):** Returned when the server does not have newer weight logs.
**Errors:**
- **400 Bad Request:** If the last_synced_at query param is missing or invalid.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error fetching weight logs.

### Food Log Endpoints

#### Egress Sync Food Logs
**Path:** `PUT /api/v1/food-logs/sync`

Pushes food log updates to the server. The server will upsert each log and entry and may return partial success when some logs fail.

**Request Body:**
```JSON
{
    "food_logs": [
        {
            "food_log": {
                "id": string,
                "name": string,
                "Time": string (RFC3339 timestamp),
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "food-log_entries": [
                {
                    "id": string,
                    "log_id": string,
                    "food_id": string,
                    "food_name": string,
                    "serving_id": string,
                    "unit": string,
                    "cal": number,
                    "fat": number,
                    "carbs": number,
                    "protein": number,
                    "quantity": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ]
}
```
**Success (200 OK):** Returns new logs from the server, plus any failed client logs.
```JSON
{
    "new_logs": [
        {
            "food_log": {
                "id": string,
                "name": string,
                "Time": string (RFC3339 timestamp),
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "food-log_entries": [
                {
                    "id": string,
                    "log_id": string,
                    "food_id": string,
                    "food_name": string,
                    "serving_id": string,
                    "unit": string,
                    "cal": number,
                    "fat": number,
                    "carbs": number,
                    "protein": number,
                    "quantity": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ],
    "failed_logs": [
        {
            "food_log": {
                "id": string,
                "name": string,
                "Time": string (RFC3339 timestamp),
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "food-log_entries": [
                {
                    "id": string,
                    "log_id": string,
                    "food_id": string,
                    "food_name": string,
                    "serving_id": string,
                    "unit": string,
                    "cal": number,
                    "fat": number,
                    "carbs": number,
                    "protein": number,
                    "quantity": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ],
    "error": string | null
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error updating food logs.

#### Ingress Sync Food Logs
**Path:** `GET /api/v1/food-logs/sync?last_synced_at=<RFC3339 timestamp>`

Fetches food logs updated since the last synced date.

**Success (200 OK):** Returns a list of food logs updated after the provided timestamp.
```JSON
[
    {
        "food_log": {
            "id": string,
            "name": string,
            "Time": string (RFC3339 timestamp),
            "created_at": string (RFC3339 timestamp),
            "updated_at": string (RFC3339 timestamp),
            "deleted_at": string | null
        },
        "food-log_entries": [
            {
                "id": string,
                "log_id": string,
                "food_id": string,
                "food_name": string,
                "serving_id": string,
                "unit": string,
                "cal": number,
                "fat": number,
                "carbs": number,
                "protein": number,
                "quantity": number,
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            }
        ]
    }
]
```
**Not Modified (304):** Returned when the server does not have newer food logs.
**Errors:**
- **400 Bad Request:** If the last_synced_at query param is missing or invalid.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error fetching food logs.

### Exercise Log Endpoints

#### Egress Sync Exercise Logs
**Path:** `PUT /api/v1/exercise-logs/sync`

Pushes exercise log updates to the server. The server will upsert each log and its sets and may return partial success when some logs fail.

**Request Body:**
```JSON
{
    "exercise_logs": [
        {
            "exercise_log": {
                "id": string,
                "exercise_id": string,
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "exercise_sets": [
                {
                    "id": string,
                    "log_id": string,
                    "set_number": number,
                    "weight": number,
                    "reps": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ]
}
```
**Success (200 OK):** Returns new logs from the server, plus any failed client logs.
```JSON
{
    "new_logs": [
        {
            "exercise_log": {
                "id": string,
                "exercise_id": string,
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "exercise_sets": [
                {
                    "id": string,
                    "log_id": string,
                    "set_number": number,
                    "weight": number,
                    "reps": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ],
    "failed_logs": [
        {
            "exercise_log": {
                "id": string,
                "exercise_id": string,
                "created_at": string (RFC3339 timestamp),
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            },
            "exercise_sets": [
                {
                    "id": string,
                    "log_id": string,
                    "set_number": number,
                    "weight": number,
                    "reps": number,
                    "updated_at": string (RFC3339 timestamp),
                    "deleted_at": string | null
                }
            ]
        }
    ],
    "error": string | null
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error updating exercise logs.

#### Ingress Sync Exercise Logs
**Path:** `GET /api/v1/exercise-logs/sync?last_synced_at=<RFC3339 timestamp>`

Fetches exercise logs updated since the last synced date.

**Success (200 OK):** Returns a list of exercise logs updated after the provided timestamp.
```JSON
[
    {
        "exercise_log": {
            "id": string,
            "exercise_id": string,
            "created_at": string (RFC3339 timestamp),
            "updated_at": string (RFC3339 timestamp),
            "deleted_at": string | null
        },
        "exercise_sets": [
            {
                "id": string,
                "log_id": string,
                "set_number": number,
                "weight": number,
                "reps": number,
                "updated_at": string (RFC3339 timestamp),
                "deleted_at": string | null
            }
        ]
    }
]
```
**Not Modified (304):** Returned when the server does not have newer exercise logs.
**Errors:**
- **400 Bad Request:** If the last_synced_at query param is missing or invalid.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error fetching exercise logs.

### Analysis Endpoints

#### Analyze Food
**Path:** `POST /api/v1/analyze/food`

Analyzes an image of food to estimate nutritional information.  
**Request Body:**
```JSON
{
    "picture": string (base64 encoded bytes),
    "description": string (optional)
}
```
`description` is optional free text passed alongside the image to the model as extra context (e.g. "grilled, no oil" or a portion estimate) - omit it or send an empty string if there's nothing to add.
**Success (200 OK):** Returns the estimated nutritional content of the food in the image.
```JSON
{
    "food_name": string,
    "cal": number,
    "fat": number,
    "protein": number,
    "carbs": number
}
```
**Errors:**
- **400 Bad Request:** If the request body is invalid or the image cannot be processed.
- **500 Internal Server Error:** If there is an error with the analysis service.

#### Analyze Label
**Path:** `POST /api/v1/analyze/label`

Analyzes an image of a nutrition label to extract nutritional information.  
**Request Body:**
```JSON
{
    "picture": string (base64 encoded bytes)
}
```
**Success (200 OK):** Returns the extracted nutritional content from the label.
```JSON
{
    "units": string,
    "base_quantity": number,
    "macros": {
        "cal": number,
        "fat": number,
        "protein": number,
        "carbs": number
    }
}
```
**Errors:**
- **400 Bad Request:** If the request body is invalid or the image cannot be processed.
- **500 Internal Server Error:** If there is an error with the analysis service.

### Waitlist Endpoints

#### Remind Me
**Path:** `POST /api/waitlist`

Registers an email address to be notified when the app releases. Public endpoint — no access token required. There is no automated email sent yet; addresses are collected here to be exported and emailed manually later.

**Request Body:**
```JSON
{
    "email": string
}
```
**Success (200 OK):** Returns an empty body.

**Conflict (409):** Returned if the email is already on the list.
```JSON
{
    "code": "ERR_ALREADY_ON_LIST",
    "message": "You're already on the list"
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded, or if storing the email fails for any reason other than it already being registered.

### System Endpoints

#### Health
**Path:** `GET /api/system/health`

Simple health check.

**Success (200 OK):** Returns a plain-text response.
```text
Ok!
```
