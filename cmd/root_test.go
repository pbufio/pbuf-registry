package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/pbufio/pbuf-registry/internal/background"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDaemon struct {
	name  string
	runs  atomic.Int32
	runFn func() error
}

func (f *fakeDaemon) Name() string {
	return f.name
}

func (f *fakeDaemon) Run() error {
	f.runs.Add(1)
	if f.runFn != nil {
		return f.runFn()
	}
	return nil
}

type fakeApp struct {
	runs  int
	runFn func() error
}

func (f *fakeApp) Run() error {
	f.runs++
	if f.runFn != nil {
		return f.runFn()
	}
	return nil
}

func TestCreateRootCommand_OnceAndLazyInit(t *testing.T) {
	compaction := &fakeDaemon{name: "compaction"}
	protoParsing := &fakeDaemon{name: "proto parsing"}

	var mainBuilt, debugBuilt int
	launcher := &Launcher{
		buildMainApp: func() (appRunner, error) {
			mainBuilt++
			return &fakeApp{}, nil
		},
		buildDebugApp: func() (appRunner, error) {
			debugBuilt++
			return &fakeApp{}, nil
		},
		buildCompactionDaemon: func() (background.Daemon, error) {
			return compaction, nil
		},
		buildProtoParsingDaemon: func() (background.Daemon, error) {
			return protoParsing, nil
		},
	}

	// --once must not construct mainApp (gRPC/HTTP/TLS) or debugApp (:8082)
	rootCmd := CreateRootCommand(launcher)
	rootCmd.SetArgs([]string{"compaction", "--once"})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, int32(1), compaction.runs.Load())
	assert.Equal(t, int32(0), protoParsing.runs.Load())
	assert.Equal(t, 0, mainBuilt)
	assert.Equal(t, 0, debugBuilt)

	rootCmd2 := CreateRootCommand(launcher)
	rootCmd2.SetArgs([]string{"proto-parsing", "--once"})
	require.NoError(t, rootCmd2.Execute())
	assert.Equal(t, int32(1), compaction.runs.Load())
	assert.Equal(t, int32(1), protoParsing.runs.Load())
	assert.Equal(t, 0, mainBuilt)
	assert.Equal(t, 0, debugBuilt)

	// Cron mode constructs debugApp and never constructs mainApp
	cfg := &config.Config{}
	cfg.Daemons.Compaction.CronSchedule = "0 * * * *"
	launcher.config = cfg

	rootCmd3 := CreateRootCommand(launcher)
	rootCmd3.SetArgs([]string{"compaction"})
	require.NoError(t, rootCmd3.Execute())
	assert.Equal(t, 0, mainBuilt)
	assert.Equal(t, 1, debugBuilt)
}

func TestRegisterDaemonJob_TransientErrorAndSingleton(t *testing.T) {
	t.Run("survives transient errors across runs", func(t *testing.T) {
		s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
		require.NoError(t, err)
		defer func() { _ = s.Shutdown() }()

		var failNext atomic.Bool
		failNext.Store(true)
		doneCh := make(chan struct{}, 2)

		daemon := &fakeDaemon{
			name: "proto parsing",
			runFn: func() error {
				defer func() { doneCh <- struct{}{} }()
				if failNext.Swap(false) {
					return errors.New("transient postgres timeout")
				}
				return nil
			},
		}

		job, err := registerDaemonJob(s, "0 * * * *", daemon)
		require.NoError(t, err)
		s.Start()

		require.NoError(t, job.RunNow())
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for first job run")
		}

		require.NoError(t, job.RunNow())
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for second job run")
		}

		assert.Equal(t, int32(2), daemon.runs.Load())
	})

	t.Run("singleton mode prevents overlapping daemon runs", func(t *testing.T) {
		s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
		require.NoError(t, err)
		defer func() { _ = s.Shutdown() }()

		started := make(chan struct{}, 4)
		release := make(chan struct{})
		var inFlight, maxInFlight atomic.Int32

		daemon := &fakeDaemon{
			name: "compaction",
			runFn: func() error {
				cur := inFlight.Add(1)
				for {
					prev := maxInFlight.Load()
					if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
						break
					}
				}
				started <- struct{}{}
				<-release
				inFlight.Add(-1)
				return nil
			},
		}

		job, err := registerDaemonJob(s, "0 * * * *", daemon)
		require.NoError(t, err)
		s.Start()

		require.NoError(t, job.RunNow())
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for first run to start")
		}

		// Trigger a second execution while the first is still running and verify no overlap starts
		require.NoError(t, job.RunNow())
		select {
		case <-started:
			t.Fatal("overlapping daemon run started despite singleton mode")
		case <-time.After(50 * time.Millisecond):
		}
		assert.Equal(t, int32(1), inFlight.Load())
		assert.Equal(t, int32(1), maxInFlight.Load())

		close(release)
	})
}

func TestRunBackgroundDaemon_InvalidCronReturnsError(t *testing.T) {
	err := runBackgroundDaemon("not-a-cron", &fakeDaemon{name: "compaction"}, &fakeApp{})
	require.ErrorContains(t, err, "failed to create cron job")
}
