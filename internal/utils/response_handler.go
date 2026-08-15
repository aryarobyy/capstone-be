package responsehandler

import (
	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
}

type Pagination struct {
	CurrentPage int   `json:"current_page"`
	TotalPage   int   `json:"total_page"`
	PerPage     int   `json:"per_page"`
	TotalData   int64 `json:"total_data"`
}

type PaginatedResponse struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

func JSONSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func JSONError(c *gin.Context, statusCode int, message string, errs interface{}) {
	var formattedErrors interface{} = errs
	if err, ok := errs.(error); ok {
		formattedErrors = err.Error()
	}

	c.JSON(statusCode, Response{
		Success: false,
		Message: message,
		Errors:  formattedErrors,
	})
}

func JSONPaginated(c *gin.Context, statusCode int, message string, data interface{}, page, limit int, totalData int64) {
	totalPage := int(totalData) / limit
	if limit > 0 && int(totalData)%limit != 0 {
		totalPage++
	}
	if limit <= 0 {
		totalPage = 1
	}

	c.JSON(statusCode, PaginatedResponse{
		Success: true,
		Message: message,
		Data:    data,
		Pagination: Pagination{
			CurrentPage: page,
			TotalPage:   totalPage,
			PerPage:     limit,
			TotalData:   totalData,
		},
	})
}
