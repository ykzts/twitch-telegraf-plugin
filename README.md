# twitch-telegraf-plugin

Gather user information from [Twitch](https://www.twitch.tv/) users.

### Configuration

```toml
[[inputs.twitch]]
  ## List of users to monitor.
  users = [
    12826,
    133151915
  ]

  # client_id = ""
  # client_secret = ""
  # access_token = ""
```

### Metrics

- twitch_user
  - tags:
    - display_name - The display name
    - id - The ID of the user
    - login - The login name
  - fields:
    - videos (int)
    - followers (int)
    - following (int)
    - streams (int)
    - streams_total_viewers (int)
    - videos (int)
    - videos_total_viewers (int)

### Example Output

```plain
twitch_user,display_name=TwitchJP,id=133151915,login=twitchjp videos=172i,followers=46755i,following=4i,streams=0i,streams_total_viewers=0i,videos_total_viewers=8498459i 1656511107704078995
twitch_user,display_name=Twitch,id=12826,login=twitch videos=2294i,followers=2215319i,following=152i,streams=0i,streams_total_viewers=0i,videos_total_viewers=35262507i 1656511107704078995
```
