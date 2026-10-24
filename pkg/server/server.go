package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"frps-panel/pkg/server/controller"
	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	cfg     controller.HandleController
	servers []*http.Server // the plugin's, and the UI's when it has its own listener
	tls     TLS
	done    chan struct{}
	rootDir string
}

type TLS struct {
	Enable   bool
	Cert     string
	Key      string
	Protocol string
}

func New(rootDir string, cfg controller.HandleController, tls TLS) (*Server, error) {
	s := &Server{
		cfg:     cfg,
		done:    make(chan struct{}),
		rootDir: rootDir,
		tls:     tls,
	}

	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

// serve one listener; TLS (tls_mode) only for the UI's
func (s *Server) serve(srv *http.Server, addr string, withTLS bool, what string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	protocol := "HTTP"
	if withTLS {
		protocol = s.tls.Protocol
	}
	log.Printf("%s: %s server listen on %s", what, protocol, l.Addr().String())
	go func() {
		if withTLS && s.tls.Enable {
			configDir := filepath.Dir(s.cfg.ConfigFile)

			cert := filepath.Join(configDir, s.tls.Cert)
			_, err := os.Stat(cert)
			if err != nil && !os.IsExist(err) {
				cert = s.tls.Cert
			}

			key := filepath.Join(configDir, s.tls.Key)
			_, err = os.Stat(key)
			if err != nil && !os.IsExist(err) {
				key = s.tls.Key
			}

			err = srv.ServeTLS(l, cert, key)
			if !errors.Is(err, http.ErrServerClosed) {
				log.Printf("error shutdown %s server: %v", what, err)
				_ = s.Stop()
			}
		} else {
			err = srv.Serve(l)
			if !errors.Is(err, http.ErrServerClosed) {
				log.Printf("error shutdown %s server: %v", what, err)
				_ = s.Stop()
			}
		}
	}()
	return nil
}

func (s *Server) Run() error {
	common := s.cfg.CommonInfo
	pluginAddr := net.JoinHostPort(common.PluginAddr, strconv.Itoa(common.PluginPort))
	if len(s.servers) == 1 {
		// one listener for both, as before
		if err := s.serve(s.servers[0], pluginAddr, true, "plugin + UI"); err != nil {
			return err
		}
	} else {
		if err := s.serve(s.servers[0], pluginAddr, false, "plugin"); err != nil {
			return err
		}
		uiAddr := net.JoinHostPort(common.UIAddr, strconv.Itoa(common.UIPort))
		if err := s.serve(s.servers[1], uiAddr, true, "UI"); err != nil {
			return err
		}
	}
	<-s.done
	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range s.servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("shutdown server error: %v", err)
		}
	}
	log.Printf("servers exited")

	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}

func (s *Server) init() error {
	if err := s.initHTTPServer(); err != nil {
		log.Printf("init server error: %v", err)
		return err
	}
	return nil
}

func LoadSupportLanguage(dir string) ([]language.Tag, error) {
	var tags []language.Tag

	files, err := os.Open(dir)

	fileList, err := files.Readdir(-1)
	if err != nil {
		log.Printf("error read lang directory: %v", err)
		return tags, err
	}

	err = files.Close()
	if err != nil {
		return nil, err
	}

	for _, file := range fileList {
		name, _ := strings.CutSuffix(file.Name(), ".json")
		parsedLang, _ := language.Parse(name)
		tags = append(tags, parsedLang)
	}

	if len(tags) == 0 {
		return tags, fmt.Errorf("not found any language file in directory: %v", dir)
	}

	return tags, nil
}

func GinI18nLocalize(rootDir string) gin.HandlerFunc {
	assets := filepath.Join(rootDir, "assets")
	_, err := os.Stat(assets)
	if err != nil && !os.IsExist(err) {
		assets = "./assets"
	}
	lang := filepath.Join(assets, "lang")
	tags, err := LoadSupportLanguage(lang)
	if err != nil {
		log.Panicf("language file is not found: %v", err)
	}

	return ginI18n.Localize(
		ginI18n.WithBundle(&ginI18n.BundleCfg{
			RootPath:         lang,
			AcceptLanguage:   tags,
			DefaultLanguage:  language.Chinese,
			FormatBundleFile: "json",
			UnmarshalFunc:    json.Unmarshal,
		}),
		ginI18n.WithGetLngHandle(
			func(context *gin.Context, defaultLng string) string {
				header := context.GetHeader("Accept-Language")
				lang, _, err := language.ParseAcceptLanguage(header)
				if err != nil || len(lang) == 0 { // no Accept-Language (curl, scripts): the default
					return defaultLng
				}
				return lang[0].String()
			},
		),
	)
}

func (s *Server) initHTTPServer() error {
	gin.SetMode(gin.ReleaseMode)
	common := s.cfg.CommonInfo
	if strings.TrimSpace(common.AdminUser) == "" || strings.TrimSpace(common.AdminPwd) == "" {
		return errors.New("admin_user and admin_pwd are required: the UI has no open mode")
	}
	ctl := controller.NewHandleController(&s.cfg)

	// the session cookie: signed and encrypted with keys made at every start (a restart logs everyone
	// out); it holds the admin's name only
	authKey, encKey := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(authKey); err != nil {
		return err
	}
	if _, err := rand.Read(encKey); err != nil {
		return err
	}
	authStore := cookie.NewStore(authKey, encKey)
	authStore.Options(sessions.Options{
		Secure:   common.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   common.AdminKeepTime,
	})
	ui := gin.New()
	ui.Use(gin.Recovery())
	ui.Use(sessions.Sessions(controller.SessionName, authStore))
	ui.Use(GinI18nLocalize(s.rootDir))
	ctl.RegisterUI(s.rootDir, ui)

	if common.UIPort == 0 {
		// one listener: the plugin's endpoint on the UI's engine, as before
		ctl.RegisterPlugin(ui)
		s.servers = []*http.Server{{Handler: ui}}
		return nil
	}
	plugin := gin.New()
	plugin.Use(gin.Recovery())
	ctl.RegisterPlugin(plugin)
	s.servers = []*http.Server{{Handler: plugin}, {Handler: ui}}
	return nil
}
