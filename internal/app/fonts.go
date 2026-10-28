package app

import (
	"sync"

	"github.com/BIGGASSS/FlClash-MyGo/assets"
	"github.com/egoist/mygo/ui"
)

var fontsOnce sync.Once
var fontsError error

func registerFonts() error {
	fontsOnce.Do(func() {
		for _, font := range [][2]string{{"Roboto-Regular.ttf", "Roboto"}, {"Roboto-Medium.ttf", "Roboto"}, {"Roboto-Bold.ttf", "Roboto"}, {"JetBrainsMono-Regular.ttf", "JetBrains Mono"}, {"Twemoji.Mozilla.ttf", "Twemoji"}} {
			if err := ui.RegisterFont(assets.Read("fonts/"+font[0]), font[1]); err != nil {
				fontsError = err
				return
			}
		}
	})
	return fontsError
}
