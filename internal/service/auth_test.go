package service

import (
	"context"
	"database/sql"
	"errors"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/store"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
)

// MockAuthRepo fakes the auth store. Each method has its own error so a test can make one step fail
// while the others succeed, and it records what it was called with.
type MockAuthRepo struct {
	mockUser  *models.User
	upsertErr error

	mockFirebaseId string // returned by LookupUserIdToFirebase
	lookupErr      error
	deleteErr      error

	upsertPayload *models.AuthPayload // payload SignIn passed to UpsertUserWithAuth
	deletedUserId string              // user id DeleteUser was called with, "" if never called
	txErr         error               // error the transaction function returned (non-nil means a real tx would roll back)
}

// DeleteUser implements [models.AuthRepository].
func (m *MockAuthRepo) DeleteUser(ctx context.Context, tx *sql.Tx, userID string) error {
	m.deletedUserId = userID
	return m.deleteErr
}

// LookupFirebaseIdToUserId implements [models.AuthRepository].
func (m *MockAuthRepo) LookupFirebaseIdToUserId(ctx context.Context, firebaseID string) (string, error) {
	return m.mockUser.ID, nil
}

// LookupUserIdToFirebase implements [models.AuthRepository].
func (m *MockAuthRepo) LookupUserIdToFirebase(ctx context.Context, userid string) (string, error) {
	return m.mockFirebaseId, m.lookupErr
}

func (m *MockAuthRepo) UpsertUserWithAuth(ctx context.Context, tx *sql.Tx, p *models.AuthPayload) (*models.User, error) {
	m.upsertPayload = p
	return m.mockUser, m.upsertErr
}
func (m *MockAuthRepo) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	m.txErr = fn(nil)
	return m.txErr
}

// MockAuthClient fakes the Firebase auth client. Verify and delete have separate errors.
type MockAuthClient struct {
	payload   *mockFirebaseUser
	verifyErr error
	deleteErr error

	deletedUid string // uid DeleteUser was called with, "" if never called
}
type mockFirebaseUser struct {
	email    *string // nil means the token has no email claim
	verified bool
}

// DeleteUser implements [models.AuthClient].
func (m *MockAuthClient) DeleteUser(ctx context.Context, uid string) error {
	m.deletedUid = uid
	return m.deleteErr
}

// VerifyIDToken implements [models.AuthClient].
func (m *MockAuthClient) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if m.verifyErr != nil {
		return nil, m.verifyErr
	}
	claims := map[string]interface{}{
		"email_verified": m.payload.verified,
	}
	if m.payload.email != nil {
		claims["email"] = *m.payload.email
	}
	return &auth.Token{UID: "234", Claims: claims}, nil
}

func TestSignIn_FullFlow(t *testing.T) {
	// 1. Setup mocks
	email := "kalen@laurier.ca"
	mockUser := &models.User{ID: "1", Email: &email, CreatedAt: time.Now()}
	repo := &MockAuthRepo{mockUser: mockUser, mockFirebaseId: "firebase"}

	client := &MockAuthClient{payload: &mockFirebaseUser{email: &email, verified: true}}

	// 2. Initialize Service with the mocks
	as := NewAuthService(repo, client)

	// 3. Execute the Sign In
	resp, err := as.SignIn(context.Background(), "1234")

	// 4. Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.UserId != mockUser.ID {
		t.Errorf("Expected user ID %s, got %s", mockUser.ID, resp.UserId)
	}
}

func TestSignIn_DatabaseFailure(t *testing.T) {
	// 1. Setup mocks
	repo := &MockAuthRepo{upsertErr: errors.New("Database Connection Failed")}
	email := "kalen@laurier.ca"
	client := &MockAuthClient{payload: &mockFirebaseUser{email: &email, verified: true}}
	// 2. Initialize Service with the mocks
	as := NewAuthService(repo, client)

	// 3. Execute the Sign In
	resp, err := as.SignIn(context.Background(), "123")

	// 4. Assertions
	if err == nil {
		t.Error("Expected an error from the service when DB fails, but got nil")
	}
	if resp != nil {
		t.Error("Expected response to be nil when error occurs")
	}
}

func TestSignIn_InvalidToken(t *testing.T) {
	repo := &MockAuthRepo{}
	client := &MockAuthClient{verifyErr: errors.New("token expired")}
	as := NewAuthService(repo, client)

	resp, err := as.SignIn(context.Background(), "bad-token")

	if !apperrors.Is(err, apperrors.InvalidToken) {
		t.Fatalf("Expected %s, got %v", apperrors.InvalidToken.Code, err)
	}
	if resp != nil {
		t.Error("Expected response to be nil when the token is invalid")
	}
	if repo.upsertPayload != nil {
		t.Error("User was upserted even though the token was invalid")
	}
}

