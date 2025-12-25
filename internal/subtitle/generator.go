package subtitle

import (
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultGenerator implements the Generator interface
type DefaultGenerator struct {
	MaxCharsPerLine int
	MaxLinesPerSub  int
	MinDuration     time.Duration
	MaxDuration     time.Duration
}

func NewDefaultGenerator() *DefaultGenerator {
	return &DefaultGenerator{
		MaxCharsPerLine: 42, // Standard subtitle line length
		MaxLinesPerSub:  2,  // Most players support 2 lines
		MinDuration:     time.Second,
		MaxDuration:     7 * time.Second,
	}
}

// converts transcription segments to subtitle
func (g *DefaultGenerator) Generate(segments []Segment) (*Subtitle, error) {
	if len(segments) == 0 {
		return &Subtitle{
			Entries: []Entry{},
			Format:  string(FormatSRT),
		}, nil
	}

	var entries []Entry
	index := 1

	for _, seg := range segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}

		if g.needsSplit(text, seg.EndTime-seg.StartTime) {
			splitEntries := g.splitSegment(seg, index)
			entries = append(entries, splitEntries...)
			index += len(splitEntries)
		} else {
			entries = append(entries, Entry{
				Index:     index,
				StartTime: seg.StartTime,
				EndTime:   seg.EndTime,
				Text:      g.formatText(text),
			})
			index++
		}
	}

	return &Subtitle{
		Entries: entries,
		Format:  string(FormatSRT),
	}, nil
}

func (g *DefaultGenerator) needsSplit(
	text string,
	duration time.Duration,
) bool {
	// if text is too long, split
	if utf8.RuneCountInString(text) > g.MaxCharsPerLine*g.MaxLinesPerSub {
		return true
	}

	// if duration is too long, split
	if duration > g.MaxDuration {
		return true
	}

	return false
}

// splits long segment into multiple entries
func (g *DefaultGenerator) splitSegment(seg Segment, startIndex int) []Entry {
	text := strings.TrimSpace(seg.Text)
	words := strings.Fields(text)
	totalDuration := seg.EndTime - seg.StartTime

	if len(words) == 0 {
		return nil
	}

	// approximate characters per subtitle
	maxChars := g.MaxCharsPerLine * g.MaxLinesPerSub
	totalChars := utf8.RuneCountInString(text)

	// estimate of splits needed
	numSplits := (totalChars + maxChars - 1) / maxChars
	if numSplits < 1 {
		numSplits = 1
	}

	durationSplits := int(totalDuration/g.MaxDuration) + 1
	if durationSplits > numSplits {
		numSplits = durationSplits
	}

	// distribute words across splits
	wordsPerSplit := (len(words) + numSplits - 1) / numSplits
	durationPerSplit := totalDuration / time.Duration(numSplits)

	var entries []Entry
	currentStart := seg.StartTime

	for i := 0; i < numSplits && len(words) > 0; i++ {
		// take words for this split
		endIdx := wordsPerSplit
		if endIdx > len(words) {
			endIdx = len(words)
		}

		splitWords := words[:endIdx]
		words = words[endIdx:]

		splitText := strings.Join(splitWords, " ")
		currentEnd := currentStart + durationPerSplit

		// Last split should end at the original end time
		if len(words) == 0 {
			currentEnd = seg.EndTime
		}

		entries = append(entries, Entry{
			Index:     startIndex + i,
			StartTime: currentStart,
			EndTime:   currentEnd,
			Text:      g.formatText(splitText),
		})

		currentStart = currentEnd
	}

	return entries
}

// formatText formats text for display with line wrapping
func (g *DefaultGenerator) formatText(text string) string {
	text = strings.TrimSpace(text)
	runeCount := utf8.RuneCountInString(text)

	// if text fits on one line, return as is
	if runeCount <= g.MaxCharsPerLine {
		return text
	}

	// try to split into two lines at a natural break point
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}

	// find the best split point (closest to middle)
	middle := runeCount / 2
	bestSplit := 0
	bestDiff := runeCount

	currentLen := 0
	for i, word := range words[:len(words)-1] {
		currentLen += utf8.RuneCountInString(word)
		if i > 0 {
			currentLen++ // space
		}

		diff := abs(currentLen - middle)
		if diff < bestDiff {
			bestDiff = diff
			bestSplit = i + 1
		}
	}

	if bestSplit > 0 && bestSplit < len(words) {
		line1 := strings.Join(words[:bestSplit], " ")
		line2 := strings.Join(words[bestSplit:], " ")
		return line1 + "\n" + line2
	}

	return text
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
