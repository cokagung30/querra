//go:build !keychain

package secrets

// realKeychain is false in normal test runs: Keyring is tested against
// go-keyring's mock provider.
const realKeychain = false
