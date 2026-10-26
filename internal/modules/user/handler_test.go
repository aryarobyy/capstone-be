package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockUserService struct{ err error }

func (m *mockUserService) Detail(ctx context.Context, req UserDetailRequest) (*UserResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &UserResponse{ID: req.ID, Name: "A"}, nil
}
func (m *mockUserService) List(ctx context.Context, req ListUserRequest) (*ListUserResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ListUserResponse{Data: []ListUserData{{ID: 1}}, Total: 1, Limit: 10}, nil
}
func (m *mockUserService) Update(ctx context.Context, req UpdateUserRequest) (*UserResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &UserResponse{ID: req.ID, Name: req.Name}, nil
}
func (m *mockUserService) Delete(ctx context.Context, req DeleteUserRequest) error { return m.err }

func newTestRouter(svc UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUserHandler(svc)
	g := r.Group("/user")
	g.POST("/list", h.List)
	g.POST("/detail", h.Detail)
	g.POST("/update", h.Update)
	g.POST("/delete", h.Delete)
	return r
}

func post(r http.Handler, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestUserHandler_StatusCodes(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name, path, body string
		err              error
		want             int
	}{
		{"detail ok", "/user/detail", `{"id":1}`, nil, http.StatusOK},
		{"detail missing id", "/user/detail", `{}`, nil, http.StatusBadRequest},
		{"detail not found", "/user/detail", `{"id":1}`, ErrUserNotFound, http.StatusNotFound},
		{"detail internal", "/user/detail", `{"id":1}`, boom, http.StatusInternalServerError},

		{"list ok", "/user/list", `{"limit":5}`, nil, http.StatusOK},
		{"list bad json", "/user/list", `{`, nil, http.StatusBadRequest},
		{"list internal", "/user/list", `{}`, boom, http.StatusInternalServerError},

		{"update ok", "/user/update", `{"id":1,"name":"Bob"}`, nil, http.StatusOK},
		{"update invalid email", "/user/update", `{"id":1,"email":"nope"}`, nil, http.StatusBadRequest},
		{"update short name", "/user/update", `{"id":1,"name":"a"}`, nil, http.StatusBadRequest},
		{"update not found", "/user/update", `{"id":1}`, ErrUserNotFound, http.StatusNotFound},
		{"update email conflict", "/user/update", `{"id":1,"email":"a@x.io"}`, ErrEmailAlreadyExists, http.StatusConflict},
		{"update internal", "/user/update", `{"id":1}`, boom, http.StatusInternalServerError},

		{"delete ok", "/user/delete", `{"id":1}`, nil, http.StatusOK},
		{"delete empty body", "/user/delete", ``, nil, http.StatusBadRequest},
		{"delete not found", "/user/delete", `{"id":1}`, ErrUserNotFound, http.StatusNotFound},
		{"delete internal", "/user/delete", `{"id":1}`, boom, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := post(newTestRouter(&mockUserService{err: tt.err}), tt.path, tt.body)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", w.Code, tt.want, w.Body)
			}
			var resp struct {
				Success bool `json:"success"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("bad json: %v", err)
			}
			if resp.Success != (tt.want < 300) {
				t.Fatalf("success = %v for status %d", resp.Success, w.Code)
			}
		})
	}
}
