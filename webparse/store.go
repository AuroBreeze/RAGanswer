package webparse

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path"

	"crypto/rand"
	"encoding/hex"
)

var ErrImmutableConflict = errors.New("immutable object already exists with different bytes")

// Store is a local, private snapshot and artifact store. The root must already exist.
type Store struct {
	root *os.Root
}

func OpenStore(directory string) (*Store, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Close() error { return s.root.Close() }

func (s *Store) read(ref string, limit int) ([]byte, error) {
	f, err := s.root.Open(ref)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, errors.New("object is not a regular file within the size limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err == nil && len(data) > limit {
		return nil, errors.New("object exceeds size limit")
	}
	return data, err
}

func (s *Store) putImmutable(ref string, data []byte) error {
	dir := path.Dir(ref)
	parent := path.Dir(dir)
	if err := s.root.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	tmp := path.Join(parent, ".tmp-"+hex.EncodeToString(random[:]))
	if err := s.root.Mkdir(tmp, 0o700); err != nil {
		return err
	}
	defer s.root.Remove(tmp)
	tempFile := path.Join(tmp, path.Base(ref))
	f, err := s.root.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer s.root.Remove(tempFile)
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return err
	}
	// A published object's directory is nonempty and cannot be replaced by Rename.
	if renameErr := s.root.Rename(tmp, dir); renameErr != nil {
		current, err := s.read(ref, MaxArtifactBytes)
		if err != nil {
			return errors.Join(renameErr, err)
		}
		if !bytes.Equal(current, data) {
			return ErrImmutableConflict
		}
	}
	return nil
}
