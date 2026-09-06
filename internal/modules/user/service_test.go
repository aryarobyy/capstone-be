package user

import (
	"context"
	"testing"
	"time"
)

type mockUserRepository struct {
	capturedLimit int
	capturedIndex int
	users         []User
	total         int
	err           error
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return nil, nil
}

func (m *mockUserRepository) Detail(ctx context.Context, req UserDetailRequest) (*User, error) {
	return nil, nil
}

func (m *mockUserRepository) List(ctx context.Context, limit, index int) ([]User, int, error) {
	m.capturedLimit = limit
	m.capturedIndex = index
	return m.users, m.total, m.err
}

func (m *mockUserRepository) Update(ctx context.Context, req UpdateUserRequest) error {
	return nil
}

func (m *mockUserRepository) Delete(ctx context.Context, req DeleteUserRequest) error {
	return nil
}

func TestUserService_List_Pagination(t *testing.T) {
	tests := []struct {
		name          string
		req           ListUserRequest
		mockTotal     int
		mockUsers     []User
		expectedLimit int
		expectedIndex int
	}{
		{
			name:          "Default pagination when request is empty",
			req:           ListUserRequest{},
			mockTotal:     12,
			mockUsers:     []User{{ID: 1, Name: "John Doe", Email: "john@example.com"}},
			expectedLimit: 10,
			expectedIndex: 0,
		},
		{
			name: "Pagination with explicit Limit and Index",
			req: ListUserRequest{
				Limit: 5,
				Index: 10,
			},
			mockTotal:     12,
			mockUsers:     []User{{ID: 6, Name: "Jane Doe", Email: "jane@example.com"}},
			expectedLimit: 5,
			expectedIndex: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockUserRepository{
				total: tt.mockTotal,
				users: tt.mockUsers,
			}
			svc := NewUserService(mockRepo)

			res, err := svc.List(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mockRepo.capturedLimit != tt.expectedLimit {
				t.Errorf("repo limit = %d, expected %d", mockRepo.capturedLimit, tt.expectedLimit)
			}
			if mockRepo.capturedIndex != tt.expectedIndex {
				t.Errorf("repo index = %d, expected %d", mockRepo.capturedIndex, tt.expectedIndex)
			}
			if res.Limit != tt.expectedLimit {
				t.Errorf("response Limit = %d, expected %d", res.Limit, tt.expectedLimit)
			}
			if res.Index != tt.expectedIndex {
				t.Errorf("response Index = %d, expected %d", res.Index, tt.expectedIndex)
			}
			if res.Total != tt.mockTotal {
				t.Errorf("response Total = %d, expected %d", res.Total, tt.mockTotal)
			}
		})
	}
}

func TestUserService_List_DataMapping(t *testing.T) {
	now := time.Now()
	mockRepo := &mockUserRepository{
		total: 1,
		users: []User{
			{
				ID:        1,
				Name:      "Alice",
				Email:     "alice@example.com",
				Msisdn:    "08123456789",
				Password:  "secret-hashed-password",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	svc := NewUserService(mockRepo)

	res, err := svc.List(context.Background(), ListUserRequest{Limit: 10, Index: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Data))
	}
	item := res.Data[0]
	if item.ID != 1 || item.Name != "Alice" || item.Email != "alice@example.com" {
		t.Errorf("data mapping mismatch: %+v", item)
	}
}
