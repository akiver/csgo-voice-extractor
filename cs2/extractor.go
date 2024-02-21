package cs2

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/akiver/csgo-voice-extractor/common"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msgs2"
	"google.golang.org/protobuf/proto"
)

// Opus format since the arms race update (07/02/2024), Steam format before that.
var format msgs2.VoiceDataFormatT

func GetDemoNetworkProtocol(demoPath string, file *os.File) (int32, error) {
	br := NewLargeBitReader(file)
	br.ReadBytes(8) // Some data
	msgType := br.ReadVarInt32()
	// The first proto message should always be EDemoCommands.DEM_FileHeader
	if msgType != 1 {
		return 0, errors.New("First message is not DEM_FileHeader")
	}

	br.ReadVarInt32() // tick
	size := br.ReadVarInt32()
	bytes := br.ReadBytes(int(size))
	var msg msgs2.CDemoFileHeader
	err := proto.Unmarshal(bytes, &msg)
	if err != nil {
		return 0, errors.New("Failed to parse CDemoFileHeader")
	}

	return msg.GetNetworkProtocol(), nil
}

//go:embed event-list-dump/*.bin
var eventListFolder embed.FS

func getGameEventListBinForProtocol(networkProtocol int32) ([]byte, error) {
	switch {
	case networkProtocol < 13992:
		return eventListFolder.ReadFile("event-list-dump/s2_CMsgSource1LegacyGameEventList.bin")
	default:
		return eventListFolder.ReadFile("event-list-dump/s2_CMsgSource1LegacyGameEventList_13992.bin")
	}
}

func Extract(options common.ExtractOptions) {
	common.AssertLibraryFilesExist()

	demoPath := options.DemoPath
	parserConfig := dem.DefaultParserConfig
	gameEventListBin, err := getGameEventListBinForProtocol(options.NetworkProtocol)
	if err != nil {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
			Err:      err,
			ExitCode: common.ParsingError,
		})
		return
	}
	parserConfig.Source2FallbackGameEventListBin = gameEventListBin

	parser := dem.NewParserWithConfig(options.File, parserConfig)
	defer parser.Close()
	var voiceDataPerPlayer = map[string][][]byte{}

	parser.RegisterNetMessageHandler(func(m *msgs2.CSVCMsg_VoiceData) {
		playerID := common.GetPlayerID(parser, m.GetXuid())
		format = m.GetAudio().GetFormat()

		if format != msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_STEAM && format != msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS {
			common.UnsupportedCodecError = &common.UnsupportedCodec{
				Name: format.String(),
			}
			parser.Cancel()
			return
		}

		if playerID == "" {
			return
		}

		if voiceDataPerPlayer[playerID] == nil {
			voiceDataPerPlayer[playerID] = make([][]byte, 0)
		}

		voiceDataPerPlayer[playerID] = append(voiceDataPerPlayer[playerID], m.Audio.VoiceData)
	})

	err = parser.ParseToEnd()

	isCorruptedDemo := errors.Is(err, dem.ErrUnexpectedEndOfDemo)
	isCanceled := errors.Is(err, dem.ErrCancelled)
	if err != nil && !isCorruptedDemo && !isCanceled {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("Failed to parse demo: %s\n", demoPath),
			Err:      err,
			ExitCode: common.ParsingError,
		})
		return
	}

	if isCanceled {
		return
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
		if format == msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS {
			convertOpusAudioDataToWavFiles(voiceData, wavFilePath)
		} else {
			convertAudioDataToWavFiles(voiceData, wavFilePath)
		}
	}
}

func convertAudioDataToWavFiles(payloads [][]byte, fileName string) {
	// This sample rate can be set using data from the VoiceData net message.
	// But every demo processed has used 24000 and is single channel.
	voiceDecoder, err := NewSteamDecoder(24000, 1)

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

func convertOpusAudioDataToWavFiles(data [][]byte, fileName string) {
	decoder, err := NewOpusDecoder(48000, 1)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Failed to create Opus decoder",
			Err:      err,
			ExitCode: common.DecodingError,
		})
		return
	}

	var pcmBuffer []int

	for _, d := range data {
		pcm, err := Decode(decoder, d)
		if err != nil {
			log.Println(err)
			continue
		}

		pp := make([]int, len(pcm))

		for i, p := range pcm {
			pp[i] = int(p * 2147483647)
		}

		pcmBuffer = append(pcmBuffer, pp...)
	}

	file, err := os.Create(fileName)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't create WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
		return
	}
	defer file.Close()

	enc := wav.NewEncoder(file, 48000, 32, 1, 1)
	defer enc.Close()

	buffer := &audio.IntBuffer{
		Data: pcmBuffer,
		Format: &audio.Format{
			SampleRate:  48000,
			NumChannels: 1,
		},
	}

	if err := enc.Write(buffer); err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't write WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
	}
}
