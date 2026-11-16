package asc

// HasAccessibilityDeclarationUpdates reports whether any update attributes are set.
func HasAccessibilityDeclarationUpdates(attrs AccessibilityDeclarationUpdateAttributes) bool {
	return attrs.Publish != nil ||
		attrs.SupportsAudioDescriptions != nil ||
		attrs.SupportsCaptions != nil ||
		attrs.SupportsDarkInterface != nil ||
		attrs.SupportsDifferentiateWithoutColorAlone != nil ||
		attrs.SupportsLargerText != nil ||
		attrs.SupportsReducedMotion != nil ||
		attrs.SupportsSufficientContrast != nil ||
		attrs.SupportsVoiceControl != nil ||
		attrs.SupportsVoiceover != nil
}
