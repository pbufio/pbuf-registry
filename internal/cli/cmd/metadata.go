package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

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
			outputFormat, err := parseOutputFormat(cmd)
			if err != nil {
				return err
			}
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

			if outputFormat == "json" {
				return printJSON(cmd.OutOrStdout(), resp)
			}
			return printMetadataTable(cmd.OutOrStdout(), resp.GetPackages())
		},
	}

	getCmd.Flags().StringP("output", "o", "json", "output format: table|json")
	return getCmd
}

func printMetadataTable(w io.Writer, packages []*v1.Package) error {
	if len(packages) == 0 {
		_, err := fmt.Fprintln(w, "No packages found.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PACKAGE\tFILES\tMESSAGES\tENUMS\tSERVICES\tRPCS"); err != nil {
		return err
	}
	for _, pkg := range packages {
		files := pkg.GetProtoFiles()
		messages, enums, services, methods := 0, 0, 0, 0
		for _, file := range files {
			msgCount, enumCount := countMetadataTypes(file.GetMessages(), 0)
			messages += msgCount
			enums += len(file.GetEnums()) + enumCount
			services += len(file.GetServices())
			for _, service := range file.GetServices() {
				methods += len(service.GetMethods())
			}
		}
		name := pkg.GetName()
		if name == "" {
			name = "(no package)"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\n", tableText(name), len(files), messages, enums, services, methods); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func countMetadataTypes(messages []*v1.Message, depth int) (int, int) {
	if depth > 100 {
		return 0, 0
	}
	msgCount := len(messages)
	enumCount := 0
	for _, m := range messages {
		enumCount += len(m.GetNestedEnums())
		subMsgs, subEnums := countMetadataTypes(m.GetNestedMessages(), depth+1)
		msgCount += subMsgs
		enumCount += subEnums
	}
	return msgCount, enumCount
}
