package core

// SetStateVersion makes state files written and read from now on behave as if
// this were another sakumock release, and returns the function that undoes it.
func SetStateVersion(v string) (restore func()) {
	prev := stateVersion
	stateVersion = v
	return func() { stateVersion = prev }
}
