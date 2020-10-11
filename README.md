# boe-tea-go

[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)

<img align="center" src="https://cdn.discordapp.com/avatars/636468907049353216/9bba642061fe0d500e92987098fdcf85.png?size=256">

**Boe Tea** is a source image bot that reposts Pixiv, Twitter images, detects reposts and searches sauce for you!

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

Current documentation is out of date. All these commands still work as documented, but there are more undocumented commands or missed features on documented ones. Also their behaviour is subject to change soon. I'll make sure Boe Tea is fully documented when I finally release its own fansy website.

-   `bt!sauce` - tries to find original image source on SauceNAO search engine.
    -   _Usage_: `bt!sauce <optional link>`. If link is not present looks up for an attachment.
-   `bt!pixiv` - advanced pixiv command, let's you exclude certain pictures.
    -   _Usage_: `bt!pixiv <pixiv link> [excluded pictures]`
    -   _pixiv link_: Required link to a pixiv post.
    -   _excluded pictures_: Array of excluded pictures separated by space, supports range syntax (e.g. `1-7` excludes 1 through 7 inclusively)
    -   _Example_: `bt!pixiv https://www.pixiv.net/en/artworks/81893997 2-3` will repost only the first image of the set.
-   `bt!twitter` - reposts all images from a tweet, useful for mobile that doesn't support all images natively.
    -   _Usage_: `bt!twitter <required tweet link>`
-   `bt!deepfry` - deepfries an image for le epic memes.
    -   _Usage_: `bt!deepfry <image link> <times deepfried>`
    -   _image link_: optional if image is attached, link is prioritized if both are present.
    -   _times deepfried_: optional, fries even deeper, up to 5 times.
    -   _Example_: `bt!deepfry https://imgur.com/image.png 5`
-   `bt!help` - displays Boe Tea's command documentation
    -   _Advanced usage_: `bt!help <group name>` for the list of group's commands. `bt!help <group> <command>` for documentation of a specific command.
-   `bt!set` - displays current guild settings or changes them (e.g `bt!set prefix uwu`)
    -   _prefix_: bot's prefix. String up to 5 characters, if last character is a letter whitespace is assumed.
    -   _pixiv_ | _twitter_ | _crosspost_: switches corresponding feature server-wide. Accepted parameters: [_enabled, on, t,true_]  [_disabled, off, f, false_]
    -   _repost_: Configures repost checker behaviour. Accepts **enabled**, **disabled**, or **strict** parameters.
    -   _limit_: Album size limit. If album exceeds the limit only first image of every separate post will be send.
-   `bt!ping` - checks if Boe Tea online and sends its ping back
-   `bt!nhentai` - sends detailed information about an nhentai book.
    - _Usage_: `bt!nhentai <magic number>`
    - _magic number_: Typically, but not always, a 6-digit numbers only weebs can understand meaning of.

## Building

### Requirements
- Golang (1.13+)
- ffmpeg

Soon there will be a guide how build and deploy your own Boe Tea but until then you can go through the source code to figure it out yourself Kappa