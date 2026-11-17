package middleware

import (
	"context"
	stderrors "errors"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
)

var (
	ErrPermissionDenied = errors.Forbidden("PERMISSION_DENIED", "permission denied")
)

type authzMiddleware struct {
	aclRepo   data.ACLRepository
	driftRepo data.DriftRepository
	logger    *log.Helper
}

// NewAuthorizationMiddleware creates an authorization middleware
func NewAuthorizationMiddleware(aclRepo data.ACLRepository, logger log.Logger, driftRepos ...data.DriftRepository) middleware.Middleware {
	var driftRepo data.DriftRepository
	if len(driftRepos) > 0 {
		driftRepo = driftRepos[0]
	}
	a := &authzMiddleware{
		aclRepo:   aclRepo,
		driftRepo: driftRepo,
		logger:    log.NewHelper(log.With(logger, "module", "middleware/Authorization")),
	}
	return a.handle
}

func (a *authzMiddleware) handle(handler middleware.Handler) middleware.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		// Admin always has access
		if IsAdmin(ctx) {
			return handler(ctx, req)
		}

		// Get user from context
		user, ok := GetUserFromContext(ctx)
		if !ok {
			// No user in context means authentication failed or not required
			return handler(ctx, req)
		}

		// Get the method being called
		serverContext, ok := transport.FromServerContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		operation := serverContext.Operation()

		if strings.Contains(operation, "/AcknowledgeDriftEvent") && a.driftRepo != nil {
			allowed, err := a.authorizeAcknowledgeDriftEvent(ctx, user, req)
			if err != nil {
				a.logger.Errorf("failed to authorize drift event acknowledgement for user %s: %v", user.ID, err)
				return nil, ErrPermissionDenied
			}
			if !allowed {
				a.logger.Warnf("user %s does not have write permission to acknowledge drift event", user.Name)
				return nil, ErrPermissionDenied
			}
			return handler(ctx, req)
		}

		// Determine required permission based on method
		requiredPermission, moduleName := getRequiredPermission(operation, req)
		if requiredPermission == "" {
			// No permission check required
			return handler(ctx, req)
		}

		// Check if user has the required permission
		hasPermission, err := a.aclRepo.CheckPermission(ctx, user.ID, moduleName, requiredPermission)
		if err != nil {
			a.logger.Errorf("failed to check permission for user %s on module %s: %v", user.ID, moduleName, err)
			return nil, ErrPermissionDenied
		}

		if !hasPermission {
			a.logger.Warnf("user %s does not have %s permission on module %s", user.Name, requiredPermission, moduleName)
			return nil, ErrPermissionDenied
		}

		return handler(ctx, req)
	}
}

func (a *authzMiddleware) authorizeAcknowledgeDriftEvent(ctx context.Context, user *model.User, req interface{}) (bool, error) {
	eventID := extractEventID(req)
	if parsedID, err := uuid.Parse(eventID); err == nil {
		moduleName, err := a.driftRepo.GetDriftEventModuleName(ctx, parsedID.String())
		if err == nil {
			return a.aclRepo.CheckPermission(ctx, user.ID, moduleName, model.PermissionWrite)
		}
		if !stderrors.Is(err, data.ErrDriftEventNotFound) {
			return false, err
		}
	}

	entries, err := a.aclRepo.ListUserPermissions(ctx, user.ID)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry != nil && (entry.Permission == model.PermissionWrite || entry.Permission == model.PermissionAdmin) {
			return true, nil
		}
	}
	return false, nil
}

// getRequiredPermission determines the required permission level based on the gRPC method
func getRequiredPermission(operation string, req interface{}) (model.Permission, string) {
	// UserService operations - only admin can access
	if strings.Contains(operation, "UserService/") {
		return model.PermissionAdmin, "*"
	}

	// Metadata service operations
	if strings.Contains(operation, "MetadataService/") {
		// Read operations
		moduleName := extractModuleName(req)
		return model.PermissionRead, moduleName
	}

	// DriftService operations
	if strings.Contains(operation, "DriftService/") {
		switch {
		case strings.Contains(operation, "/ListDriftEvents"):
			// Global read operation
			return model.PermissionRead, "*"
		case strings.Contains(operation, "/GetModuleDriftEvents"),
			strings.Contains(operation, "/GetModuleDependencyDriftStatus"):
			// Module-scoped read operations
			moduleName := extractModuleName(req)
			return model.PermissionRead, moduleName
		case strings.Contains(operation, "/AcknowledgeDriftEvent"):
			// Write operation - acknowledging drift events
			return model.PermissionWrite, "*"
		default:
			return model.PermissionRead, "*"
		}
	}

	// Registry service operations
	switch {
	case strings.Contains(operation, "/ListModules"):
		// ListModules filters results to permitted modules in the repository layer
		return "", ""

	case strings.Contains(operation, "/GetModule"),
		strings.Contains(operation, "/PullModule"),
		strings.Contains(operation, "/GetModuleDependencies"):
		// Read operations
		moduleName := extractModuleName(req)
		return model.PermissionRead, moduleName

	case strings.Contains(operation, "/RegisterModule"),
		strings.Contains(operation, "/PushModule"):
		// Write operations
		moduleName := extractModuleName(req)
		return model.PermissionWrite, moduleName

	case strings.Contains(operation, "/DeleteModule"),
		strings.Contains(operation, "/DeleteModuleTag"):
		// Admin operations
		moduleName := extractModuleName(req)
		return model.PermissionAdmin, moduleName

	default:
		// No permission check required (e.g., health checks)
		return "", ""
	}
}

// extractModuleName attempts to extract module name from the request
func extractModuleName(req interface{}) string {
	type moduleNameGetter interface {
		GetModuleName() string
	}

	type nameGetter interface {
		GetName() string
	}

	if r, ok := req.(moduleNameGetter); ok {
		if name := r.GetModuleName(); name != "" {
			return name
		}
	}

	if r, ok := req.(nameGetter); ok {
		if name := r.GetName(); name != "" {
			return name
		}
	}

	// Default to wildcard - checks global permissions
	return "*"
}

func extractEventID(req interface{}) string {
	type eventIDGetter interface {
		GetEventId() string
	}
	if r, ok := req.(eventIDGetter); ok {
		return r.GetEventId()
	}
	return ""
}
