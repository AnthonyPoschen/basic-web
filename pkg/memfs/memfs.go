package memfs

import (
	"bytes"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

type MemFS struct {
	files map[string][]byte
	dirs  map[string][]fs.DirEntry
}

func NewMemFS(files map[string][]byte, dirs map[string][]fs.DirEntry) *MemFS {
	return &MemFS{files: files, dirs: dirs}
}

func (m *MemFS) Open(name string) (fs.File, error) {
	if name == "." || name == "" {
		name = "."
	}
	if entries, ok := m.dirs[name]; ok {
		return &memFile{
			Reader:  bytes.NewReader(nil),
			name:    name,
			isDir:   true,
			entries: entries,
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
		return &MemFileInfo{name: path.Base(m.name), isDir: true}, nil
	}
	return &MemFileInfo{name: path.Base(m.name), size: int64(m.Reader.Len())}, nil
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

// CreateMinifiedFS copies webFS into memory with HTML, CSS, and JS minified.
// Authors keep readable source; production responses should still be small.
// See docs/performance.md.
func CreateMinifiedFS(webFS fs.FS) *MemFS {
	files := make(map[string][]byte)
	dirs := map[string][]fs.DirEntry{".": {}}
	m := minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)
	fs.WalkDir(webFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			ensureDir(dirs, path)
			addDirToParent(dirs, path)
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
		addFileToParent(dirs, path, len(minifiedData))
		return nil
	})
	sortDirEntries(dirs)
	return NewMemFS(files, dirs)
}

func ensureDir(dirs map[string][]fs.DirEntry, dir string) {
	if dir == "" {
		dir = "."
	}
	if _, ok := dirs[dir]; !ok {
		dirs[dir] = nil
	}
}

func addDirToParent(dirs map[string][]fs.DirEntry, dir string) {
	if dir == "." || dir == "" {
		return
	}
	parent := path.Dir(dir)
	ensureDir(dirs, parent)
	addDirEntry(dirs, parent, fs.FileInfoToDirEntry(&MemFileInfo{name: path.Base(dir), isDir: true}))
}

func addFileToParent(dirs map[string][]fs.DirEntry, filePath string, size int) {
	parent := path.Dir(filePath)
	ensureDir(dirs, parent)
	addDirEntry(dirs, parent, fs.FileInfoToDirEntry(NewMemFileInfo(path.Base(filePath), int64(size))))
}

func addDirEntry(dirs map[string][]fs.DirEntry, dir string, entry fs.DirEntry) {
	for _, existing := range dirs[dir] {
		if existing.Name() == entry.Name() {
			return
		}
	}
	dirs[dir] = append(dirs[dir], entry)
}

func sortDirEntries(dirs map[string][]fs.DirEntry) {
	for dir := range dirs {
		sort.Slice(dirs[dir], func(i, j int) bool {
			return dirs[dir][i].Name() < dirs[dir][j].Name()
		})
	}
}
