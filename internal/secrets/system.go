package secrets

import (
	"errors"

	"github.com/egoist/mygo"
)

// System is the system's credential store, through MyGo, under the app's
// identifier.
type System struct{}

// Get implements Backend.
func (System) Get(key string) ([]byte, error) {
	value, err := mygo.Secrets.Get(key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, ErrNotFound
	}
	return value, err
}

// Set implements Backend.
func (System) Set(key string, value []byte) error { return mygo.Secrets.Set(key, value) }

// Delete implements Backend.
func (System) Delete(key string) error { return mygo.Secrets.Delete(key) }
