package centralauth

import "net/http"

// CallbackHandler returns an http.HandlerFunc that extracts the "code" query
// parameter from the callback request, exchanges it for user info, and routes
// to the appropriate callback.
func CallbackHandler(
	client *Client,
	onSuccess func(user *UserInfo, w http.ResponseWriter, r *http.Request),
	onError func(err error, w http.ResponseWriter, r *http.Request),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			onError(&Error{Message: "missing code parameter", StatusCode: 400}, w, r)
			return
		}

		user, err := client.Exchange(r.Context(), code)
		if err != nil {
			onError(err, w, r)
			return
		}

		onSuccess(user, w, r)
	}
}
