package goldsmith

import (
	"fmt"
	"io"
	"time"
)

type Goldsmith interface {
	Chain(p Plugin) Goldsmith
	FilterPush(f Filter) Goldsmith
	FilterPop() Goldsmith
	End(dstDir string) []error
}

func Begin(srcDir string) Goldsmith {
	gs := &chain{srcDir: srcDir, refs: make(map[string]bool)}
	gs.Chain(new(loader))
	return gs
}

func BeginCached(srcDir, cacheDir string) Goldsmith {
	gs := &chain{srcDir: srcDir, cache: &cache{cacheDir}, refs: make(map[string]bool)}
	gs.Chain(new(loader))
	return gs
}

type File interface {
	Path() string
	Name() string
	Dir() string
	Ext() string
	Size() int64
	ModTime() time.Time

	Value(key string) (interface{}, bool)
	SetValue(key string, value interface{})
	InheritValues(src File)

	Read(p []byte) (int, error)
	WriteTo(w io.Writer) (int64, error)
	Seek(offset int64, whence int) (int64, error)
}

type Context interface {
	DispatchFile(f File)
	CacheFile(inputFile, outputFile File, depPaths ...string)

	SrcDir() string
	DstDir() string
}

type Error struct {
	Name string
	Path string
	Err  error
}

func (e Error) Error() string {
	var path string
	if len(e.Path) > 0 {
		path = "@" + e.Path
	}

	return fmt.Sprintf("[%s%s]: %s", e.Name, path, e.Err.Error())
}

type Initializer interface {
	Initialize(ctx Context) ([]Filter, error)
}

type Processor interface {
	Process(ctx Context, f File) error
}

type Finalizer interface {
	Finalize(ctx Context) error
}

type Component interface {
	Name() string
}

type Filter interface {
	Component
	Accept(ctx Context, f File) (bool, error)
}

type Plugin interface {
	Component
}
