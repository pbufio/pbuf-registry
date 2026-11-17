package background

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebhookDriftNotifier_Validation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.DriftWebhookConfig
		wantNil   bool
		wantErrSub string
	}{
		{
			name:    "empty url returns nil notifier",
			cfg:     config.DriftWebhookConfig{},
			wantNil: true,
		},
		{
			name:       "rejects non-http scheme",
			cfg:        config.DriftWebhookConfig{URL: "file:///etc/passwd"},
			wantErrSub: "invalid drift webhook url",
		},
		{
			name:       "rejects invalid min_severity",
			cfg:        config.DriftWebhookConfig{URL: "https://example.com/hook", MinSeverity: "fatal"},
			wantErrSub: "invalid drift webhook min_severity",
		},
		{
			name:       "rejects invalid format",
			cfg:        config.DriftWebhookConfig{URL: "https://example.com/hook", Format: "xml"},
			wantErrSub: "invalid drift webhook format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewWebhookDriftNotifier(tt.cfg, log.NewStdLogger(io.Discard))
			if tt.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSub)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, notifier)
			} else {
				assert.NotNil(t, notifier)
			}
		})
	}
}

func TestWebhookDriftNotifier_FormatsAndSeverityFilter(t *testing.T) {
	detectedAt := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	sampleEvents := []model.DriftEvent{
		{
			ID:         "ev-info",
			ModuleName: "acme/payments",
			TagName:    "v2.0.0",
			Filename:   "api/v1/common.proto",
			EventType:  model.DriftEventTypeAdded,
			Severity:   model.DriftSeverityInfo,
			DetectedAt: detectedAt,
		},
		{
			ID:           "ev-crit",
			ModuleName:   "acme/payments",
			TagName:      "v2.0.0",
			Filename:     "api/v1/payment.proto",
			EventType:    model.DriftEventTypeModified,
			Severity:     model.DriftSeverityCritical,
			PreviousHash: "aaa",
			CurrentHash:  "bbb",
			DetectedAt:   detectedAt,
		},
	}

	t.Run("generic json filters below min_severity", func(t *testing.T) {
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			gotBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		notifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         srv.URL,
			MinSeverity: "WARNING",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)

		require.NoError(t, notifier.NotifyDriftEvents(context.Background(), sampleEvents))

		var payload webhookPayload
		require.NoError(t, json.Unmarshal(gotBody, &payload))
		assert.Equal(t, "acme/payments", payload.ModuleName)
		assert.Equal(t, "v2.0.0", payload.TagName)
		assert.Equal(t, "CRITICAL", payload.HighestSeverity)
		require.Len(t, payload.Events, 1)
		assert.Equal(t, "api/v1/payment.proto", payload.Events[0].Filename)
	})

	t.Run("slack block kit payload escapes control characters", func(t *testing.T) {
		var rawBody string
		var got map[string]interface{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			rawBody = string(b)
			require.NoError(t, json.Unmarshal(b, &got))
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		notifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         srv.URL,
			MinSeverity: "CRITICAL",
			Format:      "slack",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)

		eventsWithMarkup := []model.DriftEvent{
			{
				ID:         "ev-inject",
				ModuleName: "acme/payments",
				TagName:    "v2.0.0",
				Filename:   "<!channel>&test.proto",
				EventType:  model.DriftEventTypeDeleted,
				Severity:   model.DriftSeverityCritical,
				DetectedAt: detectedAt,
			},
		}
		require.NoError(t, notifier.NotifyDriftEvents(context.Background(), eventsWithMarkup))
		assert.Contains(t, got["text"], "[CRITICAL] Schema drift detected in acme/payments@v2.0.0")
		assert.NotContains(t, rawBody, "<!channel>")
		blocks, ok := got["blocks"].([]interface{})
		require.True(t, ok)
		require.Len(t, blocks, 3)
		sectionText := blocks[2].(map[string]interface{})["text"].(map[string]interface{})["text"].(string)
		assert.Contains(t, sectionText, "&lt;!channel&gt;&amp;test.proto")
	})

	t.Run("skips HTTP call when no events meet min_severity", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		notifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         srv.URL,
			MinSeverity: "CRITICAL",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)

		require.NoError(t, notifier.NotifyDriftEvents(context.Background(), sampleEvents[:1]))
		assert.Equal(t, int32(0), calls.Load())
	})

	t.Run("returns error on non-2xx status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		notifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         srv.URL,
			MinSeverity: "INFO",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)

		err = notifier.NotifyDriftEvents(context.Background(), sampleEvents)
		require.ErrorContains(t, err, "webhook returned status 502")
	})

	t.Run("rejects cross-host redirects while allowing same-host path redirects", func(t *testing.T) {
		var targetHits atomic.Int32
		evilTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			targetHits.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer evilTarget.Close()

		crossHostRedirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, evilTarget.URL+"/steal", http.StatusTemporaryRedirect)
		}))
		defer crossHostRedirector.Close()

		notifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         crossHostRedirector.URL,
			MinSeverity: "INFO",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)

		err = notifier.NotifyDriftEvents(context.Background(), sampleEvents)
		require.ErrorContains(t, err, "refusing cross-host webhook redirect")
		assert.Equal(t, int32(0), targetHits.Load())

		var sameHostHits atomic.Int32
		sameHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/old" {
				http.Redirect(w, r, "/new", http.StatusTemporaryRedirect)
				return
			}
			sameHostHits.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer sameHost.Close()

		sameHostNotifier, err := NewWebhookDriftNotifier(config.DriftWebhookConfig{
			URL:         sameHost.URL + "/old",
			MinSeverity: "INFO",
		}, log.NewStdLogger(io.Discard))
		require.NoError(t, err)
		require.NoError(t, sameHostNotifier.NotifyDriftEvents(context.Background(), sampleEvents))
		assert.Equal(t, int32(1), sameHostHits.Load())
	})
}
