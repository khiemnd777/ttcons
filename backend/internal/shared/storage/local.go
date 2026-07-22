//go:build darwin || linux

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

const LocalProvider = "local"

type Local struct {
	root        string
	rootFile    *os.File
	mu          sync.RWMutex
	beforeFinal func()
}

func NewLocal(root string) (*Local, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local storage root is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(realRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	rootFile := os.NewFile(uintptr(fd), realRoot)
	if rootFile == nil {
		_ = unix.Close(fd)
		return nil, errors.New("cannot open local storage root")
	}
	return &Local{root: realRoot, rootFile: rootFile}, nil
}

func (l *Local) Provider() string { return LocalProvider }

func (l *Local) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rootFile == nil {
		return nil
	}
	err := l.rootFile.Close()
	l.rootFile = nil
	return err
}

func (l *Local) Put(ctx context.Context, key string, source io.Reader, options PutOptions) (ObjectInfo, error) {
	if source == nil {
		return ObjectInfo{}, errors.New("storage source is nil")
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	parent, leaf, err := l.openParent(key, true)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer parent.Close()
	l.invokeBeforeFinal()
	fd, err := unix.Openat(int(parent.Fd()), leaf, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o640)
	if err != nil {
		if errors.Is(err, unix.EEXIST) {
			return ObjectInfo{}, fmt.Errorf("%w: %s", ErrObjectExists, key)
		}
		if errors.Is(err, unix.ELOOP) {
			return ObjectInfo{}, fmt.Errorf("%w: %s", ErrInvalidKey, key)
		}
		return ObjectInfo{}, err
	}
	file := os.NewFile(uintptr(fd), leaf)
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = unix.Unlinkat(int(parent.Fd()), leaf, 0)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), &contextReader{ctx: ctx, reader: source})
	if err != nil {
		return ObjectInfo{}, err
	}
	if options.ExpectedSize >= 0 && written != options.ExpectedSize {
		return ObjectInfo{}, fmt.Errorf("storage size mismatch: expected %d, wrote %d", options.ExpectedSize, written)
	}
	if err := file.Sync(); err != nil {
		return ObjectInfo{}, err
	}
	if err := file.Close(); err != nil {
		return ObjectInfo{}, err
	}
	keep = true
	return ObjectInfo{Provider: LocalProvider, Key: key, Size: written, ContentType: strings.TrimSpace(options.ContentType), ChecksumSHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func (l *Local) Open(ctx context.Context, key string) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	parent, leaf, err := l.openParent(key, false)
	if err != nil {
		return Object{}, err
	}
	defer parent.Close()
	l.invokeBeforeFinal()
	fd, err := unix.Openat(int(parent.Fd()), leaf, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return Object{}, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		if errors.Is(err, unix.ELOOP) {
			return Object{}, fmt.Errorf("%w: %s", ErrInvalidKey, key)
		}
		return Object{}, err
	}
	file := os.NewFile(uintptr(fd), leaf)
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return Object{}, err
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return Object{}, fmt.Errorf("%w: %s", ErrInvalidKey, key)
	}
	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		_ = file.Close()
		return Object{}, readErr
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return Object{}, err
	}
	contentType := "application/octet-stream"
	if n > 0 {
		contentType = http.DetectContentType(header[:n])
	}
	return Object{Reader: file, Info: ObjectInfo{Provider: LocalProvider, Key: key, Size: openedInfo.Size(), ContentType: contentType}}, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	parent, leaf, err := l.openParent(key, false)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	defer parent.Close()
	l.invokeBeforeFinal()
	var stat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), leaf, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%w: %s", ErrInvalidKey, key)
	}
	if err := unix.Unlinkat(int(parent.Fd()), leaf, 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	return nil
}

func (l *Local) openParent(key string, create bool) (*os.File, string, error) {
	segments, err := storageKeySegments(key)
	if err != nil {
		return nil, "", err
	}
	if l == nil || l.rootFile == nil {
		return nil, "", errors.New("local storage is not configured")
	}
	rootFD, err := unix.Dup(int(l.rootFile.Fd()))
	if err != nil {
		return nil, "", err
	}
	unix.CloseOnExec(rootFD)
	current := os.NewFile(uintptr(rootFD), l.root)
	for _, segment := range segments[:len(segments)-1] {
		nextFD, openErr := unix.Openat(int(current.Fd()), segment, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if errors.Is(openErr, unix.ENOENT) && create {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), segment, 0o750); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				_ = current.Close()
				return nil, "", mkdirErr
			}
			nextFD, openErr = unix.Openat(int(current.Fd()), segment, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		if openErr != nil {
			_ = current.Close()
			if errors.Is(openErr, unix.ENOENT) {
				return nil, "", fmt.Errorf("%w: parent directory is missing", ErrNotFound)
			}
			if errors.Is(openErr, unix.ELOOP) || errors.Is(openErr, unix.ENOTDIR) {
				return nil, "", fmt.Errorf("%w: parent path is not a directory", ErrInvalidKey)
			}
			return nil, "", openErr
		}
		next := os.NewFile(uintptr(nextFD), segment)
		_ = current.Close()
		current = next
	}
	return current, segments[len(segments)-1], nil
}

func storageKeySegments(key string) ([]string, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsRune(key, '\x00') || strings.HasPrefix(key, "/") || path.Clean(key) != key {
		return nil, fmt.Errorf("%w: %s", ErrInvalidKey, key)
	}
	segments := strings.Split(key, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return nil, fmt.Errorf("%w: %s", ErrInvalidKey, key)
		}
	}
	return segments, nil
}

func (l *Local) invokeBeforeFinal() {
	if l.beforeFinal != nil {
		l.beforeFinal()
	}
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
