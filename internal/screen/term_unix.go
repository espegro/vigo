//go:build !windows

package screen

// setupPlatform prepares platform specific terminal settings.
func setupPlatform(t *Terminal) error { return nil }
