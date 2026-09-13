package storage

import (
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
)

func LockFiles(paths ...string) (func() error, error) {
	locks := []*flock.Flock{}
	unlock := func() error {
		var err error
		for _, lock := range locks {
			err = errors.Join(err, lock.Unlock())
		}
		return err
	}
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, errors.Join(err, unlock())
		}
		if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
			return nil, errors.Join(err, unlock())
		}
		lock := flock.New(absolute + ".lock")
		locked, err := lock.TryLock()
		if err != nil {
			return nil, errors.Join(err, unlock())
		}
		if !locked {
			return nil, errors.Join(fmt.Errorf("stockage déjà utilisé : %s", path), unlock())
		}
		locks = append(locks, lock)
	}
	return unlock, nil
}
