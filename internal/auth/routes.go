package auth

import "net/http"

func RegisterRoutes(mux *http.ServeMux, handler *Handler, middleware *Middleware) {
	mux.HandleFunc("/register", handler.Register)
	mux.HandleFunc("/login", handler.Login)
	mux.HandleFunc("/verify-email", handler.VerifyEmail)
	mux.HandleFunc("/forgot-password", handler.ForgotPassword)
	mux.HandleFunc("/reset-password", handler.ResetPassword)

	mux.Handle("/logout", middleware.RequireAuth(http.HandlerFunc(handler.Logout)))
	mux.Handle(
		"/verify-email/pending",
		middleware.RequireAuth(http.HandlerFunc(handler.VerificationPending)),
	)
	mux.Handle(
		"/verify-email/resend",
		middleware.RequireAuth(http.HandlerFunc(handler.ResendVerification)),
	)
	mux.Handle(
		"/account/pending",
		middleware.RequireVerified(http.HandlerFunc(handler.AccountPending)),
	)
}
