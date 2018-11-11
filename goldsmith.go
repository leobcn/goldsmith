package goldsmith

import (
	"os"
	"path/filepath"
	"sync"
)

type Goldsmith struct {
	sourceDir string
	targetDir string

	pluginCtxs []*Context

	fileRefs    map[string]bool
	fileFilters []Filter
	fileCache   *cache

	errors   []error
	errorMtx sync.Mutex
}

func Begin(srcDir, cacheDir string) *Goldsmith {
	gs := &Goldsmith{sourceDir: srcDir, fileCache: &cache{cacheDir}, fileRefs: make(map[string]bool)}
	gs.Chain(new(loader))
	return gs
}

func (gs *Goldsmith) Chain(plugin Plugin) *Goldsmith {
	gs.linkPlugin(plugin)
	return gs
}

func (gs *Goldsmith) FilterPush(filter Filter) *Goldsmith {
	gs.fileFilters = append(gs.fileFilters, filter)
	return gs
}

func (gs *Goldsmith) FilterPop() *Goldsmith {
	count := len(gs.fileFilters)
	if count == 0 {
		panic("attempted to pop empty filter stack")
	}

	gs.fileFilters = gs.fileFilters[:count-1]
	return gs
}

func (gs *Goldsmith) End(targetDir string) []error {
	gs.targetDir = targetDir

	for _, ctx := range gs.pluginCtxs {
		go ctx.step()
	}

	ctx := gs.pluginCtxs[len(gs.pluginCtxs)-1]
	for file := range ctx.outputFiles {
		gs.exportFile(file)
	}

	gs.cleanupFiles()
	return gs.errors
}

func (gs *Goldsmith) linkPlugin(plug Plugin) *Context {
	ctx := &Context{chain: gs, plugin: plug, outputFiles: make(chan *File)}
	ctx.fileFilters = append(ctx.fileFilters, gs.fileFilters...)

	if len(gs.pluginCtxs) > 0 {
		ctx.inputFiles = gs.pluginCtxs[len(gs.pluginCtxs)-1].outputFiles
	}

	gs.pluginCtxs = append(gs.pluginCtxs, ctx)
	return ctx
}

func (gs *Goldsmith) cleanupFiles() {
	infos := make(chan fileInfo)
	go scanDir(gs.targetDir, infos)

	for info := range infos {
		relPath, _ := filepath.Rel(gs.targetDir, info.path)
		if contained, _ := gs.fileRefs[relPath]; !contained {
			os.RemoveAll(info.path)
		}
	}
}

func (gs *Goldsmith) exportFile(file *File) error {
	if err := file.export(gs.targetDir); err != nil {
		return err
	}

	for pathSeg := cleanPath(file.relPath); pathSeg != "."; pathSeg = filepath.Dir(pathSeg) {
		gs.fileRefs[pathSeg] = true
	}

	return nil
}

func (gs *Goldsmith) cacheWriteFile(pluginName string, inputFile, outputFile *File, depPaths []string) error {
	return gs.fileCache.writeFile(pluginName, inputFile, outputFile, depPaths)
}

func (gs *Goldsmith) cacheReadFile(pluginName string, inputFile *File) (*File, error) {
	return gs.fileCache.readFile(pluginName, inputFile)
}

func (gs *Goldsmith) fault(pluginName string, file *File, err error) {
	gs.errorMtx.Lock()
	defer gs.errorMtx.Unlock()

	faultError := &Error{Name: pluginName, Err: err}
	if file != nil {
		faultError.Path = file.relPath
	}

	gs.errors = append(gs.errors, faultError)
}
