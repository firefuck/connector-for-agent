package core

import "time"

// Protocol and resource limits are shared by the gateway, connector and bridge.
// These are safety bounds, not deployment configuration or secret credentials.
const (
	maxMessageBytes           = 12 << 20
	maxControlBodyBytes       = 64 << 10
	maxPairBodyBytes          = 4 << 10
	maxFileBytes              = 4 << 20
	maxShellOutputBytes       = 1 << 20
	maxScreenshotBytes        = 8 << 20
	maxDirectoryEntries       = 2000
	maxMCPRequests            = 256
	maxBridgeRequests         = 32
	maxBridgeNotifications    = 16
	bridgeNotificationTimeout = 10 * time.Second
	pairingLifetime           = 10 * time.Minute
	deviceOnlineWindow        = 10 * time.Second
	defaultLeaseDuration      = 5 * time.Minute
	leaseCompletionGrace      = time.Minute
	defaultActionTimeout      = 60 * time.Second
	maxActionTimeout          = 300 * time.Second
	maxAuthFailures           = 30
	authFailureWindow         = time.Minute
	maxTrackedClients         = 4096
	maxAuditBytes             = 8 << 20
)
