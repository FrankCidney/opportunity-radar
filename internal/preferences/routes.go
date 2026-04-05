package preferences

import "net/http"

func Routes(handler *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Home)
	mux.HandleFunc("/setup", handler.Setup)
	mux.HandleFunc("/settings/profile", handler.ProfileSettings)
	mux.HandleFunc("/settings/profile/edit", handler.ProfileEdit)
	mux.HandleFunc("/settings/digest", handler.DigestSettings)
	mux.HandleFunc("/run-once", handler.RunOnce)
	mux.Handle("/static/", http.HandlerFunc(handler.Static))
	return mux
}
