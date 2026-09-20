package responsehandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type SuccessResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}

type ListResponse[T any] struct {
	List  []T `json:"list"`
	Count int `json:"count"`
	Index int `json:"index"`
	Limit int `json:"limit"`
}

type Pagination struct {
	CurrentPage int   `json:"current_page"`
	TotalPage   int   `json:"total_page"`
	PerPage     int   `json:"per_page"`
	TotalData   int64 `json:"total_data"`
}

type PaginatedResponse[T any] struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message"`
	Data       T          `json:"data"`
	Pagination Pagination `json:"pagination"`
}

var (
	matchFirstCap  = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap    = regexp.MustCompile("([a-z0-9])([A-Z])")
	matchValidator = regexp.MustCompile(`Key:\s*'[^.]*\.([^']+)'\s*Error:Field validation for '[^']+' failed on the '([^']+)' tag`)
)

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func formatTagMessage(field, tag, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("The '%s' field is required.", field)
	case "email":
		return fmt.Sprintf("The '%s' field must be a valid email address.", field)
	case "min":
		return fmt.Sprintf("The '%s' field must be at least %s characters long.", field, param)
	case "max":
		return fmt.Sprintf("The '%s' field must not exceed %s characters.", field, param)
	case "gt":
		return fmt.Sprintf("The '%s' field must be greater than %s.", field, param)
	case "gte":
		return fmt.Sprintf("The '%s' field must be greater than or equal to %s.", field, param)
	case "lt":
		return fmt.Sprintf("The '%s' field must be less than %s.", field, param)
	case "lte":
		return fmt.Sprintf("The '%s' field must be less than or equal to %s.", field, param)
	case "len":
		return fmt.Sprintf("The '%s' field must be exactly %s characters.", field, param)
	case "numeric":
		return fmt.Sprintf("The '%s' field must be a numeric value.", field)
	default:
		return fmt.Sprintf("The '%s' field is invalid.", field)
	}
}

func FormatValidationError(err error) (string, any) {
	if err == nil {
		return "Invalid request payload.", nil
	}

	if errors.Is(err, io.EOF) {
		return "Request body cannot be empty.", "Please provide a valid JSON payload."
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return "Malformed JSON payload.", fmt.Sprintf("Invalid JSON syntax at byte offset %d.", syntaxErr.Offset)
	}

	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		field := toSnakeCase(unmarshalTypeErr.Field)
		return "Invalid field type in request payload.", fmt.Sprintf("The '%s' field must be of type %s.", field, unmarshalTypeErr.Type.String())
	}

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		var details []string
		for _, fe := range validationErrs {
			field := toSnakeCase(fe.Field())
			details = append(details, formatTagMessage(field, fe.Tag(), fe.Param()))
		}
		if len(details) == 1 {
			return details[0], details[0]
		}
		return "Validation failed for one or more fields.", details
	}

	errStr := err.Error()
	matches := matchValidator.FindAllStringSubmatch(errStr, -1)
	if len(matches) > 0 {
		var details []string
		for _, m := range matches {
			if len(m) >= 3 {
				field := toSnakeCase(m[1])
				tag := m[2]
				details = append(details, formatTagMessage(field, tag, ""))
			}
		}
		if len(details) == 1 {
			return details[0], details[0]
		}
		return "Validation failed for one or more fields.", details
	}

	return "Invalid request payload.", errStr
}

func isDatabaseOrInternalError(errStr string) bool {
	lower := strings.ToLower(errStr)
	return strings.Contains(lower, "pq:") ||
		strings.Contains(lower, "sql:") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "relation ") ||
		strings.Contains(lower, "syntax error at or near") ||
		strings.Contains(lower, "violates foreign key constraint") ||
		strings.Contains(lower, "violates unique constraint") ||
		strings.Contains(lower, "pg_") ||
		strings.Contains(lower, "driver: ") ||
		strings.Contains(lower, "deadlock detected") ||
		strings.Contains(lower, "for update")
}

func sanitizeError(statusCode int, message string, errs any) (string, any) {
	if statusCode >= 500 {
		if errs != nil {
			log.Printf("[ERROR] Internal server error (status %d): %v", statusCode, errs)
		}
		cleanMessage := message
		if cleanMessage == "" || isDatabaseOrInternalError(cleanMessage) {
			cleanMessage = "An unexpected error occurred while processing your request."
		}
		return cleanMessage, nil
	}

	if err, ok := errs.(error); ok {
		if statusCode == http.StatusBadRequest {
			cleanMsg, details := FormatValidationError(err)
			if message == "" || message == "Invalid request body" {
				message = cleanMsg
			}
			return message, details
		}
		errStr := err.Error()
		if isDatabaseOrInternalError(errStr) {
			return message, "A database constraint was encountered."
		}
		return message, errStr
	}

	if errStr, ok := errs.(string); ok {
		if statusCode == http.StatusBadRequest && matchValidator.MatchString(errStr) {
			cleanMsg, details := FormatValidationError(errors.New(errStr))
			if message == "" || message == "Invalid request body" {
				message = cleanMsg
			}
			return message, details
		}
		if isDatabaseOrInternalError(errStr) {
			return message, "A database constraint was encountered."
		}
		return message, errStr
	}

	return message, errs
}

func ToSuccessHandler[T any](c *gin.Context, statusCode int, message string, model T) {
	c.JSON(statusCode, SuccessResponse[T]{
		Success: true,
		Message: message,
		Data:    model,
	})
}

func ToErrorHandler(c *gin.Context, statusCode int, message string, errs any) {
	cleanMessage, formattedErrors := sanitizeError(statusCode, message, errs)

	c.JSON(statusCode, ErrorResponse{
		Success: false,
		Message: cleanMessage,
		Errors:  formattedErrors,
	})
}

func ToValidationError(c *gin.Context, err error) {
	msg, details := FormatValidationError(err)
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  details,
	})
}

func ToPaginatedSuccessHandler[T any](c *gin.Context, statusCode int, message string, model T, page, limit int, totalData int64) {
	totalPage := int(totalData) / limit
	if limit > 0 && int(totalData)%limit != 0 {
		totalPage++
	}
	if limit <= 0 {
		totalPage = 1
	}

	c.JSON(statusCode, PaginatedResponse[T]{
		Success: true,
		Message: message,
		Data:    model,
		Pagination: Pagination{
			CurrentPage: page,
			TotalPage:   totalPage,
			PerPage:     limit,
			TotalData:   totalData,
		},
	})
}
