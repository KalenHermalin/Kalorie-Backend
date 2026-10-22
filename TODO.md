# Kalorie Backend — TODO

## Current epic: error handling redesign ([ADR 001](docs/adr/001-error-handling.md))

- [ ] Convert exercise log sync service (`UpsertUserExerciseLogAndSets`, `GetUserExerciseLogs`)
- [ ] Convert food log sync service (`UpsertUserFoodLogsAndEntries`, `GetUserFoodLogs`)
- [ ] Remove soft/hard error handling from exercise and food log sync
- [x] Fix Gemini provider discarding the underlying error (`ProviderError`, unmarshal)
- [x] Remove OpenRouter wiring from `cmd/main.go` and its dependency from `go.mod`
- [ ] Convert auth, user, LLM and docs handlers to `Handle`
- [ ] Fix `RemindMe` returning 400 on server errors
- [ ] Add store error for foreign-key violations (deleted user syncing logs)
- [ ] Add tests for `apperrors` (`Is`, `Write`)
- [ ] Update ADR to allow service-level logging of per-item sync failures
- [ ] Remove old `AppError` block and commented-out code from `apperrors`
- [ ] Check iOS client against renamed error codes
- [ ] Run store integration tests (`run-int-tests.sh`) against the new store errors
- [ ] Run `go vet`, `go test`, test app end to end, merge to main

## High priority

- [ ] Fix accounts merging when a provider returns an empty or unverified email
- [ ] Add server-side scan limits for analyze endpoints
- [ ] Fix weight log for an already-logged date never syncing
- [ ] Fix exercise/food sync reporting partial failures as success

## Medium priority

- [ ] Hash refresh tokens in the database
- [ ] Restrict JWT parsing to the expected signing method
- [ ] Add tests for GitHub, Google and Apple providers
- [ ] Add tests for user service sync logic
- [ ] Update `API.md` for the new error format and sync responses
- [ ] Refactor duplicated sync handler boilerplate
- [ ] Return 503 for OAuth provider network and parse failures
- [ ] Classify provider user-info non-200 responses
- [ ] Return a clearer error for known-but-unconfigured providers (Apple)
- [ ] Optimize log sync queries to filter by `last_synced` in SQL
- [ ] Verify app handles a 304 on its first sync

## Low priority

- [ ] Remove `isPremium` from access token (check iOS app first)
- [ ] Use or remove `middlewares.GetUserID` / `GetIsPremium`
- [ ] Refactor shared request setup in Gemini provider
- [ ] Refactor analyze food/label into one path
- [ ] Add code exchange without PKCE verifier
- [ ] Rename snake_case variables in user service
- [ ] Configure Apple Developer credentials
- [ ] Decide what to do with the `port-python-fastapi` branch
