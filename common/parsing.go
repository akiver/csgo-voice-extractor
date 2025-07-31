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
	Mode       Mode
	SteamIDs   []string
}

type VoiceSegment struct {
	Data      []byte
	Timestamp float64 // in seconds
}

var playerNameCache = make(map[uint64]string)

func GetPlayerID(parser dem.Parser, steamID uint64) string {
	if name, ok := playerNameCache[steamID]; ok {
		return fmt.Sprintf("%s_%d", name, steamID)
	}

	playerName := ""
	for _, player := range parser.GameState().Participants().All() {
		if player.SteamID64 == steamID {
			invalidCharsRegex := regexp.MustCompile(`[\\/:*?"<>|]`)
			playerName = invalidCharsRegex.ReplaceAllString(player.Name, "")
			playerNameCache[steamID] = playerName
			break
		}
	}

	if playerName == "" {
		fmt.Println("Unable to find player's name with SteamID", steamID)
		return ""
	}

	return fmt.Sprintf("%s_%d", playerName, steamID)
}
