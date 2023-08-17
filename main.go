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
	command, arg, err := parseArgs()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Read the config file
	config := Config{}
	if err := readJSONFromFile(configFilename, &config); err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}

	token := &oauth2.Token{}
	err = readJSONFromFile(tokenFilename, token)
	if err != nil {
		fmt.Println("Error reading token from file:", err)
		return
	}

	ctx := context.Background()

	client, err := getSpotifyClient(ctx, config, token)
	if err != nil {
		fmt.Println("Error getting Spotify client:", err)
		return
	}

	err = executeCommand(ctx, command, arg, client)
	if err != nil {
		fmt.Println(err)
		return
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

func getSpotifyArtistsNames(artists []spotify.SimpleArtist) string {
	var names []string
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	return strings.Join(names, ", ")
}

func parseArgs() (string, string, error) {
	if len(os.Args) < 3 {
		return "", "", fmt.Errorf("Usage: spotify.exe <command> <argument>")
	}

	return os.Args[1], os.Args[2], nil
}

func getSpotifyClient(ctx context.Context, config Config, token *oauth2.Token) (*spotify.Client, error) {
	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(spotifyauth.ScopePlaylistReadPrivate, spotifyauth.ScopeUserLibraryRead),
		spotifyauth.WithClientID(config.ClientID),
		spotifyauth.WithClientSecret(config.ClientSecret),
	)

	httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	client := spotify.New(httpClient)

	if token.Valid() {
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

		if err := writeJSONToFile(tokenFilename, token); err != nil {
			fmt.Println("Error saving token to file:", err)
			return nil, err
		}
	}

	return client, nil
}

func executeCommand(ctx context.Context, command string, arg string, client *spotify.Client) error {
	switch command {
	case "playlists":
		switch arg {
		case "download":
			if len(os.Args) < 4 {
				return fmt.Errorf("Usage: spotify.exe playlists download <playlist_name>")
			}
			playlistName := os.Args[3]
			err := downloadPlaylist(ctx, client, playlistName)
			if err != nil {
				return fmt.Errorf("Error downloading playlist: %v", err)
			}
			fmt.Println("Playlist downloaded successfully.")
		case "list":
			err := listPlaylists(ctx, client)
			if err != nil {
				return fmt.Errorf("Error listing playlists: %v", err)
			}
		case "download-all":
			err := downloadPlaylists(ctx, client)
			if err != nil {
				return fmt.Errorf("Error downloading playlists: %v", err)
			}
			fmt.Println("All playlists downloaded successfully.")
		case "create-clean-all":
			err := createCleanAllPlaylists()
			if err != nil {
				return fmt.Errorf("Error creating clean playlists: %v", err)
			}
			fmt.Println("All playlists cleaned successfully.")
		case "show-info":
			if len(os.Args) < 4 {
				return fmt.Errorf("Usage: spotify.exe playlists show-info <playlist_name>")
			}
			playlistName := os.Args[3]
			err := showPlaylistInfo(ctx, client, playlistName)
			if err != nil {
				return fmt.Errorf("Error showing playlist info: %v", err)
			}
		default:
			return fmt.Errorf("Invalid argument for 'playlists' command.")
		}
	default:
		return fmt.Errorf("Invalid command.")
	}
	return nil
}
