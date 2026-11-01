package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/bsr"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/modules"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
	"github.com/pbufio/pbuf-registry/internal/mcp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// NewRootCmd creates cobra root command via function
func NewRootCmd() *cobra.Command {
	// read the file (PbufConfigFilename) and call ModulesConfig.Vendor()
	file, err := os.ReadFile(model.PbufConfigFilename)
	configNotFound := os.IsNotExist(err)
	if err != nil && !configNotFound {
		log.Fatalf("failed to read %s file: %v", model.PbufConfigFilename, err)
	}

	// create root command
	rootCmd := &cobra.Command{
		Use:   "pbuf",
		Short: "PowerBuf CLI",
		Long:  "PowerBuf CLI is a command line interface for PowerBuf",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configNotFound {
				log.Printf("no %s file found. please use `init` command", model.PbufConfigFilename)
			}

			return cmd.Help()
		},
	}

	var netrcAuth *netrc.Netrc

	usr, err := user.Current()
	if err == nil {
		netrcAuth, _ = netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
		// Ignore error if .netrc doesn't exist - auth command will create it
	} else {
		log.Printf("failed to fetch current user: %v", err)
	}

	envRegistryAddr := os.Getenv("PBUF_REGISTRY_ADDR")
	if configNotFound && envRegistryAddr == "" {
		rootCmd.AddCommand(CreateInitCmd())
		rootCmd.AddCommand(NewAuthCmd(nil, usr, netrcAuth))
		rootCmd.AddCommand(NewLoginCmd(nil, usr, netrcAuth))
		return rootCmd
	}

	modulesConfig := &model.Config{}
	if !configNotFound {
		modulesConfig, err = modules.NewConfig(file)
		if err != nil {
			log.Fatalf("failed to create modules config: %v", err)
		}
	}
	if envRegistryAddr != "" {
		modulesConfig.Registry.Addr = envRegistryAddr
		if os.Getenv("PBUF_REGISTRY_INSECURE") == "true" {
			modulesConfig.Registry.Insecure = true
		}
	}

	if modulesConfig.HasRegistry() {
		var conn grpc.ClientConnInterface
		if modulesConfig.Registry.Insecure {
			conn = registry.NewInsecureConn(modulesConfig, netrcAuth)
		} else {
			conn = registry.NewSecureConn(modulesConfig, netrcAuth)
		}

		registryClient := v1.NewRegistryClient(conn)
		usersClient := v1.NewUserServiceClient(conn)
		driftClient := v1.NewDriftServiceClient(conn)
		metadataClient := v1.NewMetadataServiceClient(conn)

		rootCmd.AddCommand(NewModuleCmd(modulesConfig, registryClient))
		rootCmd.AddCommand(NewVendorCmd(modulesConfig, netrcAuth, registryClient))
		rootCmd.AddCommand(NewAuthCmd(modulesConfig, usr, netrcAuth))
		rootCmd.AddCommand(NewLoginCmd(modulesConfig, usr, netrcAuth))
		rootCmd.AddCommand(NewUsersCmd(modulesConfig, usersClient))
		rootCmd.AddCommand(NewDriftCmd(modulesConfig, driftClient, registryClient))
		rootCmd.AddCommand(NewMetadataCmd(modulesConfig, metadataClient))
		rootCmd.AddCommand(NewMCPCmd(registryClient, metadataClient, driftClient))
	} else {
		rootCmd.AddCommand(NewVendorCmd(modulesConfig, netrcAuth, nil))
		rootCmd.AddCommand(NewAuthCmd(modulesConfig, usr, netrcAuth))
		rootCmd.AddCommand(NewLoginCmd(modulesConfig, usr, netrcAuth))
	}

	return rootCmd
}

