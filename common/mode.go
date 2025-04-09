package common

type Mode string

const (
	ModeSplitCompact Mode = "split-compact" // 1 wav file per player that contains all of the player's voice lines in one continuous sequence without silence
	ModeSplitFull    Mode = "split-full"    // 1 wav file per player that contains all of the player's voice lines in one continuous sequence with silence (demo length)
	ModeSingleFull   Mode = "single-full"   // Single wav file that contains all of the voice lines from all players in one continuous sequence with silence (demo length)
)
