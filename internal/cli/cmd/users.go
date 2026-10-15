package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

func NewUsersCmd(_ *model.Config, client v1.UserServiceClient) *cobra.Command {
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "Users",
		Long:  "Users is a command to manage users, bots, and permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	usersCmd.AddCommand(newCreateUserCmd(client))
	usersCmd.AddCommand(newListUsersCmd(client))
	usersCmd.AddCommand(newGetUserCmd(client))
	usersCmd.AddCommand(newUpdateUserCmd(client))
	usersCmd.AddCommand(newDeleteUserCmd(client))
	usersCmd.AddCommand(newRegenerateTokenCmd(client))
	usersCmd.AddCommand(newGrantPermissionCmd(client))
	usersCmd.AddCommand(newRevokePermissionCmd(client))
	usersCmd.AddCommand(newListUserPermissionsCmd(client))

	return usersCmd
}

func newCreateUserCmd(client v1.UserServiceClient) *cobra.Command {
	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create",
		Long:  "Create is a command to create a user or bot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			userTypeStr, err := cmd.Flags().GetString("type")
			if err != nil {
				return err
			}
			userType, err := parseUserType(userTypeStr)
			if err != nil {
				return err
			}

			resp, err := client.CreateUser(cmd.Context(), &v1.CreateUserRequest{
				Name: name,
				Type: userType,
			})
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	createCmd.Flags().String("type", "user", "user type: user|bot")
	return createCmd
}

func newListUsersCmd(client v1.UserServiceClient) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List",
		Long:  "List is a command to list users and bots",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageSize, err := cmd.Flags().GetInt32("page-size")
			if err != nil {
				return err
			}
			page, err := cmd.Flags().GetInt32("page")
			if err != nil {
				return err
			}

			resp, err := client.ListUsers(cmd.Context(), &v1.ListUsersRequest{
				PageSize: pageSize,
				Page:     page,
			})
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	listCmd.Flags().Int32("page-size", 50, "max number of users to return")
	listCmd.Flags().Int32("page", 0, "page number (0-indexed)")
	return listCmd
}

func newGetUserCmd(client v1.UserServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get",
		Long:  "Get is a command to get a user or bot by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			user, err := client.GetUser(cmd.Context(), &v1.GetUserRequest{Id: id})
			if err != nil {
				return err
			}
			return printJSON(user)
		},
	}

	return getCmd
}

func newUpdateUserCmd(client v1.UserServiceClient) *cobra.Command {
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update",
		Long:  "Update is a command to update a user or bot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			active, err := cmd.Flags().GetBool("active")
			if err != nil {
				return err
			}
			inactive, err := cmd.Flags().GetBool("inactive")
			if err != nil {
				return err
			}

			if active && inactive {
				return errors.New("only one of --active or --inactive can be set")
			}
			if name == "" && !active && !inactive {
				return errors.New("nothing to update: specify --name, --active, or --inactive")
			}

			isActive := false
			switch {
			case active:
				isActive = true
			case inactive:
				isActive = false
			default:
				// Server always applies IsActive from request; preserve current value if not explicitly set.
				user, err := client.GetUser(cmd.Context(), &v1.GetUserRequest{Id: id})
				if err != nil {
					return err
				}
				isActive = user.IsActive
			}

			updated, err := client.UpdateUser(cmd.Context(), &v1.UpdateUserRequest{
				Id:       id,
				Name:     name,
				IsActive: isActive,
			})
			if err != nil {
				return err
			}
			return printJSON(updated)
		},
	}

	updateCmd.Flags().String("name", "", "new name")
	updateCmd.Flags().Bool("active", false, "set user active")
	updateCmd.Flags().Bool("inactive", false, "set user inactive")
	return updateCmd
}

func newDeleteUserCmd(client v1.UserServiceClient) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete",
		Long:  "Delete is a command to delete a user or bot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			resp, err := client.DeleteUser(cmd.Context(), &v1.DeleteUserRequest{Id: id})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	return deleteCmd
}

func newRegenerateTokenCmd(client v1.UserServiceClient) *cobra.Command {
	regenCmd := &cobra.Command{
		Use:   "regenerate-token [id]",
		Short: "Regenerate token",
		Long:  "Regenerate token is a command to regenerate a user or bot token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			resp, err := client.RegenerateToken(cmd.Context(), &v1.RegenerateTokenRequest{Id: id})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	return regenCmd
}

func newGrantPermissionCmd(client v1.UserServiceClient) *cobra.Command {
	grantCmd := &cobra.Command{
		Use:   "grant-permission [user_id] [module_name]",
		Short: "Grant permission",
		Long:  "Grant permission is a command to grant permission to a user or bot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID := args[0]
			moduleName := args[1]
			permissionStr, err := cmd.Flags().GetString("permission")
			if err != nil {
				return err
			}
			permission, err := parsePermission(permissionStr)
			if err != nil {
				return err
			}

			resp, err := client.GrantPermission(cmd.Context(), &v1.GrantPermissionRequest{
				UserId:     userID,
				ModuleName: moduleName,
				Permission: permission,
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	grantCmd.Flags().String("permission", "", "permission: read|write|admin")
	_ = grantCmd.MarkFlagRequired("permission")
	return grantCmd
}

func newRevokePermissionCmd(client v1.UserServiceClient) *cobra.Command {
	revokeCmd := &cobra.Command{
		Use:   "revoke-permission [user_id] [module_name]",
		Short: "Revoke permission",
		Long:  "Revoke permission is a command to revoke permission from a user or bot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID := args[0]
			moduleName := args[1]
			resp, err := client.RevokePermission(cmd.Context(), &v1.RevokePermissionRequest{
				UserId:     userID,
				ModuleName: moduleName,
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	return revokeCmd
}

func newListUserPermissionsCmd(client v1.UserServiceClient) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list-permissions [user_id]",
		Short: "List permissions",
		Long:  "List permissions is a command to list permissions for a user or bot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID := args[0]
			resp, err := client.ListUserPermissions(cmd.Context(), &v1.ListUserPermissionsRequest{UserId: userID})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	return listCmd
}

func parseUserType(s string) (v1.UserType, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "user":
		return v1.UserType_USER_TYPE_USER, nil
	case "bot":
		return v1.UserType_USER_TYPE_BOT, nil
	default:
		return v1.UserType_USER_TYPE_UNSPECIFIED, fmt.Errorf("unknown user type %q (expected user|bot)", s)
	}
}

func parsePermission(s string) (v1.Permission, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "read":
		return v1.Permission_PERMISSION_READ, nil
	case "write":
		return v1.Permission_PERMISSION_WRITE, nil
	case "admin":
		return v1.Permission_PERMISSION_ADMIN, nil
	default:
		return v1.Permission_PERMISSION_UNSPECIFIED, fmt.Errorf("unknown permission %q (expected read|write|admin)", s)
	}
}

func printJSON(v any) error {
	marshalled, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	log.Printf("%+v", string(marshalled))
	return nil
}
