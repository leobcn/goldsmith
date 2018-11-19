package goldsmith

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
)

type fileCache struct {
	baseDir string
}

func (c *fileCache) retrieveFile(context *Context, outputPath string, inputPaths ...string) (*File, error) {
	dataPath, metaPath := c.buildCachePaths(context, outputPath)

	outputFile, err := NewFileFromAsset(outputPath, dataPath)
	if err != nil {
		return nil, err
	}

	meta, err := c.readFileMeta(metaPath)
	if err != nil {
		return nil, err
	}

	if meta != nil {
		outputFile.Meta = meta
	}

	for _, depPath := range inputPaths {
		if stat, err := os.Stat(depPath); err != nil || stat.ModTime().After(outputFile.ModTime()) {
			return nil, err
		}
	}

	return outputFile, nil
}

func (c *fileCache) storeFile(context *Context, outputFile *File, inputPaths ...string) error {
	dataPath, metaPath := c.buildCachePaths(context, outputFile.Path())

	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return err
	}

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileMeta(metaPath, outputFile); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) readFileMeta(path string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]interface{})
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (c *fileCache) writeFileData(path string, file *File) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := file.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err := file.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) writeFileMeta(path string, file *File) error {
	data, err := json.Marshal(file.Meta)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0666)
}

func (c *fileCache) buildCachePaths(context *Context, path string) (string, string) {
	hashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hashBytes, context.hash)

	hash := crc32.NewIEEE()
	hash.Write(hashBytes)
	hash.Write([]byte(path))
	hashSum := hash.Sum32()

	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_dat", hashSum))
	metaPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_rec", hashSum))

	return dataPath, metaPath
}
