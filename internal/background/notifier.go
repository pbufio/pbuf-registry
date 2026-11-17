package background

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/model"
)

const (
	webhookFormatJSON  = "json"
	webhookFormatSlack = "slack"
	maxSlackEventLines = 15
)

// DriftNotifier delivers alerts when new schema drift events are persisted.
type DriftNotifier interface {
	NotifyDriftEvents(ctx context.Context, events []model.DriftEvent) error
}

type webhookDriftNotifier struct {
	url     string
	format  string
	minRank int
	client  *http.Client
	log     *log.Helper
}

type webhookEventItem struct {
	ID           string              `json:"id"`
	Filename     string              `json:"filename"`
	EventType    model.DriftEventType `json:"event_type"`
	Severity     model.DriftSeverity  `json:"severity"`
	PreviousHash string              `json:"previous_hash,omitempty"`
	CurrentHash  string              `json:"current_hash,omitempty"`
}

type webhookPayload struct {
	ModuleName      string             `json:"module_name"`
	TagName         string             `json:"tag_name"`
	HighestSeverity string             `json:"highest_severity"`
	DetectedAt      time.Time          `json:"detected_at"`
	Events          []webhookEventItem `json:"events"`
}

// NewWebhookDriftNotifier creates a webhook/Slack notifier from config.
// Returns (nil, nil) when cfg.URL is empty.
func NewWebhookDriftNotifier(cfg config.DriftWebhookConfig, logger log.Logger) (DriftNotifier, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		return nil, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid drift webhook url: must be http or https")
	}

	minSeverity := strings.TrimSpace(cfg.MinSeverity)
	if minSeverity == "" {
		minSeverity = string(model.DriftSeverityWarning)
	}
	minRank, err := parseDriftSeverityRank(minSeverity)
	if err != nil {
		return nil, err
	}

	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	switch format {
	case "", "auto":
		host := strings.ToLower(parsedURL.Hostname())
		if host == "hooks.slack.com" || strings.HasSuffix(host, ".slack.com") {
			format = webhookFormatSlack
		} else {
			format = webhookFormatJSON
		}
	case webhookFormatJSON, webhookFormatSlack:
	default:
		return nil, fmt.Errorf("invalid drift webhook format %q (must be json or slack)", cfg.Format)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &webhookDriftNotifier{
		url:     rawURL,
		format:  format,
		minRank: minRank,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("stopped after 5 redirects")
				}
				if len(via) > 0 {
					if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
						return errors.New("refusing webhook redirect from https to non-https")
					}
					if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
						return errors.New("refusing cross-host webhook redirect")
					}
				}
				return nil
			},
		},
		log: log.NewHelper(log.With(logger, "module", "background/WebhookDriftNotifier")),
	}, nil
}

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func (n *webhookDriftNotifier) NotifyDriftEvents(ctx context.Context, events []model.DriftEvent) error {
	var groupKeys []string
	groups := make(map[string][]model.DriftEvent)
	for _, ev := range events {
		if driftSeverityRank(ev.Severity) < n.minRank {
			continue
		}
		key := ev.ModuleID + "|" + ev.ModuleName + "|" + ev.TagID + "|" + ev.TagName
		if _, exists := groups[key]; !exists {
			groupKeys = append(groupKeys, key)
		}
		groups[key] = append(groups[key], ev)
	}

	for _, key := range groupKeys {
		if err := n.sendGroup(ctx, groups[key]); err != nil {
			return err
		}
	}
	return nil
}

func (n *webhookDriftNotifier) sendGroup(ctx context.Context, matched []model.DriftEvent) error {
	var body []byte
	var err error
	if n.format == webhookFormatSlack {
		body, err = buildSlackDriftPayload(matched)
	} else {
		body, err = buildJSONDriftPayload(matched)
	}
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pbuf-registry-drift-notifier")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func buildJSONDriftPayload(events []model.DriftEvent) ([]byte, error) {
	first := events[0]
	moduleName := first.ModuleName
	if moduleName == "" {
		moduleName = first.ModuleID
	}
	tagName := first.TagName
	if tagName == "" {
		tagName = first.TagID
	}

	highest := highestSeverityLabel(events)
	items := make([]webhookEventItem, 0, len(events))
	detectedAt := first.DetectedAt
	for _, ev := range events {
		if ev.DetectedAt.After(detectedAt) {
			detectedAt = ev.DetectedAt
		}
		items = append(items, webhookEventItem{
			ID:           ev.ID,
			Filename:     ev.Filename,
			EventType:    ev.EventType,
			Severity:     ev.Severity,
			PreviousHash: ev.PreviousHash,
			CurrentHash:  ev.CurrentHash,
		})
	}

	return json.Marshal(webhookPayload{
		ModuleName:      moduleName,
		TagName:         tagName,
		HighestSeverity: highest,
		DetectedAt:      detectedAt.UTC(),
		Events:          items,
	})
}

func buildSlackDriftPayload(events []model.DriftEvent) ([]byte, error) {
	first := events[0]
	moduleName := first.ModuleName
	if moduleName == "" {
		moduleName = first.ModuleID
	}
	tagName := first.TagName
	if tagName == "" {
		tagName = first.TagID
	}

	safeModule := slackEscaper.Replace(moduleName)
	safeTag := slackEscaper.Replace(tagName)
	highest := highestSeverityLabel(events)
	summary := fmt.Sprintf("[%s] Schema drift detected in %s@%s (%d file(s))", highest, safeModule, safeTag, len(events))

	limit := len(events)
	if limit > maxSlackEventLines {
		limit = maxSlackEventLines
	}
	lines := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		ev := events[i]
		lines = append(lines, fmt.Sprintf("• *%s* `%s` (%s)", strings.ToUpper(string(ev.Severity)), slackEscaper.Replace(ev.Filename), ev.EventType))
	}
	if len(events) > maxSlackEventLines {
		lines = append(lines, fmt.Sprintf("_...and %d more file(s)_", len(events)-maxSlackEventLines))
	}

	payload := map[string]interface{}{
		"text": summary,
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("Schema Drift Alert (%s)", highest),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("*Module:*\n`%s`", safeModule)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Tag:*\n`%s`", safeTag)},
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": strings.Join(lines, "\n"),
				},
			},
		},
	}

	return json.Marshal(payload)
}

func highestSeverityLabel(events []model.DriftEvent) string {
	maxRank := 0
	for _, ev := range events {
		if r := driftSeverityRank(ev.Severity); r > maxRank {
			maxRank = r
		}
	}
	switch maxRank {
	case 3:
		return "CRITICAL"
	case 2:
		return "WARNING"
	default:
		return "INFO"
	}
}

func parseDriftSeverityRank(raw string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(model.DriftSeverityInfo):
		return 1, nil
	case string(model.DriftSeverityWarning):
		return 2, nil
	case string(model.DriftSeverityCritical):
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid drift webhook min_severity %q (must be info, warning, or critical)", raw)
	}
}

func driftSeverityRank(severity model.DriftSeverity) int {
	switch severity {
	case model.DriftSeverityCritical:
		return 3
	case model.DriftSeverityWarning:
		return 2
	case model.DriftSeverityInfo:
		return 1
	default:
		return 0
	}
}
