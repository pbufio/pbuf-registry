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
	if _, err := fmt.Fprintln(tw, "PACKAGE\tFILES\tMESSAGES\tSERVICES\tRPCS"); err != nil {
		return err
	}
	for _, pkg := range packages {
		files := pkg.GetProtoFiles()
		messages, services, methods := 0, 0, 0
		for _, file := range files {
			messages += countMetadataMessages(file.GetMessages())
			services += len(file.GetServices())
			for _, service := range file.GetServices() {
				methods += len(service.GetMethods())
			}
		}
		name := pkg.GetName()
		if name == "" {
			name = "(no package)"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\n", tableText(name), len(files), messages, services, methods); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func countMetadataMessages(messages []*v1.Message) int {
	count := 0
	groups := [][]*v1.Message{messages}
	for len(groups) > 0 {
		last := len(groups) - 1
		group := groups[last]
		groups = groups[:last]
		count += len(group)
		for _, message := range group {
			if nested := message.GetNestedMessages(); len(nested) > 0 {
				groups = append(groups, nested)
			}
		}
	}
	return count
}
