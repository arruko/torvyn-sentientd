package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/logging"
)

type Client struct {
	webhookURL string
}

func New(webhookURL string) *Client {
	if webhookURL == "" {
		return nil
	}
	return &Client{webhookURL: webhookURL}
}

type slackPayload struct {
	Text string `json:"text"`
}

// PostMessage sends a simple text message to Slack.
func (c *Client) PostMessage(ctx context.Context, text string) error {
	if c == nil {
		return nil
	}

	body, _ := json.Marshal(slackPayload{Text: text})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		logging.Error.Printf("Slack returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) PostRCA(ctx context.Context, inc domain.Incident, rca domain.RCA, planConf float64, humanNeeded bool) error {
	if c == nil {
		return nil
	}
	text := fmt.Sprintf(
		"*Incident* %s (%s/%s)\n*Severity*: %s\n*RCA (%.2f)*: %s\n*Human approval needed*: %v",
		inc.ID, inc.Cluster, inc.Service, inc.Severity, rca.Confidence, rca.Summary, humanNeeded,
	)

	return c.PostMessage(ctx, text)
}
