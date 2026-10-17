package i18n

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/vo"
)

// RouterGroup mounts dictionary CRUD on a caller-supplied group.
// GET /list lists entries; GET /get and POST /delete use messageId/language query parameters;
// POST /save and POST /update accept {"messageId":"hello","language":"en","text":"Hello"}.
func RouterGroup(group *gin.RouterGroup) {
	group.GET("/list", listHandler)
	group.GET("/get", getHandler)
	group.POST("/save", createHandler)
	group.POST("/update", updateHandler)
	group.POST("/delete", deleteHandler)
}

func apiError(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	message := "dictionary storage error"
	switch {
	case errors.Is(err, ErrInvalidEntry):
		code = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, ErrConflict):
		code = http.StatusConflict
		message = err.Error()
	case errors.Is(err, ErrNotInitialized):
		code = http.StatusServiceUnavailable
		message = err.Error()
	default:
		logs.Error("i18n dictionary operation failed: {}", err)
	}
	// Keep the framework's HTTP 200 envelope convention; Code carries the error.
	c.JSON(http.StatusOK, vo.Error(code, message))
}

func apiStore(c *gin.Context) (*Store, bool) {
	s, err := GetStore()
	if err != nil {
		apiError(c, err)
		return nil, false
	}
	return s, true
}

func getHandler(c *gin.Context) {
	s, ok := apiStore(c)
	if !ok {
		return
	}
	entry, err := s.Get(c.Request.Context(), c.Query("messageId"), c.Query("language"))
	if err != nil {
		apiError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.Success(entry))
}

func createHandler(c *gin.Context) {
	s, ok := apiStore(c)
	if !ok {
		return
	}
	var entry Entry
	if err := c.ShouldBindJSON(&entry); err != nil {
		apiError(c, ErrInvalidEntry)
		return
	}
	if err := s.Create(c.Request.Context(), &entry); err != nil {
		apiError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.Success(entry))
}

func updateHandler(c *gin.Context) {
	s, ok := apiStore(c)
	if !ok {
		return
	}
	var entry Entry
	if err := c.ShouldBindJSON(&entry); err != nil {
		apiError(c, ErrInvalidEntry)
		return
	}
	if err := s.Update(c.Request.Context(), &entry); err != nil {
		apiError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.Success(entry))
}

func deleteHandler(c *gin.Context) {
	s, ok := apiStore(c)
	if !ok {
		return
	}
	if err := s.Delete(c.Request.Context(), c.Query("messageId"), c.Query("language")); err != nil {
		apiError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.Success(nil))
}

func listHandler(c *gin.Context) {
	s, ok := apiStore(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 || page > 1000000 {
		apiError(c, ErrInvalidEntry)
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || size <= 0 || size > 100 {
		apiError(c, ErrInvalidEntry)
		return
	}
	entries, total, err := s.List(c.Request.Context(), ListOptions{
		MessageID: c.Query("messageId"), Language: c.Query("language"), Page: page, PageSize: size,
	})
	if err != nil {
		apiError(c, err)
		return
	}
	c.JSON(http.StatusOK, vo.ResponsePage[Entry]{Code: vo.SUCCESS, Msg: "success",
		SrvTime: time.Now(),
		Data:    &vo.PageData[Entry]{List: entries, Total: total, Page: page, PageSize: size},
	})
}
