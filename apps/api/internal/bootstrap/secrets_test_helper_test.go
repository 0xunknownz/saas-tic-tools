package bootstrap

import "encoding/base64"

// testSecretsKey is a fixed 32-byte SECRETS_KEY for tests; notification and identity require one.
func testSecretsKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}
