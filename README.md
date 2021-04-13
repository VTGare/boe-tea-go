# boe-tea-go

[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FVTGare%2Fboe-tea-go.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FVTGare%2Fboe-tea-go?ref=badge_shield)

<img align="center" src="https://cdn.discordapp.com/avatars/636468907049353216/9bba642061fe0d500e92987098fdcf85.png?size=256">

**Boe Tea** is an ultimate artwork sharing bot for all your artwork-related needs.

## Getting started

[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)

To invite him please follow the link above. It requires following permissions to work correctly.
-   Manage webhooks
-   Read messages
-   Send messages
-   Manage messages
-   Embed links
-   Attach files
-   Read Message History
-   Add reactions
-   Use external Emojis

If you ran into a problem or have a suggestion create an issue here, use bt!feedback command or contact me on Discord at _VTGare#3599_.

## Documentation

Bot's documentation is currently getting rewritten and won't be publicly available until website release. Please use `bt!help` command instead.

## Deployment

### Requirements
- Golang (1.16+). Download Golang from https://golang.org or using any package manager for your platform (e.g. Chocolatey on Windows or pacman on ArchLinux).
- ffmpeg

### Locally
1. Clone this repository. `git clone https://github.com/VTGare/boe-tea-go.git`
2. Change working directory to boe-tea-go. `cd boe-tea-go`
3. Download all required dependencies. `go mod download`
4. Build an executable file. `go build ./cmd/boetea`
5. Create a configuration file and fill it up. `touch config.json`
```
{
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
        "refresh_token": "Pixiv refresh token. Refer to https://gist.github.com/upbit/6edda27cb1644e94183291109b8a5fde to acquire."
    },
    "quotes": [
        {
            "content": "Test",
            "nsfw": false
        }
    ]
}
}
```
6. Run the executable file.

### Heroku
TODO

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FVTGare%2Fboe-tea-go.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FVTGare%2Fboe-tea-go?ref=badge_large)