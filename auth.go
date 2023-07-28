package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

func completeAuth(w http.ResponseWriter, r *http.Request, auth *spotifyauth.Authenticator, ctx context.Context) {
	token, err := auth.Token(context.Background(), state, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Couldn't get token: %v", err), http.StatusForbidden)
		fmt.Printf("Error: %v\n", err)
		return
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		fmt.Printf("State mismatch: %s != %s\n", st, state)
		return
	}

	client := spotify.New(auth.Client(ctx, token))
	fmt.Fprintf(w, "Login completed! You can close this window.")
	ch <- &ClientToken{Client: client, Token: token}
}
