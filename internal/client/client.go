package client

import (
	"fmt"

	zepclient "github.com/getzep/zep-go/v3/client"
	"github.com/getzep/zep-go/v3/option"
	"github.com/getzep/zepctl/internal/config"
)

// Client is an alias for the Zep client.
type Client = zepclient.Client

// New creates a new Zep client using the current configuration.
func New() (*Client, error) {
	apiKey := config.GetAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured; set ZEP_API_KEY or configure a profile")
	}

	apiURL := config.GetAPIURL()

	return zepclient.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(apiURL),
	), nil
}
