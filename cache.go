package goldsmith

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
)

type cache struct {
	baseDir string
}

func (c *cache) buildCachePaths(name, path string) (string, string, string) {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(path))
	sum := h.Sum32()

	ext := filepath.Ext(path)

	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_data%s", sum, ext))
	metaPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_meta.json", sum))
	depsPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_deps.txt", sum))

	return dataPath, metaPath, depsPath
}

func (c *cache) readFile(pluginName string, inputFile *File) (*File, error) {
	if len(c.baseDir) == 0 {
		return nil, nil
	}

	dataPath, metaPath, depsPath := c.buildCachePaths(pluginName, inputFile.Path())

	depPaths, _ := c.readFileDeps(depsPath)
	depPaths = append(depPaths, dataPath, metaPath)

	if modTime, err := newestFile(depPaths); err != nil || inputFile.ModTime().After(modTime) {
		return nil, err
	}

	_, err := c.readFileMeta(metaPath)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *cache) readFileDeps(path string) ([]string, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	var depPaths []string
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		depPaths = append(depPaths, scanner.Text())
	}

	return depPaths, scanner.Err()
}

func (c *cache) readFileMeta(path string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]interface{})
	if err := json.Unmarshal(data, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func (c *cache) writeFile(pluginName string, inputFile, outputFile *File, depPaths []string) error {
	if len(c.baseDir) == 0 {
		return nil
	}

	dataPath, metaPath, depsPath := c.buildCachePaths(pluginName, inputFile.Path())

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileMeta(metaPath, outputFile); err != nil {
		return err
	}

	if len(depPaths) > 0 {
		if err := c.writeFileDeps(depsPath, depPaths); err != nil {
			return err
		}
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

func (c *cache) writeFileMeta(path string, f *File) error {
	json, err := json.Marshal(f.Meta)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, json, 0666)
}

func (c *cache) writeFileDeps(path string, depPaths []string) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	for _, dep := range depPaths {
		if _, err := fp.WriteString(fmt.Sprintln(dep)); err != nil {
			return err
		}
	}

	return nil
}
