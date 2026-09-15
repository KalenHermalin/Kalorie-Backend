
#!/bin/bash

# 1. Start the clean test database
docker compose -f docker-compose.test.yml up -d test-db

# 2. Run the tests
# pytest will enter app/store, run the `engine` fixture (migrations),
# and then run the store integration tests.
TEST_DB_URL="postgresql://kalen_test:password123@localhost:5433/nutrikal_test?sslmode=disable" \
.venv/bin/pytest -v app/store/test_postgres_user.py

# 3. Shutdown after finishing
docker compose -f docker-compose.test.yml down
