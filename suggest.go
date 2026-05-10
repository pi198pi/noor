package main

import "strings"

// suggestModel returns a model recommendation based on the user's input.
// Returns an empty string if the current model is fine. The recommendation
// is non-binding — we only print it as a tip; the user keeps control.
func suggestModel(input, currentModel string) string {
	lower := strings.ToLower(input)
	length := len(input)

	// Image generation/editing
	imageWords := []string{"generate image", "create image", "draw ", "make a picture", "edit this photo", "edit the image", "remove background"}
	for _, w := range imageWords {
		if strings.Contains(lower, w) {
			if !strings.Contains(currentModel, "-image") {
				return "google/gemini-2.5-flash-image"
			}
			return ""
		}
	}

	// Long input or code-heavy → suggest Sonnet
	codeWords := []string{"function ", "class ", "import ", "package ", "fn ", "def ", "const ", "let ", "var ", "```", "interface ", "implements ", "compile error", "stack trace", "panic:", "traceback"}
	codeScore := 0
	for _, w := range codeWords {
		if strings.Contains(lower, w) {
			codeScore++
		}
	}
	if (length > 1500 || codeScore >= 2) && currentModel == "anthropic/claude-haiku-4.5" {
		return "anthropic/claude-sonnet-4.6"
	}

	// Heavy reasoning words → suggest Opus
	reasoningWords := []string{"prove that", "derive ", "complexity analysis", "design a system", "architecture for", "tradeoffs between", "step by step reasoning"}
	for _, w := range reasoningWords {
		if strings.Contains(lower, w) {
			if currentModel == "anthropic/claude-haiku-4.5" {
				return "anthropic/claude-opus-4.7"
			}
			return ""
		}
	}

	return ""
}
