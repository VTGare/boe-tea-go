package pixiv

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/pkg/artworks"
	models "github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/bwmarrin/discordgo"
	"github.com/everpcpc/pixiv"
)

type Pixiv struct {
	app   *pixiv.AppPixivAPI
	cache *ttlcache.Cache
}

type Artwork struct {
}

func New(authToken, refreshToken string) (artworks.Provider, error) {
	_, err := pixiv.LoadAuth(authToken, refreshToken, time.Now())
	if err != nil {
		return nil, err
	}

	cache := ttlcache.NewCache()
	cache.SetTTL(30 * time.Minute)

	return &Pixiv{pixiv.NewApp(), cache}, nil
}

func (p Pixiv) Match(s string) (string, bool) {
	return "", false
}

func (p Pixiv) Find(id string) (artworks.Artwork, error) {
	return &Artwork{}, nil
}

func (a Artwork) ToModel() *models.ArtworkInsert {
	return &models.ArtworkInsert{}
}

func (a Artwork) Embeds(quote string) []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{}
}
