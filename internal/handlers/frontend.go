package handlers

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var frontendFS http.FileSystem

func SetFrontendFS(fsys fs.FS) {
	frontendFS = http.FS(fsys)
}

func IsProxyPath(path string) bool {
	return checkURL(path) || checkURL("https://"+path)
}

func ServeFrontend(c *gin.Context) {
	if frontendFS == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Frontend not available"})
		return
	}

	path := c.Request.URL.Path

	// Try to serve static files.
	// Exclude "/" and paths ending with "/" (directories), and also exclude "/index.html"
	// to avoid http.FileServer's built-in redirect of /index.html → ./ which causes an
	// infinite redirect loop when running behind a reverse proxy such as Caddy.
	if path != "/" && path != "/index.html" && !strings.HasSuffix(path, "/") {
		file, err := frontendFS.Open(path)
		if err == nil {
			defer file.Close()
			stat, err := file.Stat()
			if err == nil && !stat.IsDir() {
				http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), file)
				return
			}
		}
	}

	// For SPA routing (and /index.html), serve index.html directly using
	// http.ServeContent to avoid the redirect loop that http.FileServer introduces
	// when the request path ends with "/index.html".
	file, err := frontendFS.Open("/index.html")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Frontend not available"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file)
}
