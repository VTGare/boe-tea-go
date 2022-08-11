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

## Privacy policy

The following entails the privacy policy and agreement that you accept when adding any of Boe Tea to a server, or as a member of such a server.

### Essential data collecting

This data is collected automatically. It is used for the bot to function correctly or to troubleshoot bugs that may occur in runtime.

- Server configuration
- Links to images that you want to find source of (stored temporarily)
- Links to artworks shared in art channels via commands or with automatic embedding (stored temporarily)

### Optional data collecting

This data is collected optionally when certain bot user enables or uses certain features.

- Links to artworks shared in art channels with enabled repost detection (stored in RAM from 1 day up to 2 weeks, configurable)
- Internal Boe Tea IDs of artworks you bookmarked
- Discord channel IDs in crosspost groups

### Can I request data deletion?

Most data mentioned above (only data stored in RAM is an exception) can be permanently removed upon your request, that includes temporary stored logged data in a given timeframe. Please use `bt!feedback` command to send me a message.

### Data storage

All stored data is kept on protected servers and it's kept on a password secured cloud storage (MongoDB Atlas). Please keep in mind that even with these protections, no data can ever be 100% secure. All efforts are taken to keep your data secure and private, but its absolute security cannot be guaranteed.

### Agreement

By adding Boe Tea to your server you are consenting to the policies outlined in this document. If you, the server manager, do not agree to this document, you may remove the bot(s) from the server. If you, the server member, do not agree to this document, you may leave the server that contains the bot(s).

## Documentation

Please use `bt!help` command for documentation. Complete documentation is coming soon:tm:, maybe even this year.

## Deployment

### Requirements

- Go (1.18+). Download Golang from <https://golang.org> or by using a package manager (e.g. Chocolatey on Windows, homebrew on Mac or pacman on ArchLinux).

### Locally

1. Clone this repository. `git clone https://github.com/VTGare/boe-tea-go.git`
2. Change working directory to boe-tea-go. `cd boe-tea-go`
3. Download all required dependencies. `go mod download`
4. Build an executable file. `go build ./cmd/boetea`
5. Create a configuration file and fill it up. `touch config.json`

```json
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
