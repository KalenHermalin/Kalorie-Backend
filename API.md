# Kalorie API Documentation
## Overview
This is an internal API for the Kalorie fitness tracking app. The server is built in Go and currently deployed on DIgitalOcean

## Connection Information
**Base URL (dev:)** `localhost:8080/`
**Base URL (Production):** `https://whale-app-2bxfv.ondigitalocean.app/`

## Authentication
Most endpoints require a **JWT Access Token** to be sent in the header

| Header | Value | Description |
|--------|-------|-------------|
|Authorization| Bearer <access_token>| Use the token received from the /`auth/signin` or `/auth/refresh` endpoint.

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
#### Login in / Sign up
**Path:** `POST /auth/login`

This endpoint handles both registration and login. If the user doesn't exist, they are created with this request.  
**Request Body:** 
```JSON
 {
    "code": string,
    "provider": string
    "verifier": string
    "platform": "ios" | "android"
 }
 ```
Note that provider can only be one of the setup providers. This include `github`, `google`, and `apple`.  

**Success (200 OK):** Returns a new auth response object containing the JWT access token and refresh token to be stored on client securely. 
```JSON
{
    "access_token": string,
    "refresh_token": string,
    "expires_in": number,
}
```
**Errors:**
- **400 Bad Request:** If the body is invalid json, or if the code or provider and empty
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
- **401 Unauthorized:** If the user is token is invalid.
- **500 Internal Server Error:** If there is an error deleting the token.

### Settings Endpoints

#### Egress Sync Settings (esync)
**Path:** `PUT /v1/settings/esync`

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
**Path:** `GET /v1/settings/isync?last_synced_at=<RFC3339 timestamp>`

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
**Errors:**
- **400 Bad Request:** If the last_synced_at query param is missing or invalid.
- **401 Unauthorized:** If the access token is missing or invalid.
- **500 Internal Server Error:** If there is an error fetching settings.

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
