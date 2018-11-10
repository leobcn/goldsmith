package goldsmith

import (
	"os"
	"path/filepath"
)

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

func scanDir(root string, infos chan fileInfo) {
	defer close(infos)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		infos <- fileInfo{FileInfo: info, path: path}
		return nil
	})
}
