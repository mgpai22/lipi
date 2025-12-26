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

// parsed Dialogue line with all fields
type ASSDialogue struct {
	FieldsBefore    []string
	Text            string
	LeadingTags     string
	TextWithoutTags string
	OriginalLine    string
}

// parsed ASS/SSA subtitle file that preserves all metadata
type ASSFile struct {
	preEventsLines        []string
	formatLine            string
	formatColumns         []string
	textColumnIndex       int
	dialogues             []ASSDialogue
	nonDialogueEventLines []string
}

func parseASSFile(path string) (*ASSFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open ASS file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	assFile := &ASSFile{
		preEventsLines:        make([]string, 0),
		dialogues:             make([]ASSDialogue, 0),
		nonDialogueEventLines: make([]string, 0),
		textColumnIndex:       -1,
	}

	scanner := bufio.NewScanner(file)
	inEventsSection := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}

		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "[") &&
			strings.HasSuffix(trimmedLine, "]") {
			sectionName := strings.ToLower(
				strings.TrimSuffix(strings.TrimPrefix(trimmedLine, "["), "]"),
			)
			if sectionName == "events" {
				inEventsSection = true
				assFile.preEventsLines = append(assFile.preEventsLines, line)
				continue
			} else {
				if inEventsSection {
					inEventsSection = false
				}
				assFile.preEventsLines = append(assFile.preEventsLines, line)
				continue
			}
		}

		if !inEventsSection {
			assFile.preEventsLines = append(assFile.preEventsLines, line)
			continue
		}

		if strings.HasPrefix(trimmedLine, "Format:") {
			assFile.formatLine = line
			formatPart := strings.TrimPrefix(trimmedLine, "Format:")
			columns := strings.Split(formatPart, ",")
			for i, col := range columns {
				columns[i] = strings.TrimSpace(col)
			}
			assFile.formatColumns = columns
			for i, col := range columns {
				if strings.EqualFold(col, "Text") {
					assFile.textColumnIndex = i
					break
				}
			}
			if assFile.textColumnIndex == -1 {
				return nil, fmt.Errorf(
					"ASS file missing Text column in Format line",
				)
			}
			continue
		}

		if strings.HasPrefix(trimmedLine, "Dialogue:") {
			dialogue, err := assFile.parseDialogueLine(line)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse Dialogue at line %d: %w",
					lineNum,
					err,
				)
			}
			assFile.dialogues = append(assFile.dialogues, dialogue)
			continue
		}

		assFile.nonDialogueEventLines = append(
			assFile.nonDialogueEventLines,
			line,
		)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ASS file: %w", err)
	}

	if assFile.formatLine == "" {
		return nil, fmt.Errorf(
			"ASS file missing Format line in [Events] section",
		)
	}

	return assFile, nil
}

func (f *ASSFile) parseDialogueLine(line string) (ASSDialogue, error) {
	dialogue := ASSDialogue{OriginalLine: line}

	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "Dialogue:") {
		return dialogue, fmt.Errorf("not a Dialogue line")
	}
	content := strings.TrimPrefix(trimmed, "Dialogue:")
	content = strings.TrimSpace(content)

	numColumns := len(f.formatColumns)
	if numColumns == 0 {
		return dialogue, fmt.Errorf("format columns not parsed yet")
	}

	parts := splitASSFields(content, numColumns)
	if len(parts) < numColumns {
		return dialogue, fmt.Errorf(
			"expected %d fields, got %d",
			numColumns,
			len(parts),
		)
	}

	dialogue.FieldsBefore = parts[:f.textColumnIndex]
	dialogue.Text = parts[f.textColumnIndex]

	leadingTags, textWithoutTags := extractLeadingTags(dialogue.Text)
	dialogue.LeadingTags = leadingTags
	dialogue.TextWithoutTags = textWithoutTags

	return dialogue, nil
}

func splitASSFields(content string, numFields int) []string {
	if numFields <= 0 {
		return nil
	}

	parts := make([]string, 0, numFields)
	remaining := content

	for i := 0; i < numFields-1; i++ {
		idx := strings.Index(remaining, ",")
		if idx == -1 {
			parts = append(parts, remaining)
			remaining = ""
			break
		}
		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+1:]
	}

	parts = append(parts, remaining)

	return parts
}

