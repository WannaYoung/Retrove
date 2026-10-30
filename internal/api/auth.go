package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"retrove/internal/auth"
)

type authBody struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}

func (s *Server) authStatus(w http.ResponseWriter, _ *http.Request) {
	has, err := s.auth.HasUser()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "查询用户失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"has_user": has})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var body authBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	user, token, err := s.auth.Register(auth.RegisterInput{
		Username:        body.Username,
		Email:           body.Email,
		Password:        body.Password,
		PasswordConfirm: body.PasswordConfirm,
	})
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body authBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	user, token, err := s.auth.Login(body.Username, body.Password)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.CookieName())
	if err == nil {
		_ = s.auth.Logout(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "未登录")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) currentUser(r *http.Request) (auth.User, bool) {
	cookie, err := r.Cookie(auth.CookieName())
	if err != nil {
		return auth.User{}, false
	}
	user, err := s.auth.UserByToken(cookie.Value)
	return user, err == nil
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(14 * 24 * time.Hour),
	})
}

func writeAuthErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrValidation):
		writeError(w, http.StatusBadRequest, "validation", "请填写用户名、有效邮箱，密码至少 8 位且两次输入一致")
	case errors.Is(err, auth.ErrUserExists):
		writeError(w, http.StatusConflict, "user_exists", "已有用户，不能再注册")
	case errors.Is(err, auth.ErrCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "登录处理失败")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
