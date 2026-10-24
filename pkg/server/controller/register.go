package controller

import (
	"github.com/gin-gonic/gin"
	"os"
	"path/filepath"
)

type HandleController struct {
	CommonInfo CommonInfo
	Tokens     map[string]TokenInfo
	Version    string
	ConfigFile string
	TokensFile string
}

func NewHandleController(config *HandleController) *HandleController {
	return config
}

// RegisterPlugin: frps's plugin endpoint, POST /handler (no login: frps calls it; keep its listener
// where only frps reaches it)
func (c *HandleController) RegisterPlugin(engine *gin.Engine) {
	engine.POST("/handler", c.MakeHandlerFunc())
}

// RegisterUI: the web UI, every page but the static files and the login behind the admin's session
func (c *HandleController) RegisterUI(rootDir string, engine *gin.Engine) {
	assets := filepath.Join(rootDir, "assets")
	_, err := os.Stat(assets)
	if err != nil && !os.IsExist(err) {
		assets = "./assets"
	}

	engine.Delims("${", "}")
	engine.LoadHTMLGlob(filepath.Join(assets, "templates/*"))
	engine.Static("/static", filepath.Join(assets, "static"))
	engine.GET("/lang.json", c.MakeLangFunc())
	engine.GET(LoginUrl, c.MakeLoginFunc())
	engine.POST(LoginUrl, c.MakeLoginFunc())
	engine.GET(LogoutUrl, c.MakeLogoutFunc())

	group := engine.Group("/", c.BasicAuth())
	group.GET("/", c.MakeIndexFunc())
	group.GET("/tokens", c.MakeQueryTokensFunc())
	group.POST("/add", c.MakeAddTokenFunc())
	group.POST("/update", c.MakeUpdateTokensFunc())
	group.POST("/remove", c.MakeRemoveTokensFunc())
	group.POST("/disable", c.MakeDisableTokensFunc())
	group.POST("/enable", c.MakeEnableTokensFunc())
	group.GET("/proxy/*serverApi", c.MakeProxyFunc())
}
