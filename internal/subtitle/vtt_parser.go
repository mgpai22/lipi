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

type VTTFile struct {
	entries []Entry
}

func parseVTTFile(path string) (*VTTFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open VTT file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var entries []Entry
	scanner := bufio.NewScanner(file)

	timestampRegex := regexp.MustCompile(
		`(\d{2}):(\d{2}):(\d{2})\.(\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})\.(\d{3})`,
	)
	shortTimestampRegex := regexp.MustCompile(
		`(\d{2}):(\d{2})\.(\d{3})\s*-->\s*(\d{2}):(\d{2})\.(\d{3})`,
	)

	var currentEntry *Entry
	var textLines []string
	lineNum := 0
	headerParsed := false
	entryIndex := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}

		if !headerParsed {
			if strings.HasPrefix(strings.TrimSpace(line), "WEBVTT") {
				headerParsed = true
				continue
			}
		}

		if strings.HasPrefix(strings.TrimSpace(line), "NOTE") {
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					break
				}
			}
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "STYLE") {
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					break
				}
			}
			continue
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

		matches := timestampRegex.FindStringSubmatch(line)
		if len(matches) == 9 {
			if currentEntry != nil && len(textLines) > 0 {
				currentEntry.Text = strings.Join(textLines, "\n")
				entries = append(entries, *currentEntry)
				textLines = nil
			}

			startTime, err := parseVTTTimestamp(
				matches[1], matches[2], matches[3], matches[4],
			)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid start timestamp at line %d: %w",
					lineNum,
					err,
				)
			}
			endTime, err := parseVTTTimestamp(
				matches[5], matches[6], matches[7], matches[8],
			)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid end timestamp at line %d: %w",
					lineNum,
					err,
				)
			}

			entryIndex++
			currentEntry = &Entry{
				Index:     entryIndex,
				StartTime: startTime,
				EndTime:   endTime,
			}
			continue
		}

		shortMatches := shortTimestampRegex.FindStringSubmatch(line)
		if len(shortMatches) == 7 {
			if currentEntry != nil && len(textLines) > 0 {
				currentEntry.Text = strings.Join(textLines, "\n")
				entries = append(entries, *currentEntry)
				textLines = nil
			}

			startTime, err := parseVTTTimestamp(
				"00", shortMatches[1], shortMatches[2], shortMatches[3],
			)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid start timestamp at line %d: %w",
					lineNum,
					err,
				)
			}
			endTime, err := parseVTTTimestamp(
				"00", shortMatches[4], shortMatches[5], shortMatches[6],
			)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid end timestamp at line %d: %w",
					lineNum,
					err,
				)
			}

			entryIndex++
			currentEntry = &Entry{
				Index:     entryIndex,
				StartTime: startTime,
				EndTime:   endTime,
			}
			continue
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
		return nil, fmt.Errorf("error reading VTT file: %w", err)
	}

	return &VTTFile{entries: entries}, nil
}

func parseVTTTimestamp(
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

func (f *VTTFile) Format() Format {
	return FormatVTT
}

func (f *VTTFile) Subtitle() *Subtitle {
	return &Subtitle{
		Entries: f.entries,
		Format:  string(FormatVTT),
	}
}

func (f *VTTFile) SetText(index int, text string) error {
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

func (f *VTTFile) Write(path string) error {
	writer, err := NewWriter(FormatVTT)
	if err != nil {
		return err
	}
	return writer.Write(f.Subtitle(), path)
}
