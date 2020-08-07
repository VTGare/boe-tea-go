package utils

import (
	"fmt"

	"github.com/VTGare/boe-tea-go/internal/database"
	"go.mongodb.org/mongo-driver/mongo"
)

func errRepostDetection(err error) error {
	return fmt.Errorf("Repost detection has failed. Please report this error to a dev and disable repost detection if problem remains.\n%v", err)
}

//IsRepost checks if something has been cached in repost cache
func IsRepost(channelID, post string) (*database.ImagePost, error) {
	rep, err := database.IsRepost(channelID, post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errRepostDetection(err)
	}

	return rep, nil
}

//NewRepostDetection caches post info per channel.
func NewRepostDetection(author, guildID, channelID, messageID, post string) error {
	err := database.InsertOnePost(database.NewImagePost(author, guildID, channelID, messageID, post))
	if err != nil {
		return errRepostDetection(err)
	}
	return nil
}
