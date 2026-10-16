package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

func NewDriftCmd(_ *model.Config, client v1.DriftServiceClient) *cobra.Command {
	driftCmd := &cobra.Command{
		Use:   "drift",
		Short: "Drift",
		Long:  "Drift is a command to manage drift detection events",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	driftCmd.AddCommand(newListDriftEventsCmd(client))
	driftCmd.AddCommand(newGetModuleDriftEventsCmd(client))
	driftCmd.AddCommand(newGetModuleDependencyDriftStatusCmd(client))
	driftCmd.AddCommand(newAcknowledgeDriftEventCmd(client))

	return driftCmd
}

func newListDriftEventsCmd(client v1.DriftServiceClient) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List drift events",
		Long:  "List is a command to list all drift events",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			unacknowledgedOnly, err := cmd.Flags().GetBool("unacknowledged-only")
			if err != nil {
				return err
			}

			resp, err := client.ListDriftEvents(cmd.Context(), &v1.ListDriftEventsRequest{
				UnacknowledgedOnly: unacknowledgedOnly,
			})
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	listCmd.Flags().Bool("unacknowledged-only", true, "only return unacknowledged events")
	return listCmd
}

func newGetModuleDriftEventsCmd(client v1.DriftServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "module [module_name]",
		Short: "Get module drift events",
		Long:  "Module is a command to get drift events for a specific module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleName := args[0]
			tagName, err := cmd.Flags().GetString("tag")
			if err != nil {
				return err
			}

			req := &v1.GetModuleDriftEventsRequest{
				ModuleName: moduleName,
			}
			if tagName != "" {
				req.TagName = &tagName
			}

			resp, err := client.GetModuleDriftEvents(cmd.Context(), req)
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	getCmd.Flags().String("tag", "", "filter drift events by tag name")
	return getCmd
}

func newGetModuleDependencyDriftStatusCmd(client v1.DriftServiceClient) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "dependencies [module_name]",
		Short: "Get dependency drift status",
		Long:  "Dependencies is a command to get dependency drift status for a specific module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			moduleName := args[0]
			tagName, err := cmd.Flags().GetString("tag")
			if err != nil {
				return err
			}

			req := &v1.GetModuleDependencyDriftStatusRequest{
				ModuleName: moduleName,
			}
			if tagName != "" {
				req.TagName = &tagName
			}

			resp, err := client.GetModuleDependencyDriftStatus(cmd.Context(), req)
			if err != nil {
				return err
			}

			return printDependencyDriftStatuses(cmd.OutOrStdout(), moduleName, tagName, resp.GetStatuses())
		},
	}

	getCmd.Flags().String("tag", "", "module tag to evaluate (latest stable when omitted)")
	return getCmd
}

func printDependencyDriftStatuses(w io.Writer, moduleName, tagName string, statuses []*v1.DependencyDriftStatus) error {
	if _, err := fmt.Fprintf(w, "Dependency drift status for %s", moduleName); err != nil {
		return err
	}
	if tagName != "" {
		if _, err := fmt.Fprintf(w, " (tag: %s)", tagName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if len(statuses) == 0 {
		_, err := fmt.Fprintln(w, "No dependency drift detected.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "DEPENDENCY\tCURRENT\tTARGET\tSEVERITY\tRECOMMENDATION"); err != nil {
		return err
	}

	for _, status := range statuses {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			status.GetDependencyName(),
			status.GetCurrentTag(),
			status.GetTargetTag(),
			status.GetSeverity().String(),
			status.GetRecommendation().String(),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func newAcknowledgeDriftEventCmd(client v1.DriftServiceClient) *cobra.Command {
	ackCmd := &cobra.Command{
		Use:   "acknowledge [event_id]",
		Short: "Acknowledge a drift event",
		Long:  "Acknowledge is a command to mark a drift event as acknowledged",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eventID := args[0]
			acknowledgedBy, err := cmd.Flags().GetString("by")
			if err != nil {
				return err
			}

			resp, err := client.AcknowledgeDriftEvent(cmd.Context(), &v1.AcknowledgeDriftEventRequest{
				EventId:        eventID,
				AcknowledgedBy: acknowledgedBy,
			})
			if err != nil {
				return err
			}

			return printJSON(resp)
		},
	}

	ackCmd.Flags().String("by", "", "identifier of the user acknowledging the event")
	return ackCmd
}
