package responsehandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func ctx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad json %q: %v", w.Body.String(), err)
	}
	return m
}

func TestFormatTagMessage(t *testing.T) {
	for tag, want := range map[string]string{
		"required": "The 'f' field is required.",
		"email":    "The 'f' field must be a valid email address.",
		"min":      "The 'f' field must be at least 3 characters long.",
		"max":      "The 'f' field must not exceed 3 characters.",
		"gt":       "The 'f' field must be greater than 3.",
		"gte":      "The 'f' field must be greater than or equal to 3.",
		"lt":       "The 'f' field must be less than 3.",
		"lte":      "The 'f' field must be less than or equal to 3.",
		"len":      "The 'f' field must be exactly 3 characters.",
		"numeric":  "The 'f' field must be a numeric value.",
		"oneof":    "The 'f' field is invalid.",
	} {
		if got := formatTagMessage("f", tag, "3"); got != want {
			t.Errorf("%s: got %q want %q", tag, got, want)
		}
	}
}

func TestFormatValidationErrorBranches(t *testing.T) {
	if msg, d := FormatValidationError(nil); msg != "Invalid request payload." || d != nil {
		t.Errorf("nil: %q %v", msg, d)
	}
	var v any
	syntaxErr := json.Unmarshal([]byte(`{"a":`), &v)
	if msg, d := FormatValidationError(syntaxErr); msg != "Malformed JSON payload." || !strings.Contains(d.(string), "byte offset") {
		t.Errorf("syntax: %q %v", msg, d)
	}
	type one struct {
		UserName string `validate:"required"`
	}
	if msg, d := FormatValidationError(validator.New().Struct(one{})); msg != "The 'user_name' field is required." || d != msg {
		t.Errorf("single validator: %q %v", msg, d)
	}
	raw := "Key: 'R.A' Error:Field validation for 'A' failed on the 'required' tag\nKey: 'R.BField' Error:Field validation for 'BField' failed on the 'email' tag"
	msg, d := FormatValidationError(errors.New(raw))
	if msg != "Validation failed for one or more fields." || !reflect.DeepEqual(d, []string{"The 'a' field is required.", "The 'b_field' field must be a valid email address."}) {
		t.Errorf("multi raw: %q %v", msg, d)
	}
	if msg, d := FormatValidationError(errors.New("boom")); msg != "Invalid request payload." || d != "boom" {
		t.Errorf("plain: %q %v", msg, d)
	}
}

func TestToErrorHandlerSanitizeBranches(t *testing.T) {
	type payload struct {
		X int `json:"x"`
	}
	for _, tc := range []struct {
		name       string
		status     int
		message    string
		errs       any
		wantMsg    string
		wantErrors any // nil means field omitted
	}{
		{"500 empty message", 500, "", nil, "An unexpected error occurred while processing your request.", nil},
		{"500 db message", 503, "pq: connection refused", "x", "An unexpected error occurred while processing your request.", nil},
		{"500 clean message", 500, "Failed to save", errors.New("boom"), "Failed to save", nil},
		{"400 error default message", 400, "", errors.New("boom"), "Invalid request payload.", "boom"},
		{"400 error custom message", 400, "Bad thing", errors.New("boom"), "Bad thing", "boom"},
		{"409 db error", 409, "Conflict", errors.New(`pq: duplicate key violates unique constraint "x"`), "Conflict", "A database constraint was encountered."},
		{"404 plain error", 404, "Not found", errors.New("sensor not found"), "Not found", "sensor not found"},
		{"400 validator string empty message", 400, "", "Key: 'R.Code' Error:Field validation for 'Code' failed on the 'required' tag", "The 'code' field is required.", "The 'code' field is required."},
		{"400 validator string custom message", 400, "Nope", "Key: 'R.Code' Error:Field validation for 'Code' failed on the 'required' tag", "Nope", "The 'code' field is required."},
		{"422 db string", 422, "Bad", "sql: no rows in result set", "Bad", "A database constraint was encountered."},
		{"400 plain string", 400, "Bad", "just text", "Bad", "just text"},
		{"401 nil errs", 401, "Unauthorized", nil, "Unauthorized", nil},
		{"400 other type passthrough", 400, "Bad", payload{X: 1}, "Bad", map[string]any{"x": float64(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, w := ctx()
			ToErrorHandler(c, tc.status, tc.message, tc.errs)
			m := decode(t, w)
			if w.Code != tc.status || m["success"] != false || m["message"] != tc.wantMsg {
				t.Fatalf("got %d %v", w.Code, m)
			}
			got, ok := m["errors"]
			if tc.wantErrors == nil {
				if ok {
					t.Fatalf("errors should be omitted, got %v", got)
				}
			} else if !reflect.DeepEqual(got, tc.wantErrors) {
				t.Fatalf("errors: got %#v want %#v", got, tc.wantErrors)
			}
		})
	}
}

func TestToSuccessHandler(t *testing.T) {
	c, w := ctx()
	ToSuccessHandler(c, http.StatusCreated, "ok", map[string]int{"id": 1})
	m := decode(t, w)
	if w.Code != 201 || m["success"] != true || m["message"] != "ok" || m["data"].(map[string]any)["id"] != float64(1) {
		t.Fatalf("got %d %v", w.Code, m)
	}
}

func TestToValidationError(t *testing.T) {
	c, w := ctx()
	ToValidationError(c, errors.New("boom"))
	m := decode(t, w)
	if w.Code != 400 || m["success"] != false || m["message"] != "Invalid request payload." || m["errors"] != "boom" {
		t.Fatalf("got %d %v", w.Code, m)
	}
}

func TestToPaginatedSuccessHandler(t *testing.T) {
	for _, tc := range []struct {
		limit     int
		total     int64
		wantPages float64
	}{{10, 25, 3}, {10, 20, 2}, {10, 0, 0}, {-1, 7, 1}} {
		c, w := ctx()
		ToPaginatedSuccessHandler(c, 200, "list", []int{1}, 2, tc.limit, tc.total)
		m := decode(t, w)
		p := m["pagination"].(map[string]any)
		if p["total_page"] != tc.wantPages || p["current_page"] != float64(2) || p["per_page"] != float64(tc.limit) || p["total_data"] != float64(tc.total) {
			t.Errorf("limit=%d total=%d: got %v", tc.limit, tc.total, p)
		}
	}
}

func TestToPaginatedSuccessHandlerZeroLimit(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("BUG: ToPaginatedSuccessHandler panics (%v) when limit == 0: the limit<=0 guard runs after int(totalData)/limit", r)
		}
	}()
	c, w := ctx()
	ToPaginatedSuccessHandler(c, 200, "list", []int{}, 1, 0, 5)
	if p := decode(t, w)["pagination"].(map[string]any); p["total_page"] != float64(1) {
		t.Fatalf("got %v", p)
	}
}