// NewMCPCmd creates the `pbuf mcp` command running the MCP server over stdio.
func NewMCPCmd(registryClient v1.RegistryClient, metadataClient v1.MetadataServiceClient, driftClient v1.DriftServiceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server over stdio",
		Long:  "Start a Model Context Protocol (MCP) server over stdio for AI coding assistants",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcp.NewServer(registryClient, metadataClient, driftClient).ServeStdio(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

func NewAuthCmd(config *model.Config, usr *user.User, auth *netrc.Netrc) *cobra.Command {
	return &cobra.Command{
		Use:   "auth [token]",
		Short: "Auth",
		Long:  "Auth is a command to setup .netrc file with auth token",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := saveTokenToNetrc(config, usr, auth, args[0]); err != nil {
				log.Fatalf("failed to save .netrc file: %v", err)
			}
		},
	}
}

func NewModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create module command
	moduleCmd := &cobra.Command{
		Use:   "modules",
		Short: "Modules",
		Long:  "Modules is a command to manage modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// add subcommands
	moduleCmd.AddCommand(NewRegisterModuleCmd(config, client))
	moduleCmd.AddCommand(NewPushModuleCmd(config, client))
	moduleCmd.AddCommand(NewDeleteTagCmd(config, client))
	moduleCmd.AddCommand(NewDeleteModuleCmd(config, client))

	moduleCmd.AddCommand(NewListModulesCmd(config, client))
	moduleCmd.AddCommand(NewGetModuleCmd(config, client))

	moduleCmd.AddCommand(NewModuleUpdateCmd(config, client))

	return moduleCmd
}

func NewRegisterModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create register module command
	registerModuleCmd := &cobra.Command{
		Use:   "register [module_name]",
		Short: "Register",
		Long:  "Register is a command to register modules",
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			moduleName := config.Name
			if len(args) > 0 && args[0] != "" {
				moduleName = args[0]
			}
			if moduleName == "" {
				log.Fatalf("module name is required. see pbuf.yaml reference")
			}

			module, err := client.RegisterModule(cmd.Context(), &v1.RegisterModuleRequest{
				Name: moduleName,
			})

			if err != nil {
				log.Fatalf("failed to register: %v", err)
			}

			log.Printf("module %s successfully registered", module.Name)
		},
	}

	return registerModuleCmd
}

func NewPushModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create push module command
	pushModuleCmd := &cobra.Command{
		Use:   "push [tag] [flags]",
		Short: "Push",
		Long:  "Push is a command to push modules",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if config.Name == "" {
				log.Fatalf("module name is required. see pbuf.yaml reference")
			}

			tag := args[0]

			isDraft, err := cmd.Flags().GetBool("draft")
			if err != nil {
				log.Fatalf("failed to get draft flag: %v", err)
				return
			}

			protoFiles, err := registry.CollectProtoFilesInDirs(config.Export.Paths)
			if err != nil {
				log.Fatalf("failed to collect proto files: %v", err)
			}

			var dependencies []*v1.Dependency
			for _, dependency := range config.Modules {
				if dependency.Name != "" && dependency.Repository == "" && !bsr.IsBSRModule(dependency) {
					dependencies = append(dependencies, &v1.Dependency{
						Name: dependency.Name,
						Tag:  dependency.Tag,
					})
				}
			}

			log.Printf("pushing module %s with tag %s", config.Name, tag)

			module, err := client.PushModule(cmd.Context(), &v1.PushModuleRequest{
				ModuleName:   config.Name,
				Tag:          tag,
				Protofiles:   protoFiles,
				Dependencies: dependencies,
				IsDraft:      isDraft,
			})

			if err != nil {
				log.Fatalf("failed to push: %v", err)
			}

			log.Printf("module %s successfully pushed", module.Name)
		},
	}

	pushModuleCmd.PersistentFlags().Bool("draft", false, "push draft module")

	return pushModuleCmd
}

func NewDeleteTagCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create delete tag command
	deleteTagCmd := &cobra.Command{
		Use:   "delete-tag [tag]",
		Short: "Delete tag",
		Long:  "Delete tag is a command to delete tags",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if config.Name == "" {
				log.Fatalf("module name is required. see pbuf.yaml reference")
			}

			tag := args[0]

			if tag == "" {
				log.Fatalf("tag is required")
			}

			_, err := client.DeleteModuleTag(cmd.Context(), &v1.DeleteModuleTagRequest{
				Name: config.Name,
				Tag:  tag,
			})

			if err != nil {
				log.Fatalf("failed to delete tag: %v", err)
			}

			log.Printf("tag %s successfully deleted", tag)
		},
	}

	return deleteTagCmd
}

func NewDeleteModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create delete module command
	deleteModuleCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete",
		Long:  "Delete is a command to delete modules",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			if config.Name == "" {
				log.Fatalf("module name is required. see pbuf.yaml reference")
			}

			_, err := client.DeleteModule(cmd.Context(), &v1.DeleteModuleRequest{
				Name: config.Name,
			})

			if err != nil {
				log.Fatalf("failed to delete: %v", err)
			}

			log.Printf("module %s successfully deleted", config.Name)
		},
	}

	return deleteModuleCmd
}

type moduleDetailsOutput struct {
	Module       *v1.Module       `json:"module"`
	Dependencies []*v1.Dependency `json:"dependencies"`
}

func parseOutputFormat(cmd *cobra.Command) (string, error) {
	raw, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "table", "":
		return "table", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("invalid --output value %q (must be table or json)", raw)
	}
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

// NewGetModuleCmd creates cobra command for get module
func NewGetModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	getModuleCmd := &cobra.Command{
		Use:   "get [module_name]",
		Short: "Get",
		Long:  "Get is a command to get modules",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFormat, err := parseOutputFormat(cmd)
			if err != nil {
				return err
			}

			var moduleName string
			if len(args) > 0 && args[0] != "" {
				moduleName = args[0]
			} else if config != nil {
				moduleName = config.Name
			}
			if moduleName == "" {
				return fmt.Errorf("module name is required. see pbuf.yaml reference")
			}

			module, err := client.GetModule(cmd.Context(), &v1.GetModuleRequest{
				Name: moduleName,
			})
			if err != nil {
				return fmt.Errorf("failed to get: %w", err)
			}

			moduleDependencies, err := client.GetModuleDependencies(cmd.Context(), &v1.GetModuleDependenciesRequest{
				Name: moduleName,
			})
			if err != nil {
				return fmt.Errorf("failed to fetch dependencies: %w", err)
			}

			return renderModuleDetails(cmd.OutOrStdout(), module, moduleDependencies.GetDependencies(), outputFormat)
		},
	}

	getModuleCmd.Flags().StringP("output", "o", "table", "output format: table|json")
	return getModuleCmd
}

