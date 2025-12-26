package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgpai22/lipi/internal/subtitle"
	"github.com/mgpai22/lipi/internal/translate"
	"github.com/spf13/cobra"
)

var translateCmd = &cobra.Command{
	Use:   "translate [subtitle_file]",
	Short: "Translate subtitles to another language using AI",
	Long: `Translate an existing subtitle file to another language using AI.

Supports SRT, VTT, and ASS/SSA formats. For ASS files, all styling and 
formatting is preserved - only the dialogue text is translated.

The --overlay flag creates bilingual subtitles with the translated text
first, followed by the original text on the next line.

Examples:
  lipi translate video.srt --target-language japanese
  lipi translate video.ass --target-language ja --overlay
  lipi translate video.vtt -l english --target-language spanish -o translated.vtt`,
	Args: cobra.ExactArgs(1),
	RunE: runTranslate,
}

func init() {
	rootCmd.AddCommand(translateCmd)

	translateCmd.Flags().
		StringP("target-language", "t", "", "Target language for translation (required)")
	translateCmd.Flags().
		Bool("overlay", false, "Overlay translated text with original (bilingual subtitles)")
	translateCmd.Flags().
		StringP("api-key", "k", "", "API key (or set GEMINI_API_KEY/OPENAI_API_KEY env var)")
	translateCmd.Flags().
		String("model", "", "Model to use for translation (provider-specific, uses sensible defaults)")
	translateCmd.Flags().
		Bool("model-override", false, "Allow any custom model, bypassing provider model validation")
	translateCmd.Flags().
		String("provider", "gemini", "Translation provider (gemini, openai)")
	translateCmd.Flags().
		Int("concurrency", 3, "Number of parallel translation workers")
	translateCmd.Flags().
		Int("batch-size", 50, "Number of subtitle entries per API request")

	_ = translateCmd.MarkFlagRequired("target-language")
}

