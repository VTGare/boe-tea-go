# boe-tea-go
[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discordapp.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537250880&scope=bot)

<img align="center" src="https://cdn.discordapp.com/avatars/636468907049353216/f22aa4bf930d9743dd40a10287de8b04.png?size=256">

**Boe Tea** is a source image bot that makes it easier to post your favourite anime art (only Pixiv as of right now) on Discord and find sauce without leaving your Discord window.

## Getting started
[![Invite](https://img.shields.io/badge/Invite%20Link-%40Boe%20Tea-brightgreen)](https://discordapp.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537250880&scope=bot)

To invite him please follow the link above. It requires following permissions to work correctly now and in future *(more fuctionality to come!)*
* Manage webhooks (for future functionality)
* Read messages
* Send messages
* Embed links
* Attach files
* Read Message History (for future functionality)
* Add reactions
* Use external Emojis

If you run into a problem or have a suggestion create an issue here or contact me on Discord at *VTGare#3370*.

## Documentation
Currently Boe Tea possesses a limited amount of features. All commands can be described on a single page, what I'm about to prove right here.
* ``bt!sauce`` - tries to find original image source on SauceNAO or ASCII2D search engine.
    * *Usage*: ``bt!sauce <optional reverse search engine> <optional link>``. If reverse search engine is not present uses guild's default. If link is not present looks up for an attachment.
* ``bt!pixiv`` - advanced pixiv command, let's you exclude certain pictures.
   * *Usage*: ``bt!pixiv <pixiv link> [excluded pictures]
   * *pixiv link*: Required link to a pixiv post.
   * *excluded pictures*: Array of excluded pictures separated by space, supports range syntax (e.g. ``1-7`` excludes 1 through 7 inclusively)
   * *Example*: ``bt!pixiv https://www.pixiv.net/en/artworks/81893997 2-3`` will repost only the first image of the set.
* ``bt!twitter`` - reposts all images from a tweet, useful for mobile that doesn't support all images natively. 
   * *Usage*: ``bt!twitter <required tweet link>``
* ``bt!deepfry`` - deepfries an image for le epic memes.
   * *Usage*: ``bt!deepfry <image link> <times deepfried>
   * *image link*: optional if image is attached, link is prioritized if both are present.
   * *times deepfried*: optional, fries even deeper, up to 5 times.
   * *Example*: ``bt!deepfry https://imgur.com/image.png 5``
* ``bt!help`` - displays Boe Tea's command documentation
* ``bt!set`` - displays current guild settings or changes them (e.g ``bt!set prefix uwu``)
    * *prefix*: prefix to invoke Boe Tea's commands. String up to 5 characters, if last character is a letter whitespace is assumed.
    * *largeset*: amount of images considered a large set that procs a confirmation prompt.
    * *pixiv*: boolean value, switches reposting functionality. Accepts *t, true, f, false*.
    * *reversesearch*: default reverse image search engine. Accepts **SauceNAO** or **ASCII2D**.
    * *repost*: Default reposting behaviour. Accepts **links**, **embeds**, and **ask** options.
    * *limit*: Hard limit to album size. If you attempt to repost a post/bulk-repost posts with more images it'll fail.
    * *promptemoji*: Custom prompt emoji. Only local guild and standard emojis are supported as of now.
* ``bt!ping`` - checks if Boe Tea online and sends its ping back

## Additional features
* NSFW check. If Pixiv post has an R-18 tag it's only allowed to be reposted in NSFW channels.
* Ability to delete an image after it was posted (react ❌ ``:x:`` to Boe Tea's message)