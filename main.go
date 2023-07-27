package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const (
	redirectURI    = "http://127.0.0.1:8080/callback"
	state          = "abc123"
	configFilename = "config.json"
	tokenFilename  = "token.json"
)

var (
	// auth *spotifyauth.Authenticator
	// ch = make(chan *spotify.Client)
	ch = make(chan *ClientToken)
)

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type ClientToken struct {
	Client *spotify.Client
	Token  *oauth2.Token
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: spotify.exe <command> <argument>")
		return
	}

	command := os.Args[1]
	arg := os.Args[2]

	// Read the config file
	config := Config{}
	if err := readJSONFromFile(configFilename, &config); err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}

	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(spotifyauth.ScopePlaylistReadPrivate, spotifyauth.ScopeUserLibraryRead),
		spotifyauth.WithClientID(config.ClientID),
		spotifyauth.WithClientSecret(config.ClientSecret),
	)

	token := &oauth2.Token{}
	err := readJSONFromFile(tokenFilename, token)

	httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	client := spotify.New(httpClient)

	ctx := context.Background()

	if err == nil && token.Valid() {
		client = spotify.New(auth.Client(ctx, token))
	} else {
		http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			completeAuth(w, r, auth, ctx)
		})

		server := &http.Server{Addr: ":8080"}
		go func() {
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				fmt.Println("ListenAndServe():", err)
			}
		}()

		url := auth.AuthURL(state)
		fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

		clientToken := <-ch
		token = clientToken.Token
		shutdown(server, ctx)

		if err == nil {
			err = writeJSONToFile(tokenFilename, token)
			if err != nil {
				fmt.Println("Error saving token to file:", err)
			}
		} else {
			fmt.Println("Error obtaining token:", err)
			return
		}
	}

	switch command {
	case "playlists":
		switch arg {
		case "download":
			if len(os.Args) < 4 {
				fmt.Println("Usage: spotify.exe playlists download <playlist_name>")
				return
			}
			playlistName := os.Args[3]
			err := downloadPlaylist(ctx, client, playlistName)
			if err != nil {
				fmt.Println("Error downloading playlist:", err)
			} else {
				fmt.Println("Playlist downloaded successfully.")
			}
		case "list":
			err := listPlaylists(ctx, client)
			if err != nil {
				fmt.Println("Error listing playlists:", err)
			}
		case "download-all":
			err := downloadPlaylists(ctx, client)
			if err != nil {
				fmt.Println("Error downloading playlists:", err)
			} else {
				fmt.Println("All playlists downloaded successfully.")
			}
		default:
			fmt.Println("Invalid argument for 'playlists' command.")
		}
	default:
		fmt.Println("Invalid command.")
	}
}

func readJSONFromFile(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSONToFile(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0600)
}

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

	// Use the token to get an authenticated client
	client := spotify.New(auth.Client(ctx, token))
	fmt.Fprintf(w, "Login completed! You can close this window.")
	ch <- &ClientToken{Client: client, Token: token}
}

func downloadPlaylist(ctx context.Context, client *spotify.Client, playlistName string) error {
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}

	playlists, err := client.GetPlaylistsForUser(ctx, user.ID)
	if err != nil {
		return err
	}

	var playlistID spotify.ID
	found := false
	for _, playlist := range playlists.Playlists {
		if strings.EqualFold(playlist.Name, playlistName) {
			playlistID = playlist.ID
			found = true
			break
		}
	}

	if !found {
		return errors.New("playlist not found")
	}

	var allTracks []spotify.PlaylistTrack
	offset := 0
	limit := 100

	for {
		playlistTracks, err := client.GetPlaylistTracks(ctx, playlistID, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return err
		}

		allTracks = append(allTracks, playlistTracks.Tracks...)

		if len(allTracks) >= playlistTracks.Total {
			break
		}

		offset += limit
	}

	playlistFile, err := os.Create(fmt.Sprintf("downloads/%s.json", playlistName))
	if err != nil {
		return err
	}
	defer playlistFile.Close()

	encoder := json.NewEncoder(playlistFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(allTracks)
}

func listPlaylists(ctx context.Context, client *spotify.Client) error {
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}

	playlists, err := client.GetPlaylistsForUser(ctx, user.ID)
	if err != nil {
		return err
	}

	fmt.Println("Playlists:")
	for _, playlist := range playlists.Playlists {
		fmt.Printf("- %s (ID: %s)\n", playlist.Name, playlist.ID)
	}

	return nil
}

func shutdown(server *http.Server, ctx context.Context) error {
	return server.Shutdown(ctx)
}

func downloadPlaylists(ctx context.Context, client *spotify.Client) error {
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}

	playlists, err := client.GetPlaylistsForUser(ctx, user.ID)
	if err != nil {
		return err
	}

	for _, playlist := range playlists.Playlists {
		err := downloadPlaylist(ctx, client, playlist.Name)
		if err != nil {
			fmt.Println("Error downloading playlist:", err)
		} else {
			fmt.Println("Playlist downloaded successfully:", playlist.Name)
		}
	}

	return nil
}
