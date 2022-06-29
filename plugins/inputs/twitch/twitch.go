package youtube

import (
	_ "embed"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/nicklaw5/helix/v2"
)

// DO NOT REMOVE THE NEXT TWO LINES! This is required to embed the sampleConfig data.
//go:embed sample.conf
var sampleConfig string

// Twitch - plugin main structure
type Twitch struct {
	AccessToken  string  `toml:"access_token"`
	ClientID     string  `toml:"client_id"`
	ClientSecret string  `toml:"client_secret"`
	Users        []int64 `toml:"users"`
	twitchClient *helix.Client
}

// Create Twitch Client
func (t *Twitch) createTwitchClient() (*helix.Client, error) {
	twitchClient, err := helix.NewClient(&helix.Options{
		ClientID:     t.ClientID,
		ClientSecret: t.ClientSecret,
	})

	if err != nil {
		return nil, err
	}

	if t.AccessToken != "" {
		twitchClient.SetUserAccessToken(t.AccessToken)
	} else {
		resp, err := twitchClient.RequestAppAccessToken(nil)

		if err != nil {
			return nil, err
		}

		twitchClient.SetAppAccessToken(resp.Data.AccessToken)
	}

	return twitchClient, nil
}

// SampleConfig returns sample configuration for this plugin.
func (t *Twitch) SampleConfig() string {
	return sampleConfig
}

// Description returns the plugin description.
func (t *Twitch) Description() string {
	return "Gather user information from Twitch users."
}

// Gather Twitch Metrics
func (t *Twitch) Gather(acc telegraf.Accumulator) error {
	if t.twitchClient == nil {
		twitchClient, err := t.createTwitchClient()
		if err != nil {
			return err
		}

		t.twitchClient = twitchClient
	}

	limit := 100
	count := int(math.Ceil(float64(len(t.Users)) / float64(limit)))

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < len(t.Users); i += limit {
		last := i + limit
		if last > len(t.Users) {
			last = len(t.Users)
		}

		go func(ids []int64, acc telegraf.Accumulator) {
			defer wg.Done()

			if err := t.gatherUsers(ids, acc); err != nil {
				acc.AddError(err)
			}
		}(t.Users[i:last], acc)
	}

	wg.Wait()

	return nil
}

func (t *Twitch) getUsers(ids []int64) ([]helix.User, error) {
	var userIDs []string
	for _, id := range ids {
		userIDs = append(userIDs, strconv.FormatInt(id, 10))
	}

	resp, err := t.twitchClient.GetUsers(&helix.UsersParams{
		IDs: userIDs,
	})
	if err != nil {
		return nil, err
	}

	return resp.Data.Users, nil
}

func (t *Twitch) getStreams(users []helix.User) ([]helix.Stream, error) {
	var ids []string

	for _, user := range users {
		ids = append(ids, user.ID)
	}

	var streams []helix.Stream
	var cursor string

	for {
		resp, err := t.twitchClient.GetStreams(&helix.StreamsParams{
			After:   cursor,
			First:   100,
			UserIDs: ids,
		})
		if err != nil {
			return nil, err
		}

		cursor = resp.Data.Pagination.Cursor
		streams = append(streams, resp.Data.Streams...)

		if cursor == "" {
			break
		}
	}

	return streams, nil
}

func (t *Twitch) gatherUsers(ids []int64, acc telegraf.Accumulator) error {
	users, err := t.getUsers(ids)
	if err != nil {
		return err
	}

	streams, err := t.getStreams(users)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(users))

	for _, user := range users {
		go func(user helix.User, streams []helix.Stream, acc telegraf.Accumulator) {
			defer wg.Done()

			if err := t.gatherUserStats(user, streams, acc); err != nil {
				acc.AddError(err)
			}
		}(user, streams, acc)
	}

	wg.Wait()

	return nil
}

func (t *Twitch) gatherUserStats(user helix.User, streams []helix.Stream, acc telegraf.Accumulator) error {
	now := time.Now()
	tags := getTags(user)

	followers, err := t.getFollowers(user)
	if err != nil {
		return err
	}

	following, err := t.getFollowing(user)
	if err != nil {
		return err
	}

	videos, err := t.getVideos(user)
	if err != nil {
		return err
	}

	videosTotalViewers := 0
	for _, video := range videos {
		videosTotalViewers = videosTotalViewers + video.ViewCount
	}

	var userStreams []helix.Stream
	for _, stream := range streams {
		if stream.UserID == user.ID {
			userStreams = append(userStreams, stream)
		}
	}

	streamsTotalViewers := 0
	for _, stream := range userStreams {
		streamsTotalViewers = streamsTotalViewers + stream.ViewerCount
	}

	fields := map[string]interface{}{
		"followers":             followers,
		"following":             following,
		"streams":               len(userStreams),
		"streams_total_viewers": streamsTotalViewers,
		"videos":                len(videos),
		"videos_total_viewers":  videosTotalViewers,
	}

	acc.AddFields("twitch_user", fields, tags, now)

	return nil
}

func (t *Twitch) getFollowers(user helix.User) (int, error) {
	resp, err := t.twitchClient.GetUsersFollows(&helix.UsersFollowsParams{
		ToID: user.ID,
	})
	if err != nil {
		return 0, err
	}

	return resp.Data.Total, nil
}

func (t *Twitch) getFollowing(user helix.User) (int, error) {
	resp, err := t.twitchClient.GetUsersFollows(&helix.UsersFollowsParams{
		FromID: user.ID,
	})
	if err != nil {
		return 0, err
	}

	return resp.Data.Total, nil
}

func (t *Twitch) getVideos(user helix.User) ([]helix.Video, error) {
	var videos []helix.Video
	var cursor string

	for {
		resp, err := t.twitchClient.GetVideos(&helix.VideosParams{
			After:  cursor,
			First:  100,
			UserID: user.ID,
		})
		if err != nil {
			return nil, err
		}

		cursor = resp.Data.Pagination.Cursor
		videos = append(videos, resp.Data.Videos...)

		if cursor == "" {
			break
		}
	}

	return videos, nil
}

func getTags(user helix.User) map[string]string {
	return map[string]string{
		"display_name": user.DisplayName,
		"id":           user.ID,
		"login":        user.Login,
	}
}

func getFields(user helix.User) map[string]interface{} {
	return map[string]interface{}{
		"followers": 0,
		"following": 0,
	}
}

func init() {
	inputs.Add("twitch", func() telegraf.Input {
		return &Twitch{}
	})
}
