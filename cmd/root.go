package main

import (
	"time"

	"github.com/go-co-op/gocron"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/background"
	"github.com/spf13/cobra"
)

func CreateRootCommand(launcher *Launcher) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:   "pbuf-registry",
		Short: "Default command to launch main application",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := launcher.mainApp.Run()
			if err != nil {
				return err
			}

			return nil
		},
	}

	rootCommand.AddCommand(CreateCompactionDaemon(launcher))
	rootCommand.AddCommand(CreateProtoParsingDaemon(launcher))

	return rootCommand
}

func CreateCompactionDaemon(launcher *Launcher) *cobra.Command {
	compactionDaemonCommand := &cobra.Command{
		Use:   "compaction",
		Short: "Run compaction daemon",
		Run: func(cmd *cobra.Command, args []string) {
			runBackgroundDaemon(
				launcher.config.Daemons.Compaction.CronSchedule,
				launcher.compactionDaemon,
				launcher.debugApp,
			)
		},
	}

	return compactionDaemonCommand
}

func CreateProtoParsingDaemon(launcher *Launcher) *cobra.Command {
	protoParsingDaemonCommand := &cobra.Command{
		Use:   "proto-parsing",
		Short: "Run proto parsing daemon",
		Run: func(cmd *cobra.Command, args []string) {
			runBackgroundDaemon(
				launcher.config.Daemons.ProtoParsing.CronSchedule,
				launcher.protoParsingDaemon,
				launcher.debugApp,
			)
		},
	}

	return protoParsingDaemonCommand
}

func runBackgroundDaemon(cronSchedule string, daemon background.Daemon, debugApp *kratos.App) {
	s := gocron.NewScheduler(time.UTC)

	// start every hour
	_, err := s.Cron(cronSchedule).Do(func() {
		err := daemon.Run()
		if err != nil {
			log.Fatalf("failed to run %s daemon: %v", daemon.Name(), err)
		}
	})

	if err != nil {
		log.Fatalf("failed to create cron job: %v", err)
	}

	// start the scheduler
	s.StartAsync()

	err = debugApp.Run()
	if err != nil {
		log.Fatalf("failed to run debug app: %v", err)
	}

	s.Stop()

	log.Infof("%s daemon stopped", daemon.Name())
}
