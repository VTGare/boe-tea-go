# boe-tea-go

<img align="center" src="https://cdn.discordapp.com/avatars/636468907049353216/9bba642061fe0d500e92987098fdcf85.png?size=256">

**Boe Tea** is an artwork sharing bot for all your artwork-related needs.

## Getting started

[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)

To invite him please follow the link above. It requires following permissions to work correctly.

- Manage webhooks
- Read messages
- Send messages
- Manage messages
- Embed links
- Attach files
- Read Message History
- Add reactions
- Use external Emojis

If you ran into a problem or have a suggestion create an issue here, use bt!feedback command or contact me on Discord at _VTGare#3599_.

## Documentation

Please use `bt!help` command for documentation. Complete documentation is planned, but the progress is extremely slow.

## Privacy policy

[Please read the following](PRIVACY-POLICY.md)

## Contributing

[Please read the following](CONTRIBUTING.md)

## Deployment

### Requirements

- Go (1.27+). Download Golang from <https://golang.org> or by using a package manager (e.g. Chocolatey on Windows, homebrew on Mac or pacman on ArchLinux).

### Locally

1. Clone this repository. `git clone https://github.com/VTGare/boe-tea-go.git`
2. Change working directory to boe-tea-go. `cd boe-tea-go`
3. Download all required dependencies. `go mod download`
4. Build an executable file. `go build ./cmd/boetea`
5. Create a configuration file and fill it up. `touch config.json`

```json
{
    "discord": {
        "token": "Your Discord bot token. Acquire it on Discord Developer Portal.",
        "author_id": "Your Discord user ID. Gives access to developer commands.",
        "dev_guild_id": "Dev server ID for instant slash-command registration. Empty means global."
    },
    "mongo": {
        "uri": "mongodb://localhost:27017",
        "default_db": "boe-tea"
    },
    "pixiv": {
        "auth_token": "Pixiv auth token. Refer to https://gist.github.com/upbit/6edda27cb1644e94183291109b8a5fde to acquire.",
        "refresh_token": "Pixiv refresh token. Refer to https://gist.github.com/upbit/6edda27cb1644e94183291109b8a5fde to acquire.",
        "proxy_host": "Pixiv reverse proxy host, defaults to https://boetea.dev"
    },
    "repost": {
        "type": "Two options are supported: redis and memory.",
        "redis_uri": "Fill this in if repost type is redis."
    },
    "saucenao": "Sauce NAO API key, optional",
    "sentry": "Sentry API key, optional",
    "quotes": [
        {
            "content": "Embed footer message",
            "nsfw": false
        }
    ]
}
```

6. Run the executable file.

### Configuration from the environment

Every setting can also come from a `BOETEA_*` environment variable, which wins over `config.json`. A variable that's set but empty still wins, so `BOETEA_DISCORD_DEV_GUILD_ID=` turns a file's dev guild off. Without a config file, the environment alone is enough.

| Variable | `config.json` |
|---|---|
| `BOETEA_CONFIG` | path of the config file, default `config.json` |
| `BOETEA_DISCORD_TOKEN` | `discord.token` |
| `BOETEA_DISCORD_AUTHOR_ID` | `discord.author_id` |
| `BOETEA_DISCORD_DEV_GUILD_ID` | `discord.dev_guild_id` |
| `BOETEA_STORE_BACKEND` | `store.backend` (`mongo` or `postgres`) |
| `BOETEA_POSTGRES_DSN` | `store.postgres.dsn` |
| `BOETEA_MONGO_URI`, `BOETEA_MONGO_DATABASE` | `mongo.uri`, `mongo.default_db` |
| `BOETEA_REPOST_TYPE`, `BOETEA_REDIS_URI` | `repost.type`, `repost.redis_uri` |
| `BOETEA_PIXIV_AUTH_TOKEN`, `BOETEA_PIXIV_REFRESH_TOKEN`, `BOETEA_PIXIV_PROXY_HOST` | `pixiv.*` |
| `BOETEA_SAUCENAO_KEY` | `saucenao` |
| `BOETEA_SENTRY_DSN` | `sentry` |
| `BOETEA_MEDIA_MAX_CONCURRENT`, `BOETEA_MEDIA_SPOOL_DIR` | `media.*` |
| `BOETEA_PPROF_PORT` | `debug.pprof_port` |
| `BOETEA_QUOTES_FILE` | a JSON array of quotes added to `quotes`, default `quotes.json` |

### Docker

Every push to `master` publishes `ghcr.io/vtgare/boe-tea-go:latest` and `:<commit sha>`. The image has no config file and reads the environment. `compose.yaml` runs it with host networking and reads the variables from `.env`; `BOETEA_IMAGE_TAG` pins a tag.
