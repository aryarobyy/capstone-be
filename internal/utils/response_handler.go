package responsehandler

import (
	"github.com/gin-gonic/gin"
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

func ToSuccessHandler[T any](c *gin.Context, statusCode int, message string, model T) {
	c.JSON(statusCode, SuccessResponse[T]{
		Success: true,
		Message: message,
		Data:    model,
	})
}

func ToErrorHandler(c *gin.Context, statusCode int, message string, errs any) {
	var formattedErrors interface{} = errs
	if err, ok := errs.(error); ok {
		formattedErrors = err.Error()
	}

	c.JSON(statusCode, ErrorResponse{
		Success: false,
		Message: message,
		Errors:  formattedErrors,
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
