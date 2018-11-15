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

type fileRecord struct {
	Meta     map[string]interface{}
	RelPath  string
	DepPaths []string
}

func (c *fileCache) retrieveFile(context *Context, inputFile *File) (*File, error) {
	dataPath, recordPath := c.buildCachePaths(context, inputFile)

	record, err := c.readFileRecord(recordPath)
	if err != nil {
		return nil, err
	}

	outputFile, err := NewFileFromAsset(record.RelPath, dataPath)
	if err != nil {
		return nil, err
	}

	if record.Meta != nil {
		outputFile.Meta = record.Meta
	}

	if inputFile.ModTime().After(outputFile.ModTime()) {
		return nil, nil
	}

	return outputFile, nil
}

func (c *fileCache) storeFile(context *Context, inputFile, outputFile *File, depPaths []string) error {
	dataPath, recordPath := c.buildCachePaths(context, inputFile)

	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return err
	}

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileRecord(recordPath, outputFile, depPaths); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) readFileRecord(path string) (*fileRecord, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var record fileRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

func (c *fileCache) writeFileData(path string, file *File) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := file.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) writeFileRecord(path string, file *File, depPaths []string) error {
	record := fileRecord{
		Meta:     file.Meta,
		RelPath:  file.Path(),
		DepPaths: depPaths,
	}

	json, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, json, 0666)
}

func (c *fileCache) buildCachePaths(context *Context, file *File) (string, string) {
	hashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hashBytes, context.hash)

	hash := crc32.NewIEEE()
	hash.Write(hashBytes)
	hash.Write([]byte(file.Path()))
	hashSum := hash.Sum32()

	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_dat", hashSum))
	recordPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_rec", hashSum))

	return dataPath, recordPath
}
