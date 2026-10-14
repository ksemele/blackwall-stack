package wazuh

import (
	"net/http"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/httpclient"
)

func buildHTTPClient(t config.TLS) (*http.Client, error) { return httpclient.New(t) }
