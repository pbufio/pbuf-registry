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
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
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

		if operation == v1.OperationDriftServiceAcknowledgeDriftEvent && a.driftRepo != nil {
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

		if operation == v1.OperationRegistryPushModule {
			if err := a.authorizePushDependencies(ctx, user, moduleName, req); err != nil {
				return nil, err
			}
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

func (a *authzMiddleware) authorizePushDependencies(ctx context.Context, user *model.User, moduleName string, req interface{}) error {
	type dependenciesGetter interface {
		GetDependencies() []*v1.Dependency
	}
	r, ok := req.(dependenciesGetter)
	if !ok {
		return nil
	}
	checked := make(map[string]struct{}, len(r.GetDependencies()))
	for _, dep := range r.GetDependencies() {
		if dep == nil {
			continue
		}
		depName := dep.GetName()
		if depName == "" || depName == moduleName {
			continue
		}
		if _, seen := checked[depName]; seen {
			continue
		}
		checked[depName] = struct{}{}
		allowed, err := a.aclRepo.CheckPermission(ctx, user.ID, depName, model.PermissionRead)
		if err != nil {
			a.logger.Errorf("failed to check read permission for user %s on dependency %s: %v", user.ID, depName, err)
			return ErrPermissionDenied
		}
		if !allowed {
			a.logger.Warnf("user %s does not have read permission on dependency %s", user.Name, depName)
			return ErrPermissionDenied
		}
	}
	return nil
}

// getRequiredPermission determines the required permission level based on the gRPC method
func getRequiredPermission(operation string, req interface{}) (model.Permission, string) {
	if strings.HasPrefix(operation, "/grpc.health.v1.Health/") {
		return "", ""
	}

	switch operation {
	case v1.OperationRegistryListModules, "/pbufregistry.v1.MCP/Handle":
		// ListModules filters results per-module in the repository layer;
		// MCP/Handle authenticates discovery/session calls and re-invokes middleware per tool call.
		return "", ""

	case v1.OperationRegistryGetModule,
		v1.OperationRegistryPullModule,
		v1.OperationRegistryGetModuleDependencies,
		v1.OperationMetadataServiceGetMetadata,
		v1.OperationDriftServiceGetModuleDriftEvents,
		v1.OperationDriftServiceGetModuleDependencyDriftStatus:
		return model.PermissionRead, extractModuleName(req)

	case v1.OperationDriftServiceListDriftEvents:
		return model.PermissionRead, "*"

	case v1.OperationRegistryRegisterModule,
		v1.OperationRegistryPushModule:
		return model.PermissionWrite, extractModuleName(req)

	case v1.OperationDriftServiceAcknowledgeDriftEvent:
		return model.PermissionWrite, "*"

	case v1.OperationRegistryDeleteModule,
		v1.OperationRegistryDeleteModuleTag:
		return model.PermissionAdmin, extractModuleName(req)

	default:
		// UserService and any unlisted operations default-deny to global admin.
		return model.PermissionAdmin, "*"
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
