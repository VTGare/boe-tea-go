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

- Go (1.21+). Download Golang from <https://golang.org> or by using a package manager (e.g. Chocolatey on Windows, homebrew on Mac or pacman on ArchLinux).

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
        "author_id": "Your Discord user ID. Gives access to developer commands."
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
