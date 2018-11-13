package goldsmith

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

type File struct {
	relPath string
	extPath string

	Meta map[string]interface{}

	reader  *bytes.Reader
	size    int64
	modTime time.Time
}

func NewFileFromData(path string, data []byte, modTime time.Time) *File {
	return &File{
		relPath: path,
		Meta:    make(map[string]interface{}),
		reader:  bytes.NewReader(data),
		size:    int64(len(data)),
		modTime: modTime,
	}
}

func NewFileFromAsset(path, asset string) (*File, error) {
	info, err := os.Stat(asset)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, errors.New("assets must be files")
	}

	file := &File{
		relPath: path,
		extPath: asset,
		Meta:    make(map[string]interface{}),
		modTime: info.ModTime(),
		size:    info.Size(),
	}

	return file, nil
}

func (f *File) Path() string {
	return f.relPath
}

func (f *File) Name() string {
	return path.Base(f.relPath)
}

func (f *File) Dir() string {
	return path.Dir(f.relPath)
}

func (f *File) Ext() string {
	return path.Ext(f.relPath)
}

func (f *File) Size() int64 {
	return f.size
}

func (f *File) ModTime() time.Time {
	return f.modTime
}

func (f *File) Value(key string) (interface{}, bool) {
	value, ok := f.Meta[key]
	return value, ok
}

func (f *File) SetValue(key string, value interface{}) {
	f.Meta[key] = value
}

func (f *File) InheritValues(sourceFile *File) {
	for name, value := range sourceFile.Meta {
		f.SetValue(name, value)
	}
}

func (f *File) Read(data []byte) (int, error) {
	if err := f.load(); err != nil {
		return 0, err
	}

	return f.reader.Read(data)
}

func (f *File) WriteTo(writer io.Writer) (int64, error) {
	if err := f.load(); err != nil {
		return 0, err
	}

	return f.reader.WriteTo(writer)
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.reader == nil && offset == 0 && (whence == os.SEEK_SET || whence == os.SEEK_CUR) {
		return 0, nil
	}

	if err := f.load(); err != nil {
		return 0, err
	}

	return f.reader.Seek(offset, whence)
}

func (f *File) export(targetDir string) error {
	targetPath := filepath.Join(targetDir, f.relPath)
	if targetInfo, err := os.Stat(targetPath); err == nil && targetInfo.ModTime().Unix() >= f.ModTime().Unix() {
		return nil
	}

	if err := os.MkdirAll(path.Dir(targetPath), 0755); err != nil {
		return err
	}

	fw, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer fw.Close()

	if f.reader == nil {
		fr, err := os.Open(f.extPath)
		if err != nil {
			return err
		}
		defer fr.Close()

		if _, err := io.Copy(fw, fr); err != nil {
			return err
		}
	} else {
		if _, err := f.Seek(0, os.SEEK_SET); err != nil {
			return err
		}
		if _, err := f.WriteTo(fw); err != nil {
			return err
		}
	}

	return nil
}

func (f *File) load() error {
	if f.reader != nil {
		return nil
	}

	data, err := ioutil.ReadFile(f.extPath)
	if err != nil {
		return err
	}

	f.reader = bytes.NewReader(data)
	return nil
}

type fileInfo struct {
	os.FileInfo
	path string
}

func cleanPath(path string) string {
	if filepath.IsAbs(path) {
		var err error
		if path, err = filepath.Rel("/", path); err != nil {
			panic(err)
		}
	}

	return filepath.Clean(path)
}

func scanDir(rootDir string, infos chan fileInfo) {
	defer close(infos)

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		infos <- fileInfo{FileInfo: info, path: path}
		return nil
	})
}
