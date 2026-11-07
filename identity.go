// Package relaydesk defines the compiled product identity and legal notices.
// These values are not runtime deployment settings. A source editor can still
// change them; compiled branding is not copy protection or proof of origin.
package relaydesk

import _ "embed"

const (
	Name       = "RelayDesk"
	ID         = "relaydesk"
	Version    = "0.1.0"
	Series     = "0.1"
	Repository = "https://github.com/firefuck/connector-for-agent"
	LicenseID  = "LicenseRef-RelayDesk-Proprietary"
)

//go:embed LICENSE
var licenseText string

//go:embed NOTICE
var noticeText string

func LicenseText() string { return licenseText }
func NoticeText() string  { return noticeText }
