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

type fileMeta struct {
	Hash uint32
}

func (c *fileCache) retrieveFile(context *Context, outputPath string, inputFile *File) (*File, error) {
	cachePath, metaPath := c.buildCachePaths(context, outputPath)

	outputFile, err := NewFileFromAsset(outputPath, cachePath)
	if err != nil {
		return nil, err
	}

	if inputFile != nil && inputFile.ModTime().After(outputFile.ModTime()) {
		return nil, nil
	}

	metaData, err := ioutil.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta fileMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}

	outputFile.hashValue = meta.Hash
	outputFile.hashValid = true

	return outputFile, nil
}

func (c *fileCache) storeFile(context *Context, outputFile *File) error {
	cachePath, metaPath := c.buildCachePaths(context, outputFile.Path())

	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return err
	}

	fp, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := outputFile.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err := outputFile.WriteTo(fp); err != nil {
		return err
	}

	if err := outputFile.hash(); err != nil {
		return err
	}

	metaData, err := json.Marshal(&fileMeta{outputFile.hashValue})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(metaPath, metaData, 0644); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) buildCachePaths(context *Context, path string) (string, string) {
	hashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hashBytes, context.hash)

	hash := crc32.NewIEEE()
	hash.Write(hashBytes)
	hash.Write([]byte(path))

	var (
		basePath  = filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x", hash.Sum32()))
		cachePath = fmt.Sprintf("%s_data%s", basePath, filepath.Ext(path))
		metaPath  = fmt.Sprintf("%s_meta.json", basePath)
	)

	return cachePath, metaPath
}
