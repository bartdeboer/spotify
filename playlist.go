package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/zmb3/spotify/v2"
)

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

func createCleanAllPlaylists() error {
	files, err := ioutil.ReadDir("downloads")
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

	data, err := ioutil.ReadFile(filePath)
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
	err = ioutil.WriteFile(cleanedFilePath, cleanedData, 0644)
	if err != nil {
		return err
	}

	return nil
}
