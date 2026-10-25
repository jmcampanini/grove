package worktree

// ShortSHASafe abbreviates a commit identifier, or returns "(no sha)" for an empty one.
func ShortSHASafe(sha string, maxLen int) string {
	if sha == "" {
		return "(no sha)"
	}
	if len(sha) <= maxLen {
		return sha
	}
	return sha[:maxLen]
}
