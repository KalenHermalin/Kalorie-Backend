package service

import (
	"context"
	"database/sql"
	"errors"
	"main/internal/auth"
	"main/internal/models"
	"main/internal/utils"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type MockUserRepo struct {
	mockUser    *models.User
	mockRefresh string
	mockErr     error
}

// DeleteUser implements [models.UserRepository].
func (m *MockUserRepo) DeleteUser(ctx context.Context, tx *sql.Tx, userID string) error {
	panic("unimplemented")
}

// GetUserExerciseLogById implements [models.UserRepository].
func (m *MockUserRepo) GetUserExerciseLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*models.ExerciseLog, error) {
	panic("unimplemented")
}

// GetUserExerciseLogs implements [models.UserRepository].
func (m *MockUserRepo) GetUserExerciseLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.FullExerciseLog, error) {
	panic("unimplemented")
}

// GetUserExerciseSetsByLogId implements [models.UserRepository].
func (m *MockUserRepo) GetUserExerciseSetsByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*models.ExerciseSet, error) {
	panic("unimplemented")
}

// GetUserFoodLogById implements [models.UserRepository].
func (m *MockUserRepo) GetUserFoodLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*models.FoodLog, error) {
	panic("unimplemented")
}

// GetUserFoodLogEntriesByLogId implements [models.UserRepository].
func (m *MockUserRepo) GetUserFoodLogEntriesByLogId(ctx context.Context, tx *sql.Tx, logId string) ([]*models.FoodLogEntry, error) {
	panic("unimplemented")
}

// GetUserFoodLogs implements [models.UserRepository].
func (m *MockUserRepo) GetUserFoodLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.FullFoodLog, error) {
	panic("unimplemented")
}

// GetUserWeightLogById implements [models.UserRepository].
func (m *MockUserRepo) GetUserWeightLogById(ctx context.Context, tx *sql.Tx, userId string, logId string) (*models.WeightLog, error) {
	panic("unimplemented")
}

// GetUserWeightLogs implements [models.UserRepository].
func (m *MockUserRepo) GetUserWeightLogs(ctx context.Context, tx *sql.Tx, userId string) ([]*models.WeightLog, error) {
	panic("unimplemented")
}

// UpsertUserExerciseLog implements [models.UserRepository].
func (m *MockUserRepo) UpsertUserExerciseLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.ExerciseLog) error {
	panic("unimplemented")
}

// UpsertUserExerciseSets implements [models.UserRepository].
func (m *MockUserRepo) UpsertUserExerciseSets(ctx context.Context, tx *sql.Tx, payload []*models.ExerciseSet) error {
	panic("unimplemented")
}

// UpsertUserFoodLog implements [models.UserRepository].
func (m *MockUserRepo) UpsertUserFoodLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.FoodLog) error {
	panic("unimplemented")
}

// UpsertUserFoodLogEntry implements [models.UserRepository].
func (m *MockUserRepo) UpsertUserFoodLogEntry(ctx context.Context, tx *sql.Tx, payload []*models.FoodLogEntry) error {
	panic("unimplemented")
}

// UpsertUserWeightLog implements [models.UserRepository].
func (m *MockUserRepo) UpsertUserWeightLog(ctx context.Context, tx *sql.Tx, userId string, payload *models.WeightLog) error {
	panic("unimplemented")
}

type MockAuthProvider struct {
	name    string
	payload *models.AuthPayload
	err     error
}

func (m *MockAuthProvider) GetPlatform() string     { return m.name }
func (m *MockAuthProvider) GetProviderName() string { return m.name }
func (m *MockAuthProvider) HandleCodeExchangeWithVerifier(ctx context.Context, code, verifier string) (*models.AuthPayload, error) {
	return m.payload, m.err
}
func (m *MockUserRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, p *models.AuthPayload) (*models.User, error) {
	return m.mockUser, m.mockErr
}
func (m *MockUserRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return fn(nil)
}

func (m *MockUserRepo) SaveRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string, expiresAt time.Time) error {
	return nil
}
func (m *MockUserRepo) DeleteRefreshToken(ctx context.Context, tx *sql.Tx, refresh string, userId string) error {
	return nil
}
func (m *MockUserRepo) FindRefreshToken(ctx context.Context, tx *sql.Tx, refresh string) (*models.User, error) {
	return nil, nil
}

