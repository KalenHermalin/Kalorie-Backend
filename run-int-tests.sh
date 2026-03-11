
#!/bin/bash

# 1. Start the clean test database
docker compose -f docker-compose.test.yml up -d test-db

# 2. Run the tests
# Go test will now enter internal/store, run TestMain (migrations), 
# and then run your Upsert test.
TEST_DB_URL="postgres://kalen_test:password123@localhost:5433/nutrikal_test?sslmode=disable&timezone=UTC" \
go test -v ./internal/store/...

# 3. Shutdown after finishing
docker compose -f docker-compose.test.yml down
