package frontend

import (
	"embed"
	"io/fs"

	"github.com/uuuuupdate/gh-proxy-auth/internal/handlers"
)

//go:embed all:dist
var distFS embed.FS

func Init() error {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return err
	}
	handlers.SetFrontendFS(subFS)
	return nil
}