func extractLeadingTags(text string) (string, string) {
	tagRegex := regexp.MustCompile(`^(\{[^}]*\})+`)
	match := tagRegex.FindString(text)
	if match == "" {
		return "", text
	}
	return match, text[len(match):]
}

func (f *ASSFile) Format() Format {
	return FormatASS
}

func (f *ASSFile) Subtitle() *Subtitle {
	entries := make([]Entry, len(f.dialogues))

	for i, d := range f.dialogues {
		startTime, endTime := f.parseDialogueTimes(d)
		text := strings.ReplaceAll(d.Text, "\\N", "\n")
		text = strings.ReplaceAll(text, "\\n", "\n")

		entries[i] = Entry{
			Index:     i + 1,
			StartTime: startTime,
			EndTime:   endTime,
			Text:      text,
		}
	}

	return &Subtitle{
		Entries: entries,
		Format:  string(FormatASS),
	}
}

func (f *ASSFile) parseDialogueTimes(
	d ASSDialogue,
) (time.Duration, time.Duration) {
	startIdx := -1
	endIdx := -1
	for i, col := range f.formatColumns {
		switch strings.ToLower(col) {
		case "start":
			startIdx = i
		case "end":
			endIdx = i
		}
	}

	var startTime, endTime time.Duration

	if startIdx >= 0 && startIdx < len(d.FieldsBefore) {
		startTime = parseASSTimestamp(d.FieldsBefore[startIdx])
	}

	if endIdx >= 0 && endIdx < len(d.FieldsBefore) {
		endTime = parseASSTimestamp(d.FieldsBefore[endIdx])
	}

	return startTime, endTime
}

func parseASSTimestamp(ts string) time.Duration {
	ts = strings.TrimSpace(ts)
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}

	// split seconds and centiseconds
	secParts := strings.Split(parts[2], ".")
	if len(secParts) != 2 {
		return 0
	}

	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0
	}

	centis, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(centis)*10*time.Millisecond
}

func (f *ASSFile) SetText(index int, text string) error {
	if index < 0 || index >= len(f.dialogues) {
		return fmt.Errorf(
			"index %d out of range (0-%d)",
			index,
			len(f.dialogues)-1,
		)
	}

	assText := strings.ReplaceAll(text, "\n", "\\N")
	f.dialogues[index].Text = f.dialogues[index].LeadingTags + assText
	f.dialogues[index].TextWithoutTags = assText

	return nil
}

func (f *ASSFile) SetTextWithOverlay(index int, translatedText string) error {
	if index < 0 || index >= len(f.dialogues) {
		return fmt.Errorf(
			"index %d out of range (0-%d)",
			index,
			len(f.dialogues)-1,
		)
	}

	assTranslated := strings.ReplaceAll(translatedText, "\n", "\\N")
	originalText := f.dialogues[index].TextWithoutTags
	newText := f.dialogues[index].LeadingTags + assTranslated + "\\N" + originalText

	f.dialogues[index].Text = newText

	return nil
}

func (f *ASSFile) Write(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create ASS file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	writer := bufio.NewWriter(file)

	for _, line := range f.preEventsLines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	if _, err := writer.WriteString(f.formatLine + "\n"); err != nil {
		return err
	}

	for _, d := range f.dialogues {
		line := f.buildDialogueLine(d)
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	for _, line := range f.nonDialogueEventLines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func (f *ASSFile) buildDialogueLine(d ASSDialogue) string {
	allFields := make([]string, len(f.formatColumns))
	for i, field := range d.FieldsBefore {
		if i < len(allFields) {
			allFields[i] = field
		}
	}

	allFields[f.textColumnIndex] = d.Text

	return "Dialogue: " + strings.Join(allFields, ",")
}

func (f *ASSFile) GetOriginalText(index int) (string, error) {
	if index < 0 || index >= len(f.dialogues) {
		return "", fmt.Errorf(
			"index %d out of range (0-%d)",
			index,
			len(f.dialogues)-1,
		)
	}
	return f.dialogues[index].TextWithoutTags, nil
}
