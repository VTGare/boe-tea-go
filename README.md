# boe-tea-go

<img align="center" src="https://cdn.discordapp.com/avatars/636468907049353216/9bba642061fe0d500e92987098fdcf85.png?size=256">

**Boe Tea** is an artwork sharing bot for all your artwork-related needs.

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

## Privacy policy

By inviting the bot to your Discord server you accept to share the following information with the developer.

### The types of personal data Boe Tea logs temporarily (up to 2 weeks):
- Artwork URLs users post paired with guild/channel ID;
- Image URLs sent to all commands that require them (sauce, share);

These are stored for debugging purposes, if something unexpected goes wrong with a specific image or command,
the developer can easily track down the issue with a URL of an artwork user posted. Guild and channel IDs are required to apply
the exact guild configuration to the test environment.

- Artwork URL, message timestamp and URL are stored for every artwork to enable repost detection for
the server specific repost expiration period of time (from 1 minute to 1 week).

### The types of personal data Boe Tea stores forever:
- The artworks you favourited and when you favourited them;
- Channel IDs added to crosspost groups.

To opt-out from sharing any personal data that can be used to somehow track down your activity on Discord disable crossposting and
repost detection on your Discord server.

## Documentation
Please use `bt!help` command for documentation. Complete documentation is coming soon:tm:, maybe even this year.

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
                "content": "Embed footer message",
                "nsfw": false
            }
        ]
    }
}
```
6. Run the executable file.