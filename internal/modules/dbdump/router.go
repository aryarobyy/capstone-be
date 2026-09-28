package dbdump

import (
	"capstone-be/config"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, cfg *config.Config) {
	handler := NewHandler(cfg)
	{
		router.POST("/dump", handler.Dump)
	}
}
