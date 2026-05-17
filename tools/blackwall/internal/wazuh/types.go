// Package wazuh is a minimal Wazuh Manager API client for blackwall.
package wazuh

import "time"

type Agent struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"`
	LastKeepAlive time.Time `json:"lastKeepAlive"`
	OS            struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"os"`
	Version       string `json:"version"`
	DateAdd       time.Time `json:"dateAdd"`
	Group         []string `json:"group"`
	Manager       string   `json:"manager"`
}

type AgentDetail struct {
	Agent
	Config map[string]string `json:"-"`
}

type agentsResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type singleAgentResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type authResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}
