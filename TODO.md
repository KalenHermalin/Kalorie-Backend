# Kalorie Backend — TODO

## Release & repo

- [ ] Check iOS client against renamed error codes and new sync responses (see `API.md`)
- [ ] Test app end to end against staging
- [x] Decide what to do with the `port-python-fastapi` branch
- [x] Merge error-handling branch to main

## Auth

- [ ] Fix accounts merging when a provider returns an empty or unverified email
- [ ] Hash refresh tokens in the database
- [ ] Restrict JWT parsing to the expected signing method
- [ ] Return 503 for OAuth provider network and parse failures
- [ ] Classify provider user-info non-200 responses
- [ ] Return a clearer error for known-but-unconfigured providers (Apple)
- [ ] Add tests for GitHub, Google and Apple providers
- [ ] Remove `isPremium` from access token (check iOS app first)
- [ ] Use or remove `middlewares.GetIsPremium`
- [ ] Configure Apple Developer credentials
- [ ] Add code exchange without PKCE verifier
- [ ] Rename `psotgres_auth.go` typo
- [x] Fix login bad-request field names (all say `refresh`)
- [x] Fix "bodu" typo in auth bad-request messages

## Settings sync

- [x] Fix `GetUserSettings` foreign-key check testing the wrong error variable
- [x] Rename settings egress `is_modified` (true currently means conflict)
- [x] Fix `IngressSyncSettings` returning 200 `null` instead of 304

## Weight log sync

- [ ] Fix weight log for an already-logged date never syncing
- [ ] Return `ERR_SESSION_OVER` from weight egress for deleted accounts (currently 207, all failed)

## Exercise & food log sync

- [x] Fix `GetUserExerciseLogById` nil-pointer panic (every exercise sync conflict returns 500)
- [x] Add store integration tests for exercise/food log upserts
- [x] Load all logs with their sets/entries using a LEFT JOIN in `GetUserExerciseLogs` / `GetUserFoodLogs`
- [x] Remove unreachable user foreign-key checks from store reads and `UpdateUserSettings`
- [ ] Add migration making `food_log_items` `food_id`, `serving_id`, `quantity`, `unit` NOT NULL (check prod for NULLs first)
- [x] Document 200-log batch limit for exercise/food egress in `API.md`
- [x] Fix food/exercise store tests comparing entries/sets by position (join order not guaranteed)
- [x] Add batch size limit to exercise/food egress
- [x] Remove unreachable `ErrLogIdForeignKeyConstraint` checks from store upserts and reads
- [x] Redesign exercise/food log sync as one timestamp per log (see `THOUGHTS.md`)
- [x] Scope exercise/food upserts and their sets/entries to the owning user
- [x] Fix exercise/food store upserts not detecting sync conflicts
- [x] Fix exercise/food sync aborting the whole batch when one log fails (shared transaction)
- [x] Fix exercise sync ignoring transaction errors (`UpsertUserExerciseLogAndSets`)
- [x] Fix food sync returning 500 when the last log in a batch fails
- [x] Fix exercise/food sync returning `null` entries in `new_logs` for logs that synced
- [x] Fix exercise/food egress returning `null` instead of `[]` for empty `new_logs` / `failed_logs`
- [x] Fix nil `lastSynced` panic in `GetUserExerciseLogs` / `GetUserFoodLogs`
- [x] Return 207 from exercise/food egress on partial failure
- [x] Convert exercise log sync service (`UpsertUserExerciseLogAndSets`, `GetUserExerciseLogs`)
- [x] Convert food log sync service (`UpsertUserFoodLogsAndEntries`, `GetUserFoodLogs`)
- [x] Remove soft/hard error handling from exercise and food log sync
- [x] Log failed food log upserts
- [x] Fix exercise sync failure log line (`Sprintf` with `%w`)
- [x] Remove stale TODO comment above `UpsertUserFoodLogsAndEntries`

## Sync (all types)

- [ ] Verify app handles a 304 on its first sync
- [ ] Add tests for user service sync logic
- [ ] Decide whether an invalid `last_synced_at` should return 400 instead of a full resync
- [ ] Optimize log sync queries to filter by `last_synced` in SQL
- [ ] Refactor duplicated sync handler boilerplate
- [ ] Decide whether to merge egress/ingress sync into one endpoint
  - Note: egress only pushes (returns failed IDs + conflict copies, never other devices' changes); ingress only pulls changes since `lastSynced`
- [ ] Rename snake_case variables in user service
- [x] Fix sync upserts using `ON CONFLICT` targets with no matching unique constraint (every sync fails)
- [x] Add store error for foreign-key violations (deleted user syncing logs)
- [x] Fix sync egress handlers setting `Content-Type` after the 207 header

## Analyze (LLM)

- [ ] Add server-side scan limits for analyze endpoints
- [ ] Refactor analyze food/label into one path
- [ ] Refactor shared request setup in Gemini provider
- [x] Fix Gemini provider discarding the underlying error (`ProviderError`, unmarshal)
- [x] Remove OpenRouter wiring from `cmd/main.go` and its dependency from `go.mod`

## Errors & handlers

- [ ] Add tests for `apperrors` (`Is`, `Write`)
- [x] Remove old `AppError` block and commented-out code from `apperrors`
- [x] Convert auth, user, LLM and docs handlers to `Handle`

## Waitlist

- [x] Fix `RemindMe` returning 400 on server errors

## Tests & tooling

- [ ] Skip store integration tests when `TEST_DB_URL` is unset (`go test ./...` shows a false FAIL)
- [x] Run store integration tests (`run-int-tests.sh`) against the new store errors
- [x] Implement tests for MockUserRepo
- [x] Update `MockUserRepo` to match the refactored `UserRepository` (service tests don't compile)
- [x] Fix `TestSignIn_FullFlow` panicking on unimplemented `MockUserRepo` stubs

## Docs

- [x] Update `API.md` for the new error format and sync responses
- [x] Document client sync rules in `API.md` (no logs without sets, set edits bump the log)
