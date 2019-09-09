package mocks

// Mocks a Conjur access token.
type AccessToken struct{}

// Returns an arbitrary byte array as an access token data as we don't really need it
func (accessToken AccessToken) Read() ([]byte, error) {
	return []byte("someAccessToken"), nil
}

// This method implementation is only so AccessToken will implement the AccessToken interface
func (accessToken AccessToken) Write(Data []byte) error {
	return nil
}

// This method implementation is only so AccessToken will implement the AccessToken interface
func (accessToken AccessToken) Delete() error {
	return nil
}