func renderModuleDetails(w io.Writer, module *v1.Module, dependencies []*v1.Dependency, outputFormat string) error {
	if outputFormat == "json" {
		deps := dependencies
		if deps == nil {
			deps = []*v1.Dependency{}
		}
		marshalled, err := json.MarshalIndent(moduleDetailsOutput{
			Module:       module,
			Dependencies: deps,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal module: %w", err)
		}
		_, err = fmt.Fprintln(w, string(marshalled))
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "NAME:\t%s\n", module.GetName()); err != nil {
		return err
	}
	if module.GetId() != "" {
		if _, err := fmt.Fprintf(tw, "ID:\t%s\n", module.GetId()); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(tw, "TAGS:\t%s\n", joinOrDash(module.GetTags())); err != nil {
		return err
	}
	if len(module.GetDraftTags()) > 0 {
		if _, err := fmt.Fprintf(tw, "DRAFT TAGS:\t%s\n", strings.Join(module.GetDraftTags(), ", ")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(tw, "PACKAGES:\t%s\n", joinOrDash(module.GetPackages())); err != nil {
		return err
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	if len(dependencies) == 0 {
		_, err := fmt.Fprintln(w, "\nDependencies: none")
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	depTW := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(depTW, "DEPENDENCY\tTAG\tTYPE"); err != nil {
		return err
	}
	for _, dep := range dependencies {
		depType := dep.GetDependencyType()
		if depType == "" {
			depType = "direct"
		}
		if _, err := fmt.Fprintf(depTW, "%s\t%s\t%s\n", dep.GetName(), dep.GetTag(), depType); err != nil {
			return err
		}
	}
	return depTW.Flush()
}

// NewListModulesCmd creates cobra command for list modules
func NewListModulesCmd(_ *model.Config, client v1.RegistryClient) *cobra.Command {
	listModulesCmd := &cobra.Command{
		Use:   "list",
		Short: "List",
		Long:  "List is a command to list modules",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			outputFormat, err := parseOutputFormat(cmd)
			if err != nil {
				return err
			}

			pageSize, err := cmd.Flags().GetInt32("page-size")
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("page-size") && pageSize <= 0 {
				return fmt.Errorf("--page-size must be positive")
			}

			pageToken, err := cmd.Flags().GetString("page-token")
			if err != nil {
				return err
			}

			fetchAll, err := cmd.Flags().GetBool("all")
			if err != nil {
				return err
			}

			var modulesList *v1.ListModulesResponse
			if fetchAll {
				var allModules []*v1.Module
				curToken := pageToken
				for {
					resp, err := client.ListModules(cmd.Context(), &v1.ListModulesRequest{
						PageSize:  pageSize,
						PageToken: curToken,
					})
					if err != nil {
						return fmt.Errorf("failed to fetch modules list: %w", err)
					}
					allModules = append(allModules, resp.GetModules()...)
					next := resp.GetNextPageToken()
					if next == "" || next == curToken {
						break
					}
					curToken = next
				}
				modulesList = &v1.ListModulesResponse{Modules: allModules}
			} else {
				resp, err := client.ListModules(cmd.Context(), &v1.ListModulesRequest{
					PageSize:  pageSize,
					PageToken: pageToken,
				})
				if err != nil {
					return fmt.Errorf("failed to fetch modules list: %w", err)
				}
				modulesList = resp
			}

			return renderModulesList(cmd.OutOrStdout(), modulesList, outputFormat)
		},
	}

	listModulesCmd.Flags().StringP("output", "o", "table", "output format: table|json")
	listModulesCmd.Flags().Int32("page-size", 50, "max number of modules per page")
	listModulesCmd.Flags().String("page-token", "", "page token for pagination")
	listModulesCmd.Flags().Bool("all", false, "fetch all pages of modules")
	return listModulesCmd
}

func renderModulesList(w io.Writer, modulesList *v1.ListModulesResponse, outputFormat string) error {
	if outputFormat == "json" {
		marshalled, err := json.MarshalIndent(modulesList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal modules list: %w", err)
		}
		_, err = fmt.Fprintln(w, string(marshalled))
		return err
	}

	modules := modulesList.GetModules()
	if len(modules) == 0 {
		_, err := fmt.Fprintln(w, "No modules found.")
		return err
	}

	hasTags := false
	for _, mod := range modules {
		if len(mod.GetTags()) > 0 {
			hasTags = true
			break
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if hasTags {
		if _, err := fmt.Fprintln(tw, "NAME\tID\tLATEST TAG\tTAGS"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(tw, "NAME\tID"); err != nil {
			return err
		}
	}
	for _, mod := range modules {
		modID := mod.GetId()
		if modID == "" {
			modID = "-"
		}
		if hasTags {
			latestTag := "-"
			if len(mod.GetTags()) > 0 {
				latestTag = mod.GetTags()[0]
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", mod.GetName(), modID, latestTag, joinOrDash(mod.GetTags())); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(tw, "%s\t%s\n", mod.GetName(), modID); err != nil {
				return err
			}
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if modulesList.GetNextPageToken() != "" {
		_, err := fmt.Fprintf(w, "\nNext page token: %s\n", modulesList.GetNextPageToken())
		return err
	}
	return nil
}

func NewModuleUpdateCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create module update command
	moduleUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update",
		Long:  "Update is a command to update modules tags to the latest ones",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			for _, module := range config.Modules {
				if module.Name != "" && module.Repository == "" && !bsr.IsBSRModule(module) {
					moduleWithTags, err := client.GetModule(cmd.Context(), &v1.GetModuleRequest{
						Name: module.Name,
					})

					if err != nil {
						log.Fatalf("failed to get module: %v", err)
					}

					if len(moduleWithTags.Tags) > 0 {
						module.Tag = moduleWithTags.Tags[0]
					}
				}
			}

			err := config.Save()
			if err != nil {
				log.Fatalf("failed to update config. error during saving: %v", err)
			}
		},
	}

	return moduleUpdateCmd
}

// NewVendorCmd creates cobra command for vendor
func NewVendorCmd(modulesConfig *model.Config, netrcAuth *netrc.Netrc, client v1.RegistryClient) *cobra.Command {
	// create vendor command
	vendorCmd := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor",
		Long:  "Vendor is a command to vendor modules",
		Run: func(cmd *cobra.Command, args []string) {
			err := modules.Vendor(modulesConfig, netrcAuth, client)
			if err != nil {
				log.Fatalf("failed to vendor: %v", err)
			}
		},
	}

	return vendorCmd
}
