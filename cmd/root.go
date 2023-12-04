package main

import (
	"time"

	"github.com/go-co-op/gocron"
	"github.com/go-kratos/kratos/v2/log"
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

	return rootCommand
}

func CreateCompactionDaemon(launcher *Launcher) *cobra.Command {
	compactionDaemonCommand := &cobra.Command{
		Use:   "compaction",
		Short: "Run compaction daemon",
		Run: func(cmd *cobra.Command, args []string) {
			s := gocron.NewScheduler(time.UTC)

			// start every hour
			_, err := s.Cron(launcher.config.Daemons.Compaction.CronSchedule).Do(func() {
				err := launcher.compactionDaemon.Run()
				if err != nil {
					log.Fatalf("failed to run compaction daemon: %v", err)
				}
			})

			if err != nil {
				log.Fatalf("failed to create cron job: %v", err)
			}

			// start the scheduler
			s.StartAsync()

			err = launcher.debugApp.Run()
			if err != nil {
				log.Fatalf("failed to run debug app: %v", err)
			}

			s.Stop()

			log.Infof("Compaction daemon stopped")
		},
	}

	return compactionDaemonCommand
}
