package goldsmith

import (
	"sync"
)

type Goldsmith struct {
	srcDir string
	dstDir string

	links    []*link
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

type Context interface {
	DispatchFile(f *File)
	CacheFile(inputFile, outputFile *File, depPaths ...string)

	SrcDir() string
	DstDir() string
}