func (us *MockUserRepo) UpdateUserSettings(ctx context.Context, tx *sql.Tx, userId string, settings *models.UserSettings) error {
	panic("unimplemented")

}

func (us *MockUserRepo) GetUserSettings(ctx context.Context, tx *sql.Tx, userId string) (*models.UserSettings, error) {
	panic("unimplemented")

}
func TestSignIn_FullFlow(t *testing.T) {
	// 1. Setup mocks
	mockUser := &models.User{ID: "1", Email: "kalen@laurier.ca", CreatedAt: time.Now()}
	repo := &MockUserRepo{mockUser: mockUser, mockRefresh: "fake_refresh_token"}

	provider := &MockAuthProvider{
		name:    "github",
		payload: &models.AuthPayload{Email: "kalen@laurier.ca", ID: "12345", Provider: "github"},
	}

	// 2. Initialize Service with the mocks
	as := NewAuthService(repo, "test_secret", "test_refresh", provider)

	// 3. Execute the Sign In
	resp, err := as.SignIn(context.Background(), "valid_code", "github", "", nil)

	// 4. Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if err = utils.CheckValidString(resp.AccessToken); err != nil {
		t.Error("Access token should not be empty")
	}
	if resp.User.ID != mockUser.ID {
		t.Errorf("Expected user ID %s, got %s", mockUser.ID, resp.User.ID)
	}
}

func TestSignIn_DatabaseFailure(t *testing.T) {
	// 1. Setup mocks
	repo := &MockUserRepo{mockErr: errors.New("Database Connection Failed")}

	provider := &MockAuthProvider{
		name:    "github",
		payload: &models.AuthPayload{Email: "kalen@laurier.ca", ID: "12345", Provider: "github"},
	}

	// 2. Initialize Service with the mocks
	as := NewAuthService(repo, "test_secret", "test_refresh", provider)

	// 3. Execute the Sign In
	resp, err := as.SignIn(context.Background(), "valid_code", "github", "", nil)

	// 4. Assertions
	if err == nil {
		t.Error("Expected an error from the service when DB failes, but got nil")
	}
	if resp != nil {
		t.Error("Expected response to be nill when error occurs")
	}
}

func TestSignIn_ProviderFailure(t *testing.T) {
	// Dont need DB because we fail before reaching it
	repo := &MockUserRepo{}

	provider := &MockAuthProvider{
		name: "github",
		err:  errors.New("invalid oauth code"),
	}

	as := NewAuthService(repo, "test_secert", "test_refresh", provider)

	resp, err := as.SignIn(context.Background(), "expired_code", "github", "", nil)

	if err == nil || err.Error() != "invalid oauth code" {
		t.Errorf("Expected `invalid oauth code` error, got %v", err)
	}
	if resp != nil {
		t.Error("Expected response to be nil when provider fails, but got a non-nil object")
	}

}
func TestGenerateJWT(t *testing.T) {
	// 1. Setup
	secret := "my-laurier-secret-123"
	// We only need the secret for this test, so we can pass a nil repo

	testUserID := "42"
	testEmail := "kalen@laurier.ca"
	testIsPremium := false

	// 2. Execution
	tokenString, err := auth.GenerateAccessToken(testUserID, testEmail, testIsPremium, secret)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// 3. Validation: Decode the token to check the claims
	var claims models.CustomClaimsAccess
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		t.Fatalf("Token is invalid: %v", err)
	}

	// 4. Assertions
	if claims.UserID != testUserID {
		t.Errorf("Expected userID %s, got %s", testUserID, claims.UserID)
	}
	if claims.Email != testEmail {
		t.Errorf("Expected email %s, got %s", testEmail, claims.Email)
	}
	if claims.IsPremium != testIsPremium {
		t.Errorf("Expected email %t, got %t", testIsPremium, claims.IsPremium)
	}
	if time.Until(claims.ExpiresAt.Time) > 15*time.Minute {
		t.Error("Expiration time is too far in the future")
	}

}
