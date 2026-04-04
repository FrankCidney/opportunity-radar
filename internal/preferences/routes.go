package preferences

import "net/http"

func Routes(handler *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Home)
	mux.HandleFunc("/setup", handler.Setup)
	mux.HandleFunc("/settings/profile", handler.ProfileSettings)
	mux.HandleFunc("/settings/digest", handler.DigestSettings)
	return mux
}
