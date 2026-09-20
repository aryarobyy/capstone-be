package responsehandler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type sampleRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gt=0"`
}

func TestFormatValidationError_Validator(t *testing.T) {
	validate := validator.New()
	req := sampleRequest{}
	err := validate.Struct(req)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	msg, details := FormatValidationError(err)
	if msg == "" {
		t.Errorf("expected non-empty summary message")
	}

	detailSlice, ok := details.([]string)
	if !ok {
		t.Fatalf("expected []string details, got %T", details)
	}

	if len(detailSlice) == 0 {
		t.Fatalf("expected details to have items")
	}

	// Verify English phrasing without Go internal jargon
	for _, item := range detailSlice {
		if strings.Contains(item, "Key:") || strings.Contains(item, "failed on the") {
			t.Errorf("error detail contains internal Go jargon: %s", item)
		}
		if !strings.HasPrefix(item, "The '") {
			t.Errorf("expected detail to start with \"The '\", got: %s", item)
		}
	}
}

func TestFormatValidationError_EOF(t *testing.T) {
	msg, details := FormatValidationError(io.EOF)
	if !strings.Contains(msg, "empty") {
		t.Errorf("expected message to mention empty, got: %s", msg)
	}
	if details == nil {
		t.Errorf("expected non-nil details")
	}
}

func TestFormatValidationError_UnmarshalTypeError(t *testing.T) {
	err := &json.UnmarshalTypeError{
		Value: "string",
		Type:  reflect.TypeOf(123),
		Field: "age",
	}
	msg, details := FormatValidationError(err)
	if msg == "" {
		t.Errorf("expected non-empty message")
	}
	detailStr, ok := details.(string)
	if !ok {
		detailSlice, okSlice := details.([]string)
		if okSlice && len(detailSlice) > 0 {
			detailStr = detailSlice[0]
		}
	}
	if !strings.Contains(detailStr, "The 'age' field must be of type int") {
		t.Errorf("unexpected detail message: %s", detailStr)
	}
}

func TestFormatValidationError_RawValidatorString(t *testing.T) {
	raw := "Key: 'CreateSensorRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag"
	msg, details := FormatValidationError(errors.New(raw))
	if msg == "" {
		t.Errorf("expected non-empty message")
	}
	detailStr, ok := details.(string)
	if !ok {
		detailSlice, okSlice := details.([]string)
		if okSlice && len(detailSlice) > 0 {
			detailStr = detailSlice[0]
		}
	}
	if !strings.Contains(detailStr, "The 'name' field is required.") {
		t.Errorf("unexpected detail: %v", detailStr)
	}
}

func TestToErrorHandler_SanitizesDatabaseErrorsOn500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbErr := errors.New("pq: relation \"users\" does not exist at character 15")
	ToErrorHandler(c, http.StatusInternalServerError, "Failed to register user", dbErr)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success to be false")
	}
	if strings.Contains(w.Body.String(), "pq:") || strings.Contains(w.Body.String(), "relation") {
		t.Errorf("database error was leaked in response body: %s", w.Body.String())
	}
	if resp.Errors != nil {
		t.Errorf("expected Errors to be nil for internal 500, got: %v", resp.Errors)
	}
}

func TestToErrorHandler_SanitizesBadRequestWithValidatorError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	raw := "Key: 'CreateSensorRequest.Code' Error:Field validation for 'Code' failed on the 'required' tag"
	ToErrorHandler(c, http.StatusBadRequest, "Invalid request body", raw)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if strings.Contains(w.Body.String(), "Key:") || strings.Contains(w.Body.String(), "failed on the") {
		t.Errorf("validator jargon leaked in response body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "The 'code' field is required.") {
		t.Errorf("expected clean English message in response: %s", w.Body.String())
	}
}
