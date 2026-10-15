package cmd

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"

	"github.com/jdx/go-netrc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/bsr"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/pbufio/pbuf-registry/internal/cli/modules"
	"github.com/pbufio/pbuf-registry/internal/cli/registry"
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

	if configNotFound {
		rootCmd.AddCommand(CreateInitCmd())
		return rootCmd
	}

	modulesConfig, err := modules.NewConfig(file)
	if err != nil {
		log.Fatalf("failed to create modules config: %v", err)
	}

	var netrcAuth *netrc.Netrc

	usr, err := user.Current()
	if err == nil {
		netrcAuth, _ = netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
		// Ignore error if .netrc doesn't exist - auth command will create it
	} else {
		log.Printf("failed to fetch current user: %v", err)
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
		rootCmd.AddCommand(NewDriftCmd(modulesConfig, driftClient))
		rootCmd.AddCommand(NewMetadataCmd(modulesConfig, metadataClient))
	} else {
		rootCmd.AddCommand(NewVendorCmd(modulesConfig, netrcAuth, nil))
		rootCmd.AddCommand(NewLoginCmd(modulesConfig, usr, netrcAuth))
	}

	return rootCmd
}

func NewAuthCmd(config *model.Config, usr *user.User, auth *netrc.Netrc) *cobra.Command {
	// create login command
	authCmd := &cobra.Command{
		Use:   "auth [token]",
		Short: "Auth",
		Long:  "Auth is a command to setup .netrc file with auth token",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if auth == nil {
				auth = &netrc.Netrc{
					Path: filepath.Join(usr.HomeDir, ".netrc"),
				}
			}

			token := args[0]

			machine := config.Registry.Addr
			machine, _, err := net.SplitHostPort(machine)
			if err != nil {
				machine = config.Registry.Addr
			}

			m := auth.Machine(machine)
			if m == nil {
				auth.AddMachine(machine, "unused", "unused")
				m = auth.Machine(machine)
			}
			m.Set("token", token)

			err = auth.Save()
			if err != nil {
				log.Fatalf("failed to save .netrc file: %v", err)
			}
		},
	}

	return authCmd
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
		Use:   "register",
		Short: "Register",
		Long:  "Register is a command to register modules",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			if config.Name == "" {
				log.Fatalf("module name is required. see pbuf.yaml reference")
			}

			module, err := client.RegisterModule(cmd.Context(), &v1.RegisterModuleRequest{
				Name: config.Name,
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

// NewGetModuleCmd creates cobra command for get module
func NewGetModuleCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create get module command
	getModuleCmd := &cobra.Command{
		Use:   "get [module_name]",
		Short: "Get",
		Long:  "Get is a command to get modules",
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				if config.Name == "" {
					log.Fatalf("module name is required. see pbuf.yaml reference")
				}
				args = append(args, config.Name)
			}

			moduleName := args[0]

			module, err := client.GetModule(cmd.Context(), &v1.GetModuleRequest{
				Name: moduleName,
			})

			if err != nil {
				log.Fatalf("failed to get: %v", err)
			}

			moduleDependencies, err := client.GetModuleDependencies(cmd.Context(), &v1.GetModuleDependenciesRequest{
				Name: moduleName,
			})

			if err != nil {
				log.Fatalf("failed to fetch dependencies: %v", err)
			}

			// print as json pretty
			marshalled, err := json.MarshalIndent(module, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal module: %v", err)
			}
			log.Printf("module:\n%+v\n", string(marshalled))

			// print deps as json pretty
			marshalled, err = json.MarshalIndent(moduleDependencies, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal module dependencies: %v", err)
			}
			log.Printf("module dependencies:\n%+v\n", string(marshalled))
		},
	}

	return getModuleCmd
}

// NewListModulesCmd creates cobra command for list modules
func NewListModulesCmd(config *model.Config, client v1.RegistryClient) *cobra.Command {
	// create list modules command
	listModulesCmd := &cobra.Command{
		Use:   "list",
		Short: "List",
		Long:  "List is a command to list modules",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			modulesList, err := client.ListModules(cmd.Context(), &v1.ListModulesRequest{})
			if err != nil {
				log.Fatalf("failed to fetch modules list: %v", err)
			}

			// print as json pretty
			marshalled, err := json.MarshalIndent(modulesList, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal modules list: %v", err)
			}
			log.Printf("%+v", string(marshalled))
		},
	}

	return listModulesCmd
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
