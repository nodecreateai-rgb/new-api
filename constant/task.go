package constant

import (
	"database/sql/driver"
	"fmt"
)

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
	TaskPlatformImage                   = "image"
	TaskPlatformPay                     = "pay"
)

func (p TaskPlatform) Value() (driver.Value, error) {
	return string(p), nil
}

func (p *TaskPlatform) Scan(value interface{}) error {
	if value == nil {
		*p = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*p = TaskPlatform(v)
	case []byte:
		*p = TaskPlatform(v)
	default:
		return fmt.Errorf("unsupported TaskPlatform type %T", value)
	}
	return nil
}

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"
	PayActionPay     = "pay"

	TaskActionGenerate          = "generate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
