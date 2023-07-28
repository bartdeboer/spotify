package main

import (
	"context"
	"fmt"
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
		case "create-clean-all":
			err := createCleanAllPlaylists()
			if err != nil {
				fmt.Println("Error creating clean playlists:", err)
			} else {
				fmt.Println("All playlists cleaned successfully.")
			}
		default:
			fmt.Println("Invalid argument for 'playlists' command.")
		}
	default:
		fmt.Println("Invalid command.")
	}
}

func shutdown(server *http.Server, ctx context.Context) error {
	return server.Shutdown(ctx)
}

type FullTrack struct {
	SimpleTrack

	AddedAt string `json:"added_at"`
	AddedBy struct {
		DisplayName  string `json:"display_name"`
		ExternalURLs struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Followers struct {
			Total int    `json:"total"`
			Href  string `json:"href"`
		} `json:"followers"`
		Href   string `json:"href"`
		ID     string `json:"id"`
		Images []struct {
			Height int    `json:"height"`
			Width  int    `json:"width"`
			URL    string `json:"url"`
		} `json:"images"`
		URI string `json:"uri"`
	} `json:"added_by"`
	IsLocal bool `json:"is_local"`
	Track   struct {
		Artists          []Artist `json:"artists"`
		AvailableMarkets []string `json:"available_markets"`
		DiscNumber       int      `json:"disc_number"`
		DurationMS       int      `json:"duration_ms"`
		Explicit         bool     `json:"explicit"`
		ExternalURLs     struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Href        string `json:"href"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		PreviewURL  string `json:"preview_url"`
		TrackNumber int    `json:"track_number"`
		URI         string `json:"uri"`
		Type        string `json:"type"`
		Album       struct {
			Name             string   `json:"name"`
			Artists          []Artist `json:"artists"`
			AlbumGroup       string   `json:"album_group"`
			AlbumType        string   `json:"album_type"`
			ID               string   `json:"id"`
			URI              string   `json:"uri"`
			AvailableMarkets []string `json:"available_markets"`
			Href             string   `json:"href"`
			Images           []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				URL    string `json:"url"`
			} `json:"images"`
			ExternalURLs struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			ReleaseDate          string `json:"release_date"`
			ReleaseDatePrecision string `json:"release_date_precision"`
		} `json:"album"`
		ExternalIDs struct {
			ISRC string `json:"isrc"`
		} `json:"external_ids"`
		Popularity int         `json:"popularity"`
		IsPlayable interface{} `json:"is_playable"`
		LinkedFrom interface{} `json:"linked_from"`
	} `json:"track"`
}

type SimpleTrack struct {
	Name        string   `json:"name"`
	TrackNumber int      `json:"track_number"`
	Artists     []Artist `json:"artists"`
	Album       struct {
		Name        string   `json:"name"`
		Artists     []Artist `json:"artists"`
		ReleaseDate string   `json:"release_date"`
	} `json:"album"`
}

type Artist struct {
	Name string `json:"name"`
}

type CleanedPlaylistItem struct {
	Track        string `json:"track"`
	TrackNumber  int    `json:"track_number"`
	TrackArtists string `json:"track_artists"`
	Album        string `json:"album"`
	AlbumArtists string `json:"album_artists"`
	ReleaseDate  string `json:"release_date"`
}

func getArtistsNames(artists []Artist) string {
	var names []string
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	return strings.Join(names, ", ")
}