func runTranslate(cmd *cobra.Command, args []string) error {
	subtitlePath := args[0]
	ctx := context.Background()

	targetLang, _ := cmd.Flags().GetString("target-language")
	overlay, _ := cmd.Flags().GetBool("overlay")
	apiKey, _ := cmd.Flags().GetString("api-key")
	model, _ := cmd.Flags().GetString("model")
	modelOverride, _ := cmd.Flags().GetBool("model-override")
	providerStr, _ := cmd.Flags().GetString("provider")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	outputPath, _ := cmd.Flags().GetString("output")
	inputLang, _ := cmd.Flags().GetString("language")

	if _, err := os.Stat(subtitlePath); os.IsNotExist(err) {
		return fmt.Errorf("subtitle file not found: %s", subtitlePath)
	}

	ext := strings.ToLower(filepath.Ext(subtitlePath))
	if ext != ".srt" && ext != ".vtt" && ext != ".ass" && ext != ".ssa" {
		return fmt.Errorf(
			"unsupported subtitle format %q: use .srt, .vtt, .ass, or .ssa",
			ext,
		)
	}

	if targetLang == "" {
		return fmt.Errorf("target language is required")
	}

	if inputLang != "" &&
		strings.EqualFold(
			strings.TrimSpace(inputLang),
			strings.TrimSpace(targetLang),
		) {
		return fmt.Errorf(
			"input language %q and target language %q cannot be the same",
			inputLang,
			targetLang,
		)
	}

	provider := translate.Provider(providerStr)

	if apiKey == "" {
		switch provider {
		case translate.ProviderGemini:
			apiKey = os.Getenv("GEMINI_API_KEY")
		case translate.ProviderOpenAI:
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	if apiKey == "" {
		var envVar string
		switch provider {
		case translate.ProviderGemini:
			envVar = "GEMINI_API_KEY"
		case translate.ProviderOpenAI:
			envVar = "OPENAI_API_KEY"
		default:
			envVar = "API_KEY"
		}
		return fmt.Errorf(
			"API key is required: use --api-key flag or set %s environment variable",
			envVar,
		)
	}

	if model != "" && !modelOverride {
		switch provider {
		case translate.ProviderGemini:
			if !isValidGeminiModel(model) {
				return fmt.Errorf(
					"unsupported Gemini model %q: valid models are gemini-3-pro-preview, gemini-3-flash-preview, gemini-2.5-pro, gemini-2.5-flash, gemini-2.5-flash-lite (use --model-override to bypass)",
					model,
				)
			}
		case translate.ProviderOpenAI:
			if !isValidOpenAIModel(model) {
				return fmt.Errorf(
					"unsupported OpenAI model %q: valid models are o1, o3-mini, o1-pro, o3, gpt-5, gpt-5-nano, gpt-5-mini, gpt-5-pro, gpt-5.1, gpt-5.2, gpt-5.2-pro (use --model-override to bypass)",
					model,
				)
			}
		}
	}

	if concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive, got %d", concurrency)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batch-size must be positive, got %d", batchSize)
	}

	if outputPath == "" {
		baseName := strings.TrimSuffix(subtitlePath, filepath.Ext(subtitlePath))
		if overlay {
			outputPath = fmt.Sprintf(
				"%s.%s.overlay%s",
				baseName,
				targetLang,
				ext,
			)
		} else {
			outputPath = fmt.Sprintf("%s.%s%s", baseName, targetLang, ext)
		}
	}

	logger.Infow("Starting subtitle translation",
		"input", subtitlePath,
		"output", outputPath,
		"target_language", targetLang,
		"input_language", inputLang,
		"overlay", overlay,
		"model", model,
	)

	logger.Infow("Parsing subtitle file")
	subFile, err := subtitle.Open(subtitlePath)
	if err != nil {
		return fmt.Errorf("failed to parse subtitle file: %w", err)
	}

	sub := subFile.Subtitle()
	if len(sub.Entries) == 0 {
		return fmt.Errorf("subtitle file contains no entries")
	}

	logger.Infow("Parsed subtitle file",
		"entries", len(sub.Entries),
		"format", subFile.Format(),
	)

	opts := translate.Options{
		InputLanguage:  inputLang,
		TargetLanguage: targetLang,
		Model:          model,
		BatchSize:      batchSize,
	}

	translator, err := translate.Factory(ctx, provider, apiKey, opts)
	if err != nil {
		return fmt.Errorf("failed to create translator: %w", err)
	}

	items := make([]translate.TranslationItem, len(sub.Entries))
	for i, entry := range sub.Entries {
		items[i] = translate.TranslationItem{
			Index: i,
			Text:  entry.Text,
		}
	}

	logger.Infow("Translating subtitles",
		"items", len(items),
		"concurrency", concurrency,
	)

	var results []translate.TranslationResult
	if concurrentTranslator, ok := translator.(translate.ConcurrentTranslator); ok {
		results, err = concurrentTranslator.TranslateWithConcurrency(
			ctx,
			items,
			concurrency,
		)
	} else {
		results, err = translator.Translate(ctx, items)
	}
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	logger.Infow("Translation complete",
		"results", len(results),
	)

	assFile, isASS := subFile.(*subtitle.ASSFile)

	for _, result := range results {
		if result.Index < 0 || result.Index >= len(sub.Entries) {
			logger.Warnw("Skipping invalid result index",
				"index", result.Index,
				"max", len(sub.Entries)-1,
			)
			continue
		}

		if overlay {
			if isASS {
				if err := assFile.SetTextWithOverlay(
					result.Index,
					result.Text,
				); err != nil {
					return fmt.Errorf(
						"failed to set overlay text for entry %d: %w",
						result.Index,
						err,
					)
				}
			} else {
				// translated + newline + original
				originalText := sub.Entries[result.Index].Text
				overlayText := result.Text + "\n" + originalText
				if err := subFile.SetText(
					result.Index,
					overlayText,
				); err != nil {
					return fmt.Errorf(
						"failed to set overlay text for entry %d: %w",
						result.Index,
						err,
					)
				}
			}
		} else {
			// replace with translation
			if err := subFile.SetText(result.Index, result.Text); err != nil {
				return fmt.Errorf(
					"failed to set text for entry %d: %w",
					result.Index,
					err,
				)
			}
		}
	}

	logger.Infow("Writing output file")
	if err := subFile.Write(outputPath); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	absOutput, _ := filepath.Abs(outputPath)
	fmt.Printf("Subtitles translated successfully: %s\n", absOutput)
	fmt.Printf("  Entries: %d\n", len(sub.Entries))
	fmt.Printf("  Target language: %s\n", targetLang)
	if overlay {
		fmt.Printf("  Mode: bilingual overlay\n")
	}

	return nil
}
