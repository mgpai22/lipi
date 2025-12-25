package subtitle

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SRTFile struct {
	entries []Entry
}

func parseSRTFile(path string) (*SRTFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SRT file: %w", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)

	timestampRegex := regexp.MustCompile(
		`(\d{2}):(\d{2}):(\d{2}),(\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2}),(\d{3})`,
	)

	var currentEntry *Entry
	var textLines []string
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}

		if strings.TrimSpace(line) == "" {
			if currentEntry != nil && len(textLines) > 0 {
				currentEntry.Text = strings.Join(textLines, "\n")
				entries = append(entries, *currentEntry)
				currentEntry = nil
				textLines = nil
			}
			continue
		}

		if currentEntry == nil {
			index, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil {
				currentEntry = &Entry{Index: index}
				continue
			}
		}

		if currentEntry != nil && currentEntry.StartTime == 0 &&
			currentEntry.EndTime == 0 {
			matches := timestampRegex.FindStringSubmatch(line)
			if len(matches) == 9 {
				startTime, err := parseSRTTimestamp(
					matches[1], matches[2], matches[3], matches[4],
				)
				if err != nil {
					return nil, fmt.Errorf(
						"invalid start timestamp at line %d: %w",
						lineNum,
						err,
					)
				}
				endTime, err := parseSRTTimestamp(
					matches[5], matches[6], matches[7], matches[8],
				)
				if err != nil {
					return nil, fmt.Errorf(
						"invalid end timestamp at line %d: %w",
						lineNum,
						err,
					)
				}
				currentEntry.StartTime = startTime
				currentEntry.EndTime = endTime
				continue
			}
		}

		if currentEntry != nil {
			textLines = append(textLines, line)
		}
	}

	if currentEntry != nil && len(textLines) > 0 {
		currentEntry.Text = strings.Join(textLines, "\n")
		entries = append(entries, *currentEntry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SRT file: %w", err)
	}

	return &SRTFile{entries: entries}, nil
}

func parseSRTTimestamp(
	hours, minutes, seconds, millis string,
) (time.Duration, error) {
	h, err := strconv.Atoi(hours)
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(minutes)
	if err != nil {
		return 0, err
	}
	s, err := strconv.Atoi(seconds)
	if err != nil {
		return 0, err
	}
	ms, err := strconv.Atoi(millis)
	if err != nil {
		return 0, err
	}

	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(s)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}

func (f *SRTFile) Format() Format {
	return FormatSRT
}

func (f *SRTFile) Subtitle() *Subtitle {
	return &Subtitle{
		Entries: f.entries,
		Format:  string(FormatSRT),
	}
}

func (f *SRTFile) SetText(index int, text string) error {
	if index < 0 || index >= len(f.entries) {
		return fmt.Errorf(
			"index %d out of range (0-%d)",
			index,
			len(f.entries)-1,
		)
	}
	f.entries[index].Text = text
	return nil
}

func (f *SRTFile) Write(path string) error {
	writer, err := NewWriter(FormatSRT)
	if err != nil {
		return err
	}
	return writer.Write(f.Subtitle(), path)
}
