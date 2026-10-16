package cmd

import (
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

func NewMetadataCmd(config *model.Config, client v1.MetadataServiceClient) *cobra.Command {
	metadataCmd := &cobra.Command{
		Use:   "metadata",
		Short: "Metadata",
		Long:  "Metadata is a command to interact with module metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	metadataCmd.AddCommand(newGetMetadataCmd(config, client))

	return metadataCmd
}

func newGetMetadataCmd(config *model.Config, client v1.MetadataServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get [module_name] [tag]",
		Short: "Get metadata",
		Long:  "Get is a command to get parsed metadata (packages) for a module tag",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleName := config.Name
			if len(args) > 0 {
				moduleName = args[0]
			}
			if moduleName == "" {
				return cmd.Help()
			}

			tag := ""
			if len(args) > 1 {
				tag = args[1]
			}

			resp, err := client.GetMetadata(cmd.Context(), &v1.GetMetadataRequest{
				Name: moduleName,
				Tag:  tag,
			})
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	return getCmd
}
