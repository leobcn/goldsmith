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

type cache struct {
	baseDir string
}

type cacheEntry struct {
	Meta     map[string]interface{}
	RelPath  string
	DepPaths []string
}

func (c *cache) buildCachePaths(context *Context, file *File) (string, string) {
	fileHashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(fileHashBytes, file.Hash())

	hash := crc32.NewIEEE()
	hash.Write([]byte(context.plugin.Name()))
	hash.Write([]byte(file.Path()))
	hash.Write(fileHashBytes)

	stateHash := hash.Sum32()
	fileExt := filepath.Ext(file.Path())

	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_data%s", stateHash, fileExt))
	entryPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_entry.json", stateHash))

	return dataPath, entryPath
}

func (c *cache) readFile(context *Context, inputFile *File) (*File, error) {
	if len(c.baseDir) == 0 {
		return nil, nil
	}

	dataPath, entryPath := c.buildCachePaths(context, inputFile)

	entry, err := c.readFileEntry(entryPath)
	if err != nil {
		return nil, err
	}

	outputFile, err := NewFileFromAsset(entry.RelPath, dataPath)
	if err != nil {
		return nil, err
	}

	outputFile.Meta = entry.Meta
	return outputFile, nil
}

func (c *cache) readFileEntry(path string) (*cacheEntry, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (c *cache) writeFile(context *Context, inputFile, outputFile *File, depPaths []string) error {
	if len(c.baseDir) == 0 {
		return nil
	}

	dataPath, entryPath := c.buildCachePaths(context, inputFile)

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileEntry(entryPath, inputFile, depPaths); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileData(path string, f *File) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := f.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileEntry(path string, file *File, depPaths []string) error {
	entry := cacheEntry{
		Meta:     file.Meta,
		RelPath:  file.Path(),
		DepPaths: depPaths,
	}

	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, json, 0666)
}
