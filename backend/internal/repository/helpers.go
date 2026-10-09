package repository

// nullable converts a Go string to a *string that is nil when the input
// is empty. Used for INSERT/UPDATE against nullable TEXT columns.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}