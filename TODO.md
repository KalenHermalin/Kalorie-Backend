# Kalorie Backend — TODO

## Current epic

- [ ] Redesign exercise/food log sync as one timestamp per log (see `THOUGHTS.md`)
- [ ] Add store integration tests for exercise/food log upserts
- [ ] Add store error for foreign-key violations (deleted user syncing logs)
- [ ] Add tests for `apperrors` (`Is`, `Write`)
- [ ] Remove old `AppError` block and commented-out code from `apperrors`
- [ ] Check iOS client against renamed error codes
- [ ] Run `go vet`, `go test`, test app end to end, merge to main
- [x] Implement tests for MockUserRepo
- [x] Fix sync egress handlers setting `Content-Type` after the 207 header
- [x] Fix "bodu" typo in auth bad-request messages
- [x] Run store integration tests (`run-int-tests.sh`) against the new store errors
- [x] Fix `TestSignIn_FullFlow` panicking on unimplemented `MockUserRepo` stubs
- [x] Fix sync upserts using `ON CONFLICT` targets with no matching unique constraint (every sync fails)
- [x] Fix login bad-request field names (all say `refresh`)
- [x] Rename settings egress `is_modified` (true currently means conflict)
- [x] Fix exercise/food egress returning `null` instead of `[]` for empty `new_logs` / `failed_logs`
- [x] Fix food sync returning 500 when the last log in a batch fails
- [x] Fix exercise/food sync returning `null` entries in `new_logs` for logs that synced
- [x] Fix nil `lastSynced` panic in `GetUserExerciseLogs` / `GetUserFoodLogs`
- [x] Fix exercise sync ignoring transaction errors (`UpsertUserExerciseLogAndSets`)
- [x] Fix `IngressSyncSettings` returning 200 `null` instead of 304
- [x] Fix exercise/food store upserts not detecting sync conflicts
- [x] Fix exercise/food sync aborting the whole batch when one log fails (shared transaction)
- [x] Return 207 from exercise/food egress on partial failure
- [x] Update `MockUserRepo` to match the refactored `UserRepository` (service tests don't compile)
- [x] Log failed food log upserts
- [x] Fix exercise sync failure log line (`Sprintf` with `%w`)
- [x] Remove stale TODO comment above `UpsertUserFoodLogsAndEntries`
- [x] Convert exercise log sync service (`UpsertUserExerciseLogAndSets`, `GetUserExerciseLogs`)
- [x] Convert food log sync service (`UpsertUserFoodLogsAndEntries`, `GetUserFoodLogs`)
- [x] Convert auth, user, LLM and docs handlers to `Handle`
- [x] Remove soft/hard error handling from exercise and food log sync
- [x] Fix Gemini provider discarding the underlying error (`ProviderError`, unmarshal)
- [x] Fix `RemindMe` returning 400 on server errors
- [x] Remove OpenRouter wiring from `cmd/main.go` and its dependency from `go.mod`

## High priority

- [ ] Fix accounts merging when a provider returns an empty or unverified email
- [ ] Add server-side scan limits for analyze endpoints
- [ ] Fix weight log for an already-logged date never syncing

## Medium priority

- [ ] Scope exercise/food upserts and their sets/entries to the owning user
- [ ] Hash refresh tokens in the database
- [ ] Restrict JWT parsing to the expected signing method
- [ ] Update `API.md` for the new error format and sync responses
- [ ] Document client sync rules in `API.md` (no logs without sets, set edits bump the log)
- [ ] Verify app handles a 304 on its first sync
- [ ] Add tests for user service sync logic
- [ ] Add tests for GitHub, Google and Apple providers
- [ ] Return 503 for OAuth provider network and parse failures
- [ ] Classify provider user-info non-200 responses
- [ ] Return a clearer error for known-but-unconfigured providers (Apple)
- [ ] Optimize log sync queries to filter by `last_synced` in SQL
- [ ] Refactor duplicated sync handler boilerplate
- [ ] Decide whether to merge egress/ingress sync into one endpoint
  - Note: egress only pushes (returns failed IDs + conflict copies, never other devices' changes); ingress only pulls changes since `lastSynced`

## Low priority

- [ ] Remove `isPremium` from access token (check iOS app first)
- [ ] Use or remove `middlewares.GetIsPremium`
- [ ] Configure Apple Developer credentials
- [ ] Refactor analyze food/label into one path
- [ ] Refactor shared request setup in Gemini provider
- [ ] Rename snake_case variables in user service
- [ ] Skip store integration tests when `TEST_DB_URL` is unset (`go test ./...` shows a false FAIL)
- [ ] Add code exchange without PKCE verifier
- [ ] Decide what to do with the `port-python-fastapi` branch
