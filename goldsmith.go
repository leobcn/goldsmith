package goldsmith

import (
	"os"
	"path/filepath"
	"sync"
)

type Goldsmith struct {
	srcDir string
	dstDir string

	links    []*Context
	refs     map[string]bool
	filters  []Filter
	cache    *cache
	complete bool

	errors   []error
	errorMtx sync.Mutex
}

func Begin(srcDir string) *Goldsmith {
	gs := &Goldsmith{srcDir: srcDir, refs: make(map[string]bool)}
	gs.Chain(new(loader))
	return gs
}

func BeginCached(srcDir, cacheDir string) *Goldsmith {
	gs := &Goldsmith{srcDir: srcDir, cache: &cache{cacheDir}, refs: make(map[string]bool)}
	gs.Chain(new(loader))
	return gs
}

func (c *Goldsmith) Chain(p Plugin) *Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.linkPlugin(p)
	return c
}

func (c *Goldsmith) FilterPush(f Filter) *Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.filters = append(c.filters, f)
	return c
}

func (c *Goldsmith) FilterPop() *Goldsmith {
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

func (c *Goldsmith) End(dstDir string) []error {
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

func (c *Goldsmith) linkPlugin(plug Plugin) *Context {
	ctx := &Context{chain: c, plugin: plug, output: make(chan *File)}
	ctx.filters = append(ctx.filters, c.filters...)

	if len(c.links) > 0 {
		ctx.input = c.links[len(c.links)-1].output
	}

	c.links = append(c.links, ctx)
	return ctx
}

func (c *Goldsmith) cleanupFiles() {
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

func (c *Goldsmith) exportFile(f *File) error {
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

func (c *Goldsmith) fault(name string, f *File, err error) {
	c.errorMtx.Lock()
	defer c.errorMtx.Unlock()

	ferr := &Error{Name: name, Err: err}
	if f != nil {
		ferr.Path = f.path
	}

	c.errors = append(c.errors, ferr)
}

func (c *Goldsmith) cacheWriteFile(pluginName string, inputFile, outputFile *File, depPaths []string) error {
	return c.cache.writeFile(pluginName, inputFile, outputFile, depPaths)
}

func (c *Goldsmith) cacheReadFile(pluginName string, inputFile *File) (*File, error) {
	outputFile, err := c.cache.readFile(pluginName, inputFile)
	if err != nil {
		return nil, err
	}

	return outputFile, nil
}
