# Kalorie API Documentation
## Overview
This is an internal API for the Kalorie fitness tracking app. The server is built in Go and currently deployed on DigitalOcean

## Connection Information
**Base URL (dev:)** `http://192.168.118.216:8080/`
**Base URL (Production):** `https://whale-app-2bxfv.ondigitalocean.app/`

## Authentication
Most endpoints require a **JWT Access Token** to be sent in the header

| Header | Value | Description |
|--------|-------|-------------|
|Authorization| Bearer <access_token>| Use the token received from the `/auth/login` or `/auth/refresh` endpoint.|

The following endpoints do not require authorization:  
`/auth/login`  
`/system/health`  
`/auth/refresh` -> note it needs a valid refresh token in the request body

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
**Path:** `POST /auth/login`

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
Note that provider can only be one of the setup providers. This includes `github` and `google`.

**Success (200 OK):** Returns a new auth response object containing the JWT access token and refresh token to be stored on client securely. 
```JSON
{
    "access_token": string,
    "refresh_token": string,
    "expires_in": number,
    "User": {
        "id": string,
        "email": string,
        "created_at": string (RFC3339 timestamp)
    }
}
```
**Errors:**
- **400 Bad Request:** If the body is invalid json, or if the code or provider is empty
- **500 Internal Server Error:** Indicates an internal error with the auth service or auth providers

#### Refresh
**Path:** `POST /auth/refresh` 

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
    "User": {
        "id": string,
        "email": string,
        "created_at": string (RFC3339 timestamp)
    }
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded
- **401 Unauthorized:** If the refresh token is expired, invalid or not found in database

#### Logout
**Path:** `POST /auth/logout`

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

### Settings Endpoints

#### Egress Sync Settings (esync)
**Path:** `PUT /v1/settings/sync`

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
**Path:** `GET /v1/settings/sync?last_synced_at=<RFC3339 timestamp>`

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
**Path:** `PUT /v1/weight-logs/sync`

Pushes weight log updates to the server. The server will upsert each log by id and may return partial success when some logs fail.

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
**Success (200 OK):** Returns new logs from the server, plus any failed client logs.
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
    "failed_logs": [
        {
            "id": string,
            "weight_kg": number,
            "log_date": string (RFC3339 timestamp),
            "updated_at": string (RFC3339 timestamp),
            "deleted_at": string | null
        }
    ],
    "error": string | null
}
```
**Errors:**
- **400 Bad Request:** If the request body could not be decoded.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error updating weight logs.

#### Ingress Sync Weight Logs
**Path:** `GET /v1/weight-logs/sync?last_synced_at=<RFC3339 timestamp>`

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
**Path:** `PUT /v1/food-logs/sync`

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
**Path:** `GET /v1/food-logs/sync?last_synced_at=<RFC3339 timestamp>`

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
**Path:** `PUT /v1/exercise-logs/sync`

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
**Path:** `GET /v1/exercise-logs/sync?last_synced_at=<RFC3339 timestamp>`

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
**Path:** `POST /v1/analyze/food`

Analyzes an image of food to estimate nutritional information.  
**Request Body:**
```JSON
{
    "picture": string (base64 encoded bytes)
}
```
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
**Path:** `POST /v1/analyze/label`

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

### System Endpoints

#### Health
**Path:** `GET /system/health`

Simple health check.

**Success (200 OK):** Returns a plain-text response.
```text
Ok!
```
