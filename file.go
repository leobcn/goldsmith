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
	path string
	Meta map[string]interface{}

	reader  *bytes.Reader
	size    int64
	modTime time.Time

	asset string
}

func NewFileFromData(path string, data []byte, modTime time.Time) *File {
	return &File{
		path:    path,
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

	f := &File{
		path:    path,
		Meta:    make(map[string]interface{}),
		size:    info.Size(),
		modTime: info.ModTime(),
		asset:   asset,
	}

	return f, nil
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Name() string {
	return path.Base(f.path)
}

func (f *File) Dir() string {
	return path.Dir(f.path)
}

func (f *File) Ext() string {
	return path.Ext(f.path)
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

func (f *File) InheritValues(src *File) {
	for name, value := range src.Meta {
		f.SetValue(name, value)
	}
}

func (f *File) Read(p []byte) (int, error) {
	if err := f.cache(); err != nil {
		return 0, err
	}

	return f.reader.Read(p)
}

func (f *File) WriteTo(w io.Writer) (int64, error) {
	if err := f.cache(); err != nil {
		return 0, err
	}

	return f.reader.WriteTo(w)
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.reader == nil && offset == 0 && (whence == os.SEEK_SET || whence == os.SEEK_CUR) {
		return 0, nil
	}

	if err := f.cache(); err != nil {
		return 0, err
	}

	return f.reader.Seek(offset, whence)
}

func (f *File) export(dstDir string) error {
	dstPath := filepath.Join(dstDir, f.path)
	if dstInfo, err := os.Stat(dstPath); err == nil && dstInfo.ModTime().Unix() >= f.ModTime().Unix() {
		return nil
	}

	if err := os.MkdirAll(path.Dir(dstPath), 0755); err != nil {
		return err
	}

	fw, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer fw.Close()

	if f.reader == nil {
		fr, err := os.Open(f.asset)
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

func (f *File) cache() error {
	if f.reader != nil {
		return nil
	}

	data, err := ioutil.ReadFile(f.asset)
	if err != nil {
		return err
	}

	f.reader = bytes.NewReader(data)
	return nil
}
