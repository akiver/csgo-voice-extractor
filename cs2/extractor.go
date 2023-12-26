package cs2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akiver/csgo-voice-extractor/common"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msgs2"
)

func Extract(options common.ExtractOptions) {
	common.AssertLibraryFilesExist()

	var voiceDataPerPlayer = map[string][][]byte{}
	parser := dem.NewParser(options.File)
	defer parser.Close()

	// The message CSVCMsg_VoiceInit is present with CS2 demos but the codec is always vaudio_speex which is wrong.
	// We don't check for unsupported codecs as we do for CS:GO demos.
	parser.RegisterNetMessageHandler(func(m *msgs2.CSVCMsg_VoiceData) {
		playerID := common.GetPlayerID(parser, m.GetXuid())
		if playerID == "" {
			return
		}

		if voiceDataPerPlayer[playerID] == nil {
			voiceDataPerPlayer[playerID] = make([][]byte, 0)
		}

		voiceDataPerPlayer[playerID] = append(voiceDataPerPlayer[playerID], m.Audio.VoiceData)
	})

	err := parser.ParseToEnd()

	demoPath := options.DemoPath
	isCorruptedDemo := errors.Is(err, dem.ErrUnexpectedEndOfDemo)
	if err != nil && !isCorruptedDemo {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
			Err:      err,
			ExitCode: common.ParsingError,
		})
	}

	if len(voiceDataPerPlayer) == 0 {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("No voice data found in demo %s\n", demoPath),
			ExitCode: common.NoVoiceDataFound,
		})
		return
	}

	demoName := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	for playerID, voiceData := range voiceDataPerPlayer {
		playerFileName := fmt.Sprintf("%s_%s", demoName, playerID)
		wavFilePath := fmt.Sprintf("%s/%s.wav", options.OutputPath, playerFileName)
		convertAudioDataToWavFiles(voiceData, wavFilePath)
	}
}

func convertAudioDataToWavFiles(payloads [][]byte, fileName string) {
	// This sample rate can be set using data from the VoiceData net message.
	// But every demo processed has used 24000 and is single channel.
	voiceDecoder, err := NewOpusDecoder(24000, 1)

	if err != nil {
		common.HandleError(common.Error{
			Message:  "Failed to create Opus decoder",
			Err:      err,
			ExitCode: common.DecodingError,
		})
		return
	}

	o := make([]int, 0, 1024)

	for _, payload := range payloads {
		c, err := DecodeChunk(payload)

		if err != nil {
			fmt.Println(err)
		}

		// Not silent frame
		if c != nil && len(c.Data) > 0 {
			pcm, err := voiceDecoder.Decode(c.Data)

			if err != nil {
				common.HandleError(common.Error{
					Message:  "Failed to decode voice data",
					Err:      err,
					ExitCode: common.DecodingError,
				})
			}

			converted := make([]int, len(pcm))
			for i, v := range pcm {
				// Float32 buffer implementation is wrong in go-audio, so we have to convert to int before encoding
				converted[i] = int(v * 2147483647)
			}

			o = append(o, converted...)
		}
	}

	outFile, err := os.Create(fileName)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't create WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
	}
	defer outFile.Close()

	// Encode new wav file, from decoded opus data.
	enc := wav.NewEncoder(outFile, 24000, 32, 1, 1)
	defer enc.Close()

	buf := &audio.IntBuffer{
		Data: o,
		Format: &audio.Format{
			SampleRate:  24000,
			NumChannels: 1,
		},
	}

	if err := enc.Write(buf); err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't write WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
	}
}
