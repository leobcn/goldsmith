package goldsmith

import "path/filepath"

type loader struct{}

func (*loader) Name() string {
	return "loader"
}

func (*loader) Initialize(ctx *Context) ([]Filter, error) {
	infos := make(chan fileInfo)
	go scanDir(ctx.gs.sourceDir, infos)

	for info := range infos {
		if info.IsDir() {
			continue
		}

		relPath, _ := filepath.Rel(ctx.gs.sourceDir, info.path)

		file := &File{
			sourcePath: relPath,
			Meta:       make(map[string]interface{}),
			modTime:    info.ModTime(),
			size:       info.Size(),
			dataPath:   info.path,
		}

		ctx.DispatchFile(file, false)
	}

	return nil, nil
}
