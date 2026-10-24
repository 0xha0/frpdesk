package controller

import (
	"crypto/subtle"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// equal compares two secrets in constant time
func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// BasicAuth lets a request through when its session holds the admin's name (the session never holds
// the password: it was checked once, at login). The server refuses to start without admin_user and
// admin_pwd, so there is no open mode.
func (c *HandleController) BasicAuth() gin.HandlerFunc {
	return func(context *gin.Context) {
		session := sessions.Default(context)
		if user, ok := session.Get(AuthName).(string); ok && equal(user, c.CommonInfo.AdminUser) {
			if c.CommonInfo.AdminKeepTime > 0 {
				cookie, _ := context.Request.Cookie(SessionName)
				if cookie != nil {
					//important thx https://blog.csdn.net/zhanghongxia8285/article/details/107321838/
					cookie.Expires = time.Now().Add(time.Second * time.Duration(c.CommonInfo.AdminKeepTime))
					http.SetCookie(context.Writer, cookie)
				}
			}
			context.Next()
			return
		}

		isAjax := context.GetHeader("X-Requested-With") == "XMLHttpRequest"

		if !isAjax && context.Request.RequestURI != LoginUrl {
			context.Redirect(http.StatusTemporaryRedirect, LoginUrl)
		} else {
			context.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}

// LoginAuth: true when the session is logged in already, or when username and password match (then
// the session records the admin's name)
func (c *HandleController) LoginAuth(username, password string, context *gin.Context) bool {
	session := sessions.Default(context)
	if user, ok := session.Get(AuthName).(string); ok && equal(user, c.CommonInfo.AdminUser) {
		return true
	}
	// both compared, whatever the first one says (no early exit to time)
	userOK := equal(username, c.CommonInfo.AdminUser)
	pwdOK := equal(password, c.CommonInfo.AdminPwd)
	if userOK && pwdOK {
		session.Set(AuthName, c.CommonInfo.AdminUser)
		_ = session.Save()
		return true
	}
	session.Delete(AuthName)
	_ = session.Save()
	return false
}

func ClearAuth(context *gin.Context) {
	session := sessions.Default(context)
	session.Delete(AuthName)
	_ = session.Save()
}
