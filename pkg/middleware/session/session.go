package session

import (
	"context"
	"fmt"
	"github.com/maczh/mgin/v2/pkg/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-session/session"
)

type (
	// ErrorHandleFunc error handling function
	ErrorHandleFunc func(*gin.Context, error)
	// Config defines the config for Session middleware
	Config struct {
		// error handling when starting the session
		ErrorHandleFunc ErrorHandleFunc
		// keys stored in the context
		StoreKey string
		// keys stored in the context
		ManageKey string
		// defines a function to skip middleware.Returning true skips processing
		// the middleware.
		Skipper func(*gin.Context) bool
	}
)

var (
	storeKey  = "github.com/maczh/mgin/session/store-key"
	manageKey = "github.com/maczh/mgin/session/manage-key"

	// DefaultConfig is the default Recover middleware config.
	DefaultConfig = Config{
		ErrorHandleFunc: func(ctx *gin.Context, err error) {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, models.Error(500, err.Error()))
		},
		StoreKey:  "github.com/go-session/gin-session/store",
		ManageKey: "github.com/go-session/gin-session/manage",
		Skipper: func(_ *gin.Context) bool {
			return false
		},
	}
)

// New create a session middleware
func New(opt ...session.Option) gin.HandlerFunc {
	return NewWithConfig(DefaultConfig, opt...)
}

// NewWithConfig create a session middleware
func NewWithConfig(config Config, opt ...session.Option) gin.HandlerFunc {
	if config.ErrorHandleFunc == nil {
		config.ErrorHandleFunc = DefaultConfig.ErrorHandleFunc
	}

	localManageKey := config.ManageKey
	if localManageKey == "" {
		localManageKey = DefaultConfig.ManageKey
	}

	localStoreKey := config.StoreKey
	if localStoreKey == "" {
		localStoreKey = DefaultConfig.StoreKey
	}

	manage := session.NewManager(opt...)
	return func(ctx *gin.Context) {
		if config.Skipper != nil && config.Skipper(ctx) {
			ctx.Next()
			return
		}

		ctx.Set(localManageKey, manage)
		ctx.Set(manageKey, localManageKey)
		store, err := manage.Start(context.Background(), ctx.Writer, ctx.Request)
		if err != nil {
			config.ErrorHandleFunc(ctx, err)
			return
		}
		ctx.Set(localStoreKey, store)
		ctx.Set(storeKey, localStoreKey)
		ctx.Next()
	}
}

// FromContext Get session storage from context
func FromContext(ctx *gin.Context) session.Store {
	key := DefaultConfig.StoreKey
	if configured, ok := ctx.Get(storeKey); ok {
		if s, ok := configured.(string); ok && s != "" { key = s }
	}
	v, ok := ctx.Get(key)
	if ok {
		if store, valid := v.(session.Store); valid { return store }
	}
	return nil
}

// Destroy a session
func Destroy(ctx *gin.Context) error {
	key := DefaultConfig.ManageKey
	if configured, ok := ctx.Get(manageKey); ok { if s, valid := configured.(string); valid && s != "" { key = s } }
	v, ok := ctx.Get(key)
	if !ok {
		return fmt.Errorf("invalid session manager")
	}
	manager, valid := v.(*session.Manager)
	if !valid || manager == nil { return fmt.Errorf("invalid session manager") }
	return manager.Destroy(nil, ctx.Writer, ctx.Request)
}

// Refresh a session and return to session storage
func Refresh(ctx *gin.Context) (session.Store, error) {
	key := DefaultConfig.ManageKey
	if configured, ok := ctx.Get(manageKey); ok { if s, valid := configured.(string); valid && s != "" { key = s } }
	v, ok := ctx.Get(key)
	if !ok {
		return nil, fmt.Errorf("invalid session manager")
	}
	manager, valid := v.(*session.Manager)
	if !valid || manager == nil { return nil, fmt.Errorf("invalid session manager") }
	return manager.Refresh(nil, ctx.Writer, ctx.Request)
}
