package desktop

// Permission can be revoked while a blocking native query is in progress. In
// particular, a redacted nil SSID after that query is not a known disconnection.
// Keep this sequencing independent of the native API so it can be tested
// without requesting authorization or reading any workstation network state.
func wifiPermissionRead(check func() error, read func() (WiFiState, error)) (WiFiState, error) {
	if err := check(); err != nil {
		return WiFiState{}, err
	}
	state, err := read()
	if err != nil {
		return WiFiState{}, err
	}
	if err := check(); err != nil {
		return WiFiState{}, err
	}
	return state, nil
}
