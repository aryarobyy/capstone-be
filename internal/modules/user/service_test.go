package user

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockUserRepository struct {
	capturedLimit int
	capturedIndex int
	users         []User
	total         int
	err           error
	detailUser    *User
	detailErr     error
	updateErr     error
	deleteErr     error
	capturedID    int64
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return nil, nil
}

func (m *mockUserRepository) Detail(ctx context.Context, req UserDetailRequest) (*User, error) {
	m.capturedID = req.ID
	return m.detailUser, m.detailErr
}

func (m *mockUserRepository) List(ctx context.Context, limit, index int) ([]User, int, error) {
	m.capturedLimit = limit
	m.capturedIndex = index
	return m.users, m.total, m.err
}

func (m *mockUserRepository) Update(ctx context.Context, req UpdateUserRequest) error {
	m.capturedID = req.ID
	return m.updateErr
}

func (m *mockUserRepository) Delete(ctx context.Context, req DeleteUserRequest) error {
	m.capturedID = req.ID
	return m.deleteErr
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

func TestUserService_List_ClampingAndError(t *testing.T) {
	repo := &mockUserRepository{}
	svc := NewUserService(repo)
	res, err := svc.List(context.Background(), ListUserRequest{Limit: 1000, Index: -1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.capturedLimit != 100 || repo.capturedIndex != 0 || res.Limit != 100 || res.Index != 0 || res.Data == nil {
		t.Fatalf("repo=%+v res=%+v", repo, res)
	}

	boom := errors.New("boom")
	if _, err := NewUserService(&mockUserRepository{err: boom}).List(context.Background(), ListUserRequest{}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

func TestUserService_Detail(t *testing.T) {
	now := time.Now()
	repo := &mockUserRepository{detailUser: &User{ID: 3, Name: "A", Email: "a@x.io", Msisdn: "1", Password: "h", CreatedAt: now, UpdatedAt: now}}
	res, err := NewUserService(repo).Detail(context.Background(), UserDetailRequest{ID: 3})
	if err != nil {
		t.Fatal(err)
	}
	want := UserResponse{ID: 3, Name: "A", Email: "a@x.io", Msisdn: "1", CreatedAt: now, UpdatedAt: now}
	if *res != want || repo.capturedID != 3 {
		t.Fatalf("got %+v", res)
	}

	_, err = NewUserService(&mockUserRepository{detailErr: ErrUserNotFound}).Detail(context.Background(), UserDetailRequest{ID: 1})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestUserService_Update(t *testing.T) {
	ctx := context.Background()
	repo := &mockUserRepository{detailUser: &User{ID: 2, Name: "New"}}
	res, err := NewUserService(repo).Update(ctx, UpdateUserRequest{ID: 2, Name: "New"})
	if err != nil || res.Name != "New" || repo.capturedID != 2 {
		t.Fatalf("got %+v %v", res, err)
	}

	if _, err := NewUserService(&mockUserRepository{updateErr: ErrEmailAlreadyExists}).Update(ctx, UpdateUserRequest{ID: 2}); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("update err: %v", err)
	}
	if _, err := NewUserService(&mockUserRepository{detailErr: ErrUserNotFound}).Update(ctx, UpdateUserRequest{ID: 2}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("detail err: %v", err)
	}
}

func TestUserService_Delete(t *testing.T) {
	repo := &mockUserRepository{}
	if err := NewUserService(repo).Delete(context.Background(), DeleteUserRequest{ID: 4}); err != nil || repo.capturedID != 4 {
		t.Fatalf("got %v id=%d", err, repo.capturedID)
	}
	if err := NewUserService(&mockUserRepository{deleteErr: ErrUserNotFound}).Delete(context.Background(), DeleteUserRequest{ID: 4}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestToUserResponse_Nil(t *testing.T) {
	if toUserResponse(nil) != nil {
		t.Fatal("expected nil")
	}
}
