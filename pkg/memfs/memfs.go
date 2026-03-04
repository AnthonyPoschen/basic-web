package memfs

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

type MemFS struct {
	files   map[string][]byte
	entries []fs.DirEntry
}

func NewMemFS(files map[string][]byte, entries []fs.DirEntry) *MemFS {
	return &MemFS{files: files, entries: entries}
}

func (m *MemFS) Open(name string) (fs.File, error) {
	if name == "." || name == "" {
		return &memFile{
			Reader:  bytes.NewReader(nil),
			name:    name,
			isDir:   true,
			entries: m.entries,
		}, nil
	}
	if data, ok := m.files[name]; ok {
		return &memFile{
			Reader: bytes.NewReader(data),
			name:   name,
			isDir:  false,
		}, nil
	}
	return nil, fs.ErrNotExist
}

type memFile struct {
	*bytes.Reader
	name    string
	isDir   bool
	entries []fs.DirEntry
}

func (m *memFile) Stat() (fs.FileInfo, error) {
	if m.isDir {
		return &MemFileInfo{name: m.name, isDir: true}, nil
	}
	return &MemFileInfo{name: m.name, size: int64(m.Reader.Len())}, nil
}

func (m *memFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !m.isDir {
		return nil, &fs.PathError{Op: "readdir", Path: m.name, Err: fs.ErrInvalid}
	}
	if n <= 0 {
		return m.entries, nil
	}
	if n > len(m.entries) {
		n = len(m.entries)
	}
	return m.entries[:n], nil
}

func (m *memFile) Close() error {
	return nil
}

type MemFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func NewMemFileInfo(name string, size int64) *MemFileInfo {
	return &MemFileInfo{name: name, size: size}
}

func (m *MemFileInfo) Name() string {
	return m.name
}

func (m *MemFileInfo) Size() int64 {
	return m.size
}

func (m *MemFileInfo) Mode() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0644
}

func (m *MemFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (m *MemFileInfo) IsDir() bool {
	return m.isDir
}

func (m *MemFileInfo) Sys() interface{} {
	return nil
}

func CreateMinifiedFS(webFS fs.FS) *MemFS {
	files := make(map[string][]byte)
	var entries []fs.DirEntry
	m := minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)
	fs.WalkDir(webFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(webFS, path)
		if err != nil {
			return err
		}
		var minifiedData []byte
		switch filepath.Ext(path) {
		case ".html":
			minifiedData, err = m.Bytes("text/html", data)
		case ".css":
			minifiedData, err = m.Bytes("text/css", data)
		case ".js":
			minifiedData, err = m.Bytes("application/javascript", data)
		default:
			minifiedData = data
		}
		if err != nil {
			return err
		}
		files[path] = minifiedData
		entries = append(entries, fs.FileInfoToDirEntry(NewMemFileInfo(path, int64(len(minifiedData)))))
		return nil
	})
	return NewMemFS(files, entries)
}
