package goldsmith

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
)

type chain struct {
	srcDir   string
	dstDir   string
	cacheDir string

	links    []*link
	refs     map[string]bool
	filters  []Filter
	complete bool

	errors   []error
	errorMtx sync.Mutex
}

func (c *chain) linkPlugin(plug Plugin) *link {
	ctx := &link{chain: c, plugin: plug, output: make(chan *file)}
	ctx.filters = append(ctx.filters, c.filters...)

	if len(c.links) > 0 {
		ctx.input = c.links[len(c.links)-1].output
	}

	c.links = append(c.links, ctx)
	return ctx
}

func (c *chain) cleanupFiles() {
	infos := make(chan fileInfo)
	go scanDir(c.dstDir, infos)

	for info := range infos {
		relPath, _ := filepath.Rel(c.dstDir, info.path)
		if contained, _ := c.refs[relPath]; contained {
			continue
		}

		os.RemoveAll(info.path)
	}
}

func (c *chain) exportFile(f *file) error {
	if err := f.export(c.dstDir); err != nil {
		return err
	}

	pathSeg := cleanPath(f.path)
	for {
		c.refs[pathSeg] = true
		if pathSeg == "." {
			break
		}

		pathSeg = filepath.Dir(pathSeg)
	}

	return nil
}

func (c *chain) fault(name string, f *file, err error) {
	c.errorMtx.Lock()
	defer c.errorMtx.Unlock()

	ferr := &Error{Name: name, Err: err}
	if f != nil {
		ferr.Path = f.path
	}

	c.errors = append(c.errors, ferr)
}

func (c *chain) buildCachePaths(name, path string) (dataPath, metaPath, depsPath string) {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(path))
	sum := h.Sum32()

	ext := filepath.Ext(path)

	dataPath = filepath.Join(c.cacheDir, fmt.Sprintf("gs_%.8x_data%s", sum, ext))
	metaPath = filepath.Join(c.cacheDir, fmt.Sprintf("gs_%.8x_meta.json", sum))
	depsPath = filepath.Join(c.cacheDir, fmt.Sprintf("gs_%.8x_deps.txt", sum))
	return
}

func (c *chain) cacheFile(name string, f *file, deps []string) error {
	if len(deps) == 0 {
		panic("cached files must have one or more dependencies")
	}

	if len(c.cacheDir) > 0 {
		dataPath, metaPath, depsPath := c.buildCachePaths(name, f.Path())
		if err := c.cacheFileData(dataPath, f); err != nil {
			return err
		}
		if err := c.cacheFileMeta(metaPath, f); err != nil {
			return err
		}
		if err := c.cacheFileDeps(depsPath, deps); err != nil {
			return err
		}
	}

	return nil
}

func (c *chain) cacheFileData(path string, f *file) error {
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

func (c *chain) cacheFileMeta(path string, f *file) error {
	metaJson, err := json.Marshal(f.Meta)
	if err != nil {
		return err
	}

	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := fp.Write(metaJson); err != nil {
		return err
	}

	return nil
}

func (c *chain) cacheFileDeps(path string, deps []string) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	for _, dep := range deps {
		if _, err := fp.WriteString(fmt.Sprintln(dep)); err != nil {
			return err
		}
	}

	return nil
}

//
//	Goldsmith Implementation
//

func (c *chain) Chain(p Plugin) Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.linkPlugin(p)
	return c
}

func (c *chain) FilterPush(f Filter) Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.filters = append(c.filters, f)
	return c
}

func (c *chain) FilterPop() Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	count := len(c.filters)
	if count == 0 {
		panic("attempted to pop empty filter stack")
	}

	c.filters = c.filters[:count-1]
	return c
}

func (c *chain) End(dstDir string) []error {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.dstDir = dstDir

	for _, ctx := range c.links {
		go ctx.step()
	}

	ctx := c.links[len(c.links)-1]
	for f := range ctx.output {
		c.exportFile(f)
	}

	c.cleanupFiles()
	c.complete = true

	return c.errors
}
