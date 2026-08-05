package service

import (
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func resolveOpsNodeIdentity(cfg *config.Config) *OpsNodeIdentity {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown"
	}

	nodeID := hostname
	region := ""
	if cfg != nil {
		if configured := strings.TrimSpace(cfg.Ops.NodeID); configured != "" {
			nodeID = configured
		}
		region = strings.TrimSpace(cfg.Ops.Region)
	}

	return &OpsNodeIdentity{
		NodeID:   nodeID,
		Region:   region,
		Hostname: hostname,
	}
}
