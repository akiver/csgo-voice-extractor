package cs2

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/akiver/csgo-voice-extractor/common"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msgs2"
	"gopkg.in/hraban/opus.v2"
)

const (
	// The samples rate could be retrieved from the VoiceData net message but it's always 48000 for Opus and 24000 for
	// Steam Voice and is single channel.
	opusSampleRate  = 48000
	steamSampleRate = 24000
)

func buildPlayerWavFileName(outputPath string, demoName string, playerID string) string {
	fileName := fmt.Sprintf("%s_%s.wav", demoName, playerID)

	return filepath.Join(outputPath, fileName)
}

func getFormatSampleRate(format msgs2.VoiceDataFormatT) int {
	if format == msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS {
		return opusSampleRate
	}

	return steamSampleRate
}

func samplesToInt32(samples []float32) []int {
	ints := make([]int, len(samples))
	for i, v := range samples {
		ints[i] = int(v * math.MaxInt32)
	}

	return ints
}

func writeAudioToWav(enc *wav.Encoder, sampleRate int, data []int) error {
	buf := &audio.IntBuffer{
		Data: data,
		Format: &audio.Format{
			SampleRate:  sampleRate,
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

	return nil
}

func writePCMToWav(pcmBuffer []int, sampleRate int, filePath string) {
	outFile, err := os.Create(filePath)
	if err != nil {
		common.HandleError(common.Error{
			Message:  "Couldn't write WAV file",
			Err:      err,
			ExitCode: common.WavFileCreationError,
		})
		return
	}
	defer outFile.Close()

	enc := wav.NewEncoder(outFile, sampleRate, 32, 1, 1)
	defer enc.Close()

	buf := &audio.IntBuffer{
		Data: pcmBuffer,
		Format: &audio.Format{
			SampleRate:  sampleRate,
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

func generateAudioFilesWithDemoLength(segmentsPerPlayer map[string][]common.VoiceSegment, format msgs2.VoiceDataFormatT, durationSeconds float64, demoName string, outputPath string) {
	for playerID, segments := range segmentsPerPlayer {
		wavFilePath := buildPlayerWavFileName(outputPath, demoName, playerID)
		if format == msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS {
			writeOpusVoiceSegmentsToWav(segments, wavFilePath, durationSeconds)
		} else {
			writeSteamVoiceSegmentsToWav(segments, wavFilePath, durationSeconds)
		}
	}
}

func generateAudioFilesWithCompactLength(segmentsPerPlayer map[string][]common.VoiceSegment, format msgs2.VoiceDataFormatT, demoName string, options common.ExtractOptions) {
	sampleRate := getFormatSampleRate(format)
	isOpusFormat := format == msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS

	for playerID, segments := range segmentsPerPlayer {
		if len(segments) == 0 {
			continue
		}

		wavFilePath := buildPlayerWavFileName(options.OutputPath, demoName, playerID)
		outFile, err := common.CreateWavFile(wavFilePath)
		if err != nil {
			continue
		}
		defer outFile.Close()

		enc := wav.NewEncoder(outFile, sampleRate, 32, 1, 1)
		defer enc.Close()

		if isOpusFormat {
			decoder, err := NewOpusDecoder(sampleRate, 1)
			if err != nil {
				continue
			}

			for _, segment := range segments {
				samples, err := Decode(decoder, segment.Data)
				if err != nil {
					fmt.Println(err)
					continue
				}

				if len(samples) == 0 {
					continue
				}

				pcmBuffer := samplesToInt32(samples)
				err = writeAudioToWav(enc, sampleRate, pcmBuffer)
				if err != nil {
					continue
				}
			}
		} else {
			decoder, err := NewSteamDecoder(sampleRate, 1)
			if err != nil {
				continue
			}

			for _, segment := range segments {
				chunk, err := DecodeChunk(segment.Data)
				if err != nil || chunk == nil || len(chunk.Data) == 0 {
					continue
				}

				samples, err := decoder.Decode(chunk.Data)
				if err != nil {
					fmt.Println(err)
					continue
				}

				if len(samples) == 0 {
					continue
				}

				pcmBuffer := samplesToInt32(samples)
				writeAudioToWav(enc, sampleRate, pcmBuffer)
			}
		}
	}
}

func writeSteamVoiceSegmentsToWav(segments []common.VoiceSegment, fileName string, totalDuration float64) {
	decoder, err := NewSteamDecoder(steamSampleRate, 1)
	if err != nil {
		return
	}

	totalSamples := int(totalDuration * float64(steamSampleRate))
	pcmBuffer := make([]int, totalSamples)
	var previousEndPos int = 0

	for _, segment := range segments {
		chunk, err := DecodeChunk(segment.Data)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Not silent frame
		if chunk != nil && len(chunk.Data) > 0 {
			samples, err := decoder.Decode(chunk.Data)

			if err != nil {
				common.HandleError(common.Error{
					Message:  "Failed to decode voice data",
					Err:      err,
					ExitCode: common.DecodingError,
				})
			}

			startPos := int(segment.Timestamp * float64(steamSampleRate))
			if startPos < previousEndPos {
				startPos = previousEndPos
			}

			if startPos >= totalSamples {
				fmt.Printf("Warning: Voice segment at %f seconds exceeds demo duration\n", segment.Timestamp)
				continue
			}

			for i, sample := range samples {
				samplePos := startPos + i
				if samplePos >= totalSamples {
					break
				}
				pcmBuffer[samplePos] = int(sample * math.MaxInt32)
			}

			previousEndPos = startPos + len(samples)
		}
	}

	writePCMToWav(pcmBuffer, steamSampleRate, fileName)
}

func writeOpusVoiceSegmentsToWav(segments []common.VoiceSegment, fileName string, durationSeconds float64) {
	decoder, err := NewOpusDecoder(opusSampleRate, 1)
	if err != nil {
		return
	}

	totalSamples := int(durationSeconds * float64(opusSampleRate))
	// store samples in a sparse map to avoid large memory usage
	samplesMap := make(map[int][]float32)
	// store start positions of segments that contain audio data
	activePositions := make([]int, 0)

	var previousEndPosition = 0
	// decode and store each voice segment in the sparse map
	for _, segment := range segments {
		samples, err := Decode(decoder, segment.Data)
		if err != nil {
			fmt.Println(err)
			continue
		}

		startPosition := int(segment.Timestamp * float64(opusSampleRate))
		if startPosition < previousEndPosition {
			startPosition = previousEndPosition
		}

		if startPosition >= totalSamples {
			fmt.Printf("Warning: Voice segment at %f seconds exceeds demo duration\n", segment.Timestamp)
			continue
		}

		samplesMap[startPosition] = samples
		activePositions = append(activePositions, startPosition)
		previousEndPosition = startPosition + len(samples)
	}

	// no voice
	if len(activePositions) == 0 {
		return
	}

	outFile, err := common.CreateWavFile(fileName)
	if err != nil {
		return
	}
	defer outFile.Close()

	enc := wav.NewEncoder(outFile, opusSampleRate, 32, 1, 1)
	defer enc.Close()

	silenceBufferSize := 8192 // small buffer size for silence
	silenceBuffer := make([]int, silenceBufferSize)
	lastPosition := 0
	for _, startPosition := range activePositions {
		silenceLength := startPosition - lastPosition
		if silenceLength > 0 {
			// write silence in chunks to avoid large memory allocations
			for silenceLength > silenceBufferSize {
				err = writeAudioToWav(enc, opusSampleRate, silenceBuffer)
				if err != nil {
					return
				}
				silenceLength -= silenceBufferSize
			}

			// write remaining silence
			if silenceLength > 0 {
				err = writeAudioToWav(enc, opusSampleRate, silenceBuffer[:silenceLength])
				if err != nil {
					return
				}
			}
		}

		// write the player's voice
		samples := samplesMap[startPosition]
		pcmBuffer := samplesToInt32(samples)
		err = writeAudioToWav(enc, opusSampleRate, pcmBuffer)
		if err != nil {
			return
		}
		lastPosition = startPosition + len(samples)
	}

	if lastPosition >= totalSamples {
		return
	}

	// write remaining silence at the end of the file
	remainingSilence := totalSamples - lastPosition
	for remainingSilence > silenceBufferSize {
		err = writeAudioToWav(enc, opusSampleRate, silenceBuffer)
		if err != nil {
			return
		}
		remainingSilence -= silenceBufferSize
	}

	if remainingSilence > 0 {
		writeAudioToWav(enc, opusSampleRate, silenceBuffer[:remainingSilence])
	}
}

func generateAudioFileWithMergedVoices(voiceDataPerPlayer map[string][]common.VoiceSegment, format msgs2.VoiceDataFormatT, durationSeconds float64, demoName string, outputPath string) {
	var err error
	var opusDecoder *opus.Decoder
	var steamDecoder *SteamDecoder
	sampleRate := getFormatSampleRate(format)
	isOpusFormat := format == msgs2.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS

	if isOpusFormat {
		opusDecoder, err = NewOpusDecoder(sampleRate, 1)
		if err != nil {
			return
		}
	} else {
		steamDecoder, err = NewSteamDecoder(sampleRate, 1)
		if err != nil {
			return
		}
	}

	totalSamples := int(durationSeconds * float64(sampleRate))
	wavFilePath := filepath.Join(outputPath, demoName+".wav")
	outFile, err := common.CreateWavFile(wavFilePath)
	if err != nil {
		return
	}
	defer outFile.Close()

	enc := wav.NewEncoder(outFile, sampleRate, 32, 1, 1)
	defer enc.Close()

	type VoiceSegmentInfo struct {
		StartPosition int
		Samples       []float32
	}

	// decode and store players' voice segments
	voiceSegments := make([]VoiceSegmentInfo, 0)
	for _, segments := range voiceDataPerPlayer {
		previousEndPosition := 0
		for _, segment := range segments {
			var pcm []float32
			if isOpusFormat {
				chunk, err := Decode(opusDecoder, segment.Data)
				if err != nil {
					fmt.Println(err)
					continue
				}
				pcm = chunk
			} else {
				chunk, err := DecodeChunk(segment.Data)
				if err != nil || chunk == nil || len(chunk.Data) == 0 {
					continue
				}
				samples, err := steamDecoder.Decode(chunk.Data)
				if err != nil {
					fmt.Println(err)
					continue
				}
				pcm = samples
			}

			if len(pcm) == 0 {
				continue
			}

			startPosition := int(segment.Timestamp * float64(sampleRate))
			if startPosition < previousEndPosition {
				startPosition = previousEndPosition
			}

			if startPosition >= totalSamples {
				fmt.Printf("Warning: Voice segment at %f seconds exceeds demo duration\n", segment.Timestamp)
				continue
			}

			voiceSegments = append(voiceSegments, VoiceSegmentInfo{
				StartPosition: startPosition,
				Samples:       pcm,
			})

			previousEndPosition = startPosition + len(pcm)
		}
	}

	// process in small chunks to avoid high memory usage
	const chunkSize = 8192
	for chunkStart := 0; chunkStart < totalSamples; chunkStart += chunkSize {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > totalSamples {
			chunkEnd = totalSamples
		}

		chunkLength := chunkEnd - chunkStart
		samples := make([]float32, chunkLength)
		activeSources := make([]int, chunkLength)

		// find segments that overlap with the current chunk
		for _, segment := range voiceSegments {
			segmentEnd := segment.StartPosition + len(segment.Samples)
			// check if this segment does not overlap with the current chunk
			if segmentEnd <= chunkStart || segment.StartPosition >= chunkEnd {
				continue
			}

			// calculate the start and end of the overlap
			overlapStart := segment.StartPosition
			if overlapStart < chunkStart {
				overlapStart = chunkStart
			}
			overlapEnd := segmentEnd
			if overlapEnd > chunkEnd {
				overlapEnd = chunkEnd
			}

			// add samples in the chunk and track active sources (players talking at the same time)
			for i := overlapStart; i < overlapEnd; i++ {
				sampleIndex := i - chunkStart
				sampleStartPosition := i - segment.StartPosition

				if sampleStartPosition >= 0 && sampleStartPosition < len(segment.Samples) {
					sample := segment.Samples[sampleStartPosition]
					if sample != 0 { // ignore silence
						samples[sampleIndex] += sample
						activeSources[sampleIndex]++
					}
				}
			}
		}

		// normalize and mix samples in the chunk
		for sampleIndex := range samples {
			// ignore silence
			if samples[sampleIndex] == 0 || activeSources[sampleIndex] == 0 {
				continue
			}

			// normalize the sample if several players are talking at the same time
			if activeSources[sampleIndex] > 1 {
				mixCoeff := 1.0 / float32(math.Sqrt(float64(activeSources[sampleIndex])))
				samples[sampleIndex] *= mixCoeff
			}
		}

		// find the maximum value in the chunk to potentially normalize
		maxSampleValue := float32(1.0)
		for _, v := range samples {
			f := math.Abs(float64(v))
			if f > float64(maxSampleValue) {
				maxSampleValue = float32(f)
			}
		}

		// normalize if needed
		if maxSampleValue > 1.0 {
			for i := range samples {
				samples[i] /= maxSampleValue
			}
		}

		pcmBuffer := samplesToInt32(samples)
		err = writeAudioToWav(enc, sampleRate, pcmBuffer)
		if err != nil {
			return
		}
	}

	wavFilePath = filepath.Join(outputPath, demoName+".wav")
}

func Extract(options common.ExtractOptions) {
	common.AssertLibraryFilesExist()

	demoPath := options.DemoPath
	parserConfig := dem.DefaultParserConfig
	parser := dem.NewParserWithConfig(options.File, parserConfig)
	defer parser.Close()
	var segmentsPerPlayer = map[string][]common.VoiceSegment{}
	var format msgs2.VoiceDataFormatT

	parser.RegisterNetMessageHandler(func(m *msgs2.CSVCMsg_VoiceData) {
		steamID := m.GetXuid()
		if len(options.SteamIDs) > 0 && !slices.Contains(options.SteamIDs, fmt.Sprintf("%d", steamID)) {
			return
		}

		playerID := common.GetPlayerID(parser, steamID)
		// Opus format since the arms race update (07/02/2024), Steam Voice format before that.
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

		if segmentsPerPlayer[playerID] == nil {
			segmentsPerPlayer[playerID] = make([]common.VoiceSegment, 0)
		}

		segmentsPerPlayer[playerID] = append(segmentsPerPlayer[playerID], common.VoiceSegment{
			Data:      m.Audio.VoiceData,
			Timestamp: parser.CurrentTime().Seconds(),
		})
	})

	err := parser.ParseToEnd()

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

	if len(segmentsPerPlayer) == 0 {
		common.HandleError(common.Error{
			Message:  fmt.Sprintf("No voice data found in demo %s\n", demoPath),
			ExitCode: common.NoVoiceDataFound,
		})
		return
	}

	fmt.Println("Parsing done, generating audio files...")
	durationSeconds := parser.CurrentTime().Seconds()
	demoName := strings.TrimSuffix(filepath.Base(demoPath), filepath.Ext(demoPath))
	if options.Mode == common.ModeSingleFull {
		generateAudioFileWithMergedVoices(segmentsPerPlayer, format, durationSeconds, demoName, options.OutputPath)
	} else if options.Mode == common.ModeSplitFull {
		generateAudioFilesWithDemoLength(segmentsPerPlayer, format, durationSeconds, demoName, options.OutputPath)
	} else {
		generateAudioFilesWithCompactLength(segmentsPerPlayer, format, demoName, options)
	}
}
