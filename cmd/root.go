package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/background"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/spf13/cobra"
)

func CreateRootCommand(launcher *Launcher) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:           "pbuf-registry",
		Short:         "Default command to launch main application",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if launcher.buildMainApp == nil {
				return fmt.Errorf("main application is not configured")
			}
			app, err := launcher.buildMainApp()
			if err != nil {
				return err
			}
			return app.Run()
		},
	}

	rootCommand.AddCommand(CreateCompactionDaemon(launcher))
	rootCommand.AddCommand(CreateProtoParsingDaemon(launcher))

	return rootCommand
}

func CreateCompactionDaemon(launcher *Launcher) *cobra.Command {
	return newDaemonCommand(
		"compaction",
		"Run compaction daemon",
		launcher,
		func() (background.Daemon, error) { return launcher.buildCompactionDaemon() },
		func(cfg *config.Config) string { return cfg.Daemons.Compaction.CronSchedule },
	)
}

func CreateProtoParsingDaemon(launcher *Launcher) *cobra.Command {
	return newDaemonCommand(
		"proto-parsing",
		"Run proto parsing daemon",
		launcher,
		func() (background.Daemon, error) { return launcher.buildProtoParsingDaemon() },
		func(cfg *config.Config) string { return cfg.Daemons.ProtoParsing.CronSchedule },
	)
}

func newDaemonCommand(
	use string,
	short string,
	launcher *Launcher,
	buildDaemon func() (background.Daemon, error),
	scheduleOf func(*config.Config) string,
) *cobra.Command {
	var once bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			daemon, err := buildDaemon()
			if err != nil {
				return err
			}
			baseCtx := cmd.Context()
			if baseCtx == nil {
				baseCtx = context.Background()
			}
			ctx, stop := signal.NotifyContext(baseCtx, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if once {
				return daemon.Run(ctx)
			}
			if launcher.buildDebugApp == nil {
				return fmt.Errorf("debug server is not configured")
			}
			debugApp, err := launcher.buildDebugApp()
			if err != nil {
				return err
			}
			var cronSchedule string
			if cfg := launcher.getConfig(); cfg != nil {
				cronSchedule = scheduleOf(cfg)
			}
			return runBackgroundDaemon(ctx, cronSchedule, daemon, debugApp)
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, fmt.Sprintf("Run %s once and exit", use))
	return cmd
}

func registerDaemonJob(ctx context.Context, s gocron.Scheduler, cronSchedule string, daemon background.Daemon) (gocron.Job, error) {
	return s.NewJob(
		gocron.CronJob(cronSchedule, false),
		gocron.NewTask(func() {
			if err := daemon.Run(ctx); err != nil {
				log.Errorf("failed to run %s daemon: %v", daemon.Name(), err)
			}
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
}

func runBackgroundDaemon(parentCtx context.Context, cronSchedule string, daemon background.Daemon, debugApp appRunner) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	if _, err := registerDaemonJob(ctx, s, cronSchedule, daemon); err != nil {
		_ = s.Shutdown()
		return fmt.Errorf("failed to create cron job: %w", err)
	}

	s.Start()

	runErr := debugApp.Run()
	cancel()
	if shutdownErr := s.Shutdown(); shutdownErr != nil {
		log.Errorf("failed to shutdown scheduler: %v", shutdownErr)
	}
	if runErr != nil {
		return fmt.Errorf("failed to run debug app: %w", runErr)
	}

	log.Infof("%s daemon stopped", daemon.Name())
	return nil
}
