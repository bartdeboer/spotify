package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func downloadPlaylist(ctx context.Context, client *spotify.Client, playlistName string) error {
	_, playlists, err := getUserPlaylists(ctx, client)
	if err != nil {
		return err
	}

	var playlistID spotify.ID
	found := false
	for _, playlist := range playlists {
		if strings.EqualFold(playlist.Name, playlistName) {
			playlistID = playlist.ID
			found = true
			break
		}
	}

	if !found {
		return errors.New("playlist not found")
	}

	allTracks, err := fetchAllPlaylistTracks(ctx, client, playlistID)
	if err != nil {
		return err
	}

	playlistFile, err := os.Create(fmt.Sprintf("downloads/%s.json", playlistName))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := playlistFile.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%s; %s", err, closeErr)
			} else {
				err = closeErr
			}
		}
	}()

	encoder := json.NewEncoder(playlistFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(allTracks)
}

func displayPlaylist(ctx context.Context, client *spotify.Client, playlistName string) error {
	_, playlists, err := getUserPlaylists(ctx, client)
	if err != nil {
		return err
	}

	var playlistID spotify.ID
	found := false
	for _, playlist := range playlists {
		if strings.EqualFold(playlist.Name, playlistName) {
			playlistID = playlist.ID
			found = true
			break
		}
	}

	if !found {
		return errors.New("playlist not found")
	}

	allTracks, err := fetchAllPlaylistTracks(ctx, client, playlistID)
	if err != nil {
		return err
	}

	fmt.Println("Playlist:", playlistName)
	for _, track := range allTracks {
		fmt.Printf("- %s by %s\n", track.Track.Name, getSpotifyArtistsNames(track.Track.Artists))
	}

	return nil
}

func listPlaylists(ctx context.Context, client *spotify.Client) error {
	_, playlists, err := getUserPlaylists(ctx, client)
	if err != nil {
		return err
	}

	fmt.Println("Playlists:")
	for _, playlist := range playlists {
		fmt.Printf("- %s (ID: %s)\n", playlist.Name, playlist.ID)
	}

	return nil
}

func downloadPlaylists(ctx context.Context, client *spotify.Client) error {
	_, playlists, err := getUserPlaylists(ctx, client)
	if err != nil {
		return err
	}

	for _, playlist := range playlists {
		err := downloadPlaylist(ctx, client, playlist.Name)
		if err != nil {
			fmt.Println("Error downloading playlist:", err)
			return err
		}
		fmt.Println("Playlist downloaded successfully:", playlist.Name)
	}

	return nil
}

func createCleanAllPlaylists() error {
	files, err := os.ReadDir("downloads")
	if err != nil {
		return err
	}

	for _, file := range files {
		fileName := file.Name()
		if strings.HasSuffix(fileName, ".json") {
			err := cleanPlaylist(fileName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanPlaylist(fileName string) error {
	filePath := fmt.Sprintf("downloads/%s", fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var playlist []FullTrack
	err = json.Unmarshal(data, &playlist)
	if err != nil {
		return err
	}

	var cleanedPlaylist []CleanedPlaylistItem
	for _, track := range playlist {
		cleanedTrack := CleanedPlaylistItem{
			Track:        track.Track.Name,
			TrackNumber:  track.Track.TrackNumber,
			TrackArtists: getArtistsNames(track.Track.Artists),
			Album:        track.Track.Album.Name,
			AlbumArtists: getArtistsNames(track.Track.Album.Artists),
			ReleaseDate:  track.Track.Album.ReleaseDate,
		}
		cleanedPlaylist = append(cleanedPlaylist, cleanedTrack)
	}

	cleanedData, err := json.MarshalIndent(cleanedPlaylist, "", "  ")
	if err != nil {
		return err
	}

	cleanedFilePath := fmt.Sprintf("cleaned/%s", fileName)
	err = os.WriteFile(cleanedFilePath, cleanedData, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getUserPlaylists(ctx context.Context, client *spotify.Client) (*spotify.PrivateUser, []spotify.SimplePlaylist, error) {
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return &spotify.PrivateUser{}, nil, err
	}

	playlists, err := client.GetPlaylistsForUser(ctx, user.ID)
	if err != nil {
		return &spotify.PrivateUser{}, nil, err
	}

	return user, playlists.Playlists, nil
}

const (
	trackFetchLimit = 100
	initialOffset   = 0
)

func fetchAllPlaylistTracks(ctx context.Context, client *spotify.Client, playlistID spotify.ID) ([]spotify.PlaylistTrack, error) {
	var allTracks []spotify.PlaylistTrack
	offset := initialOffset

	for {
		playlistTracks, err := client.GetPlaylistTracks(ctx, playlistID, spotify.Limit(trackFetchLimit), spotify.Offset(offset))
		if err != nil {
			return nil, err
		}

		allTracks = append(allTracks, playlistTracks.Tracks...)

		if len(allTracks) >= playlistTracks.Total {
			break
		}

		offset += trackFetchLimit
	}

	return allTracks, nil
}

func showPlaylistInfo(ctx context.Context, client *spotify.Client, playlistName string) error {
	_, playlists, err := getUserPlaylists(ctx, client)
	if err != nil {
		return err
	}

	var playlistID spotify.ID
	found := false
	for _, playlist := range playlists {
		if strings.EqualFold(playlist.Name, playlistName) {
			playlistID = playlist.ID
			found = true
			break
		}
	}

	if !found {
		return errors.New("playlist not found")
	}

	allTracks, err := fetchAllPlaylistTracks(ctx, client, playlistID)
	if err != nil {
		return err
	}

	fmt.Println("Playlist:", playlistName)
	for _, track := range allTracks {
		fmt.Printf("- %s by %s\n", track.Track.Name, getSpotifyArtistsNames(track.Track.Artists))
	}

	return nil
}
