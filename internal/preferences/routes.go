package preferences

import "net/http"

func Routes(handler *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Home)
	mux.HandleFunc("/setup", handler.Setup)
	mux.HandleFunc("/settings/profile", handler.ProfileSettings)
	mux.HandleFunc("/settings/profile/edit", handler.ProfileEdit)
	mux.HandleFunc("/settings/profile/reset", handler.ProfileReset)
	mux.HandleFunc("/settings/digest", handler.DigestSettings)
	mux.HandleFunc("/settings/digest/reset", handler.DigestReset)
	mux.HandleFunc("/run-once", handler.RunOnce)
	mux.Handle("/static/", http.HandlerFunc(handler.Static))
	return mux
}
