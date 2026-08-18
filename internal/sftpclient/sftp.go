package sftpclient

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/pkg/sftp"
)

// Entry represents a single file or directory on the remote side.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Owner   string `json:"owner"`
	Group   string `json:"group"`
}

// List reads a remote directory and returns sorted entries (dirs first).
func List(client *sftp.Client, dir string) ([]Entry, error) {
	if dir == "" {
		dir = "."
	}
	infos, err := client.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, Entry{
			Name:    info.Name(),
			Path:    path.Join(dir, info.Name()),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().UnixMilli(),
			IsDir:   info.IsDir(),
			Owner:   "", // owner/group require extra lookup; keep empty for now
			Group:   "",
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// ReadFile reads a remote file and returns its content.
func ReadFile(client *sftp.Client, p string) ([]byte, error) {
	f, err := client.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > 10*1024*1024 {
		return nil, fmt.Errorf("file too large (>10MB)")
	}
	buf := make([]byte, stat.Size())
	_, err = f.Read(buf)
	return buf, err
}

// WriteFile writes data to a remote file.
func WriteFile(client *sftp.Client, p string, data []byte) error {
	f, err := client.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// Upload copies a local file to a remote path (creates parent dirs as needed).
func Upload(client *sftp.Client, localPath, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local: %w", err)
	}
	defer src.Close()

	if dir := path.Dir(remotePath); dir != "." && dir != "/" {
		_ = client.MkdirAll(dir)
	}

	dst, err := client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Download copies a remote file to a local path.
func Download(client *sftp.Client, remotePath, localPath string) error {
	src, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote: %w", err)
	}
	defer src.Close()

	if dir := filepath.Dir(localPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir local: %w", err)
		}
	}

	dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open local: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Rename moves/renames a remote path.
func Rename(client *sftp.Client, oldPath, newPath string) error {
	return client.Rename(oldPath, newPath)
}

// Delete removes a remote file or directory recursively.
func Delete(client *sftp.Client, p string) error {
	info, err := client.Stat(p)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return client.Remove(p)
	}
	entries, err := client.ReadDir(p)
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := path.Join(p, e.Name())
		if e.IsDir() {
			if err := Delete(client, child); err != nil {
				return err
			}
		} else {
			if err := client.Remove(child); err != nil {
				return err
			}
		}
	}
	return client.RemoveDirectory(p)
}

// Mkdir creates a remote directory (and parents if needed).
func Mkdir(client *sftp.Client, p string) error {
	return client.MkdirAll(p)
}

// Stat returns information about a remote path.
func Stat(client *sftp.Client, p string) (os.FileInfo, error) {
	return client.Stat(p)
}
