package system_setting

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PasskeySettings struct {
	Enabled              bool   `json:"enabled"`
	RPDisplayName        string `json:"rp_display_name"`
	RPID                 string `json:"rp_id"`
	Origins              string `json:"origins"`
	AllowInsecureOrigin  bool   `json:"allow_insecure_origin"`
	UserVerification     string `json:"user_verification"`
	AttachmentPreference string `json:"attachment_preference"`
}

var defaultPasskeySettings = PasskeySettings{
	Enabled:              false,
	RPDisplayName:        common.SystemName,
	RPID:                 "",
	Origins:              "",
	AllowInsecureOrigin:  false,
	UserVerification:     "preferred",
	AttachmentPreference: "",
}

func init() {
	config.GlobalConfig.Register("passkey", &defaultPasskeySettings)
}

// GetPasskeySettings returns effective passkey settings for the current request.
//
// It must NOT mutate defaultPasskeySettings when deriving RPID/Origins from ServerAddress:
// ServerAddress is reloaded from the DB on a timer, and writing derived fields into the global
// struct would freeze RPID/Origins at the first seen ServerAddress (breaking domain moves).
func GetPasskeySettings() *PasskeySettings {
	out := defaultPasskeySettings

	origins := strings.TrimSpace(out.Origins)
	if origins == "" || origins == "[]" {
		out.Origins = strings.TrimSpace(ServerAddress)
	}

	rpID := strings.TrimSpace(out.RPID)
	if rpID == "" {
		serverAddr := strings.TrimSpace(ServerAddress)
		if serverAddr != "" {
			if parsed, err := url.Parse(serverAddr); err == nil && parsed.Host != "" {
				rpID = parsed.Host
			} else {
				rpID = serverAddr
			}
		}
	}
	out.RPID = rpID

	return &out
}
