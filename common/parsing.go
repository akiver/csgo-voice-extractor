package common

import (
	"fmt"
	"os"
	"regexp"

	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
)

type ExtractOptions struct {
	DemoPath   string
	DemoName   string
	File       *os.File
	OutputPath string
}

func GetPlayerID(parser dem.Parser, steamID uint64) string {
	playerName := ""
	for _, player := range parser.GameState().Participants().All() {
		if player.SteamID64 == steamID {
			var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)
			playerName = nonAlphanumericRegex.ReplaceAllString(player.Name, "")
			break
		}
	}

	if playerName == "" {
		fmt.Println("Unable to find player's name with SteamID", steamID)
		return ""
	}

	return fmt.Sprintf("%s_%d", playerName, steamID)
}
