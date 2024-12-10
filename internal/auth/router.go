package auth

import "net/http"

type AuthController interface {
	Login(w http.ResponseWriter, r *http.Request)
	Refresh(w http.ResponseWriter, r *http.Request)
}

func newRouter(
	authController AuthController,
) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", authController.Login)
	mux.HandleFunc("POST /refresh", authController.Refresh)
	return mux
}