func TestSignIn_Email(t *testing.T) {
	email := "kalen@laurier.ca"
	tests := []struct {
		name          string
		firebaseUser  *mockFirebaseUser
		expectedEmail *string
	}{
		{"Verified email is stored", &mockFirebaseUser{email: &email, verified: true}, &email},
		{"Unverified email is dropped", &mockFirebaseUser{email: &email, verified: false}, nil},
		{"Missing email stays nil", &mockFirebaseUser{email: nil, verified: false}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockAuthRepo{mockUser: &models.User{ID: "1"}}
			client := &MockAuthClient{payload: tc.firebaseUser}
			as := NewAuthService(repo, client)

			if _, err := as.SignIn(context.Background(), "1234"); err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if repo.upsertPayload == nil {
				t.Fatal("User was never upserted")
			}
			if repo.upsertPayload.ID != "234" {
				t.Errorf("Expected firebase uid 234 in payload, got %s", repo.upsertPayload.ID)
			}

			got := repo.upsertPayload.Email
			switch {
			case tc.expectedEmail == nil && got != nil:
				t.Errorf("Expected no email, got %s", *got)
			case tc.expectedEmail != nil && got == nil:
				t.Errorf("Expected email %s, got nil", *tc.expectedEmail)
			case tc.expectedEmail != nil && *got != *tc.expectedEmail:
				t.Errorf("Expected email %s, got %s", *tc.expectedEmail, *got)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	dbDown := errors.New("database connection failed")
	firebaseDown := errors.New("firebase unavailable")

	tests := []struct {
		name   string
		repo   *MockAuthRepo
		client *MockAuthClient

		expectErr      bool
		expectKind     *apperrors.ErrorKind // checked only when set
		expectDbDelete bool
		expectFbDelete bool
		expectRollback bool // the transaction function returned an error, so a real database would roll back
	}{
		{
			name:           "Success deletes database row then Firebase user",
			repo:           &MockAuthRepo{mockFirebaseId: "fb-1"},
			client:         &MockAuthClient{},
			expectDbDelete: true,
			expectFbDelete: true,
		},
		{
			name:           "Unknown user returns session over and deletes nothing",
			repo:           &MockAuthRepo{lookupErr: store.ErrNotFound},
			client:         &MockAuthClient{},
			expectErr:      true,
			expectKind:     &apperrors.SessionOver,
			expectRollback: true,
		},
		{
			name:           "Lookup failure deletes nothing",
			repo:           &MockAuthRepo{lookupErr: dbDown},
			client:         &MockAuthClient{},
			expectErr:      true,
			expectRollback: true,
		},
		{
			name:           "Database delete failure never touches Firebase",
			repo:           &MockAuthRepo{mockFirebaseId: "fb-1", deleteErr: dbDown},
			client:         &MockAuthClient{},
			expectErr:      true,
			expectDbDelete: true,
			expectRollback: true,
		},
		{
			name:           "Row already gone is treated as success and never touches Firebase",
			repo:           &MockAuthRepo{mockFirebaseId: "fb-1", deleteErr: store.ErrNoRowsAffected},
			client:         &MockAuthClient{},
			expectDbDelete: true,
			expectRollback: true,
		},
		{
			name:           "Firebase failure rolls back the database delete",
			repo:           &MockAuthRepo{mockFirebaseId: "fb-1"},
			client:         &MockAuthClient{deleteErr: firebaseDown},
			expectErr:      true,
			expectDbDelete: true,
			expectFbDelete: true,
			expectRollback: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			as := NewAuthService(tc.repo, tc.client)

			err := as.DeleteUser(context.Background(), "user-1")

			if tc.expectErr && err == nil {
				t.Fatal("Expected an error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if tc.expectKind != nil && !apperrors.Is(err, *tc.expectKind) {
				t.Errorf("Expected %s, got %v", tc.expectKind.Code, err)
			}

			dbDeleted := tc.repo.deletedUserId != ""
			if dbDeleted != tc.expectDbDelete {
				t.Errorf("Database delete called: %v, expected %v", dbDeleted, tc.expectDbDelete)
			}
			if dbDeleted && tc.repo.deletedUserId != "user-1" {
				t.Errorf("Database delete got user id %s, expected user-1", tc.repo.deletedUserId)
			}

			fbDeleted := tc.client.deletedUid != ""
			if fbDeleted != tc.expectFbDelete {
				t.Errorf("Firebase delete called: %v, expected %v", fbDeleted, tc.expectFbDelete)
			}
			if fbDeleted && tc.client.deletedUid != tc.repo.mockFirebaseId {
				t.Errorf("Firebase delete got uid %s, expected %s", tc.client.deletedUid, tc.repo.mockFirebaseId)
			}

			rolledBack := tc.repo.txErr != nil
			if rolledBack != tc.expectRollback {
				t.Errorf("Transaction rolled back: %v, expected %v", rolledBack, tc.expectRollback)
			}
		})
	}
}
