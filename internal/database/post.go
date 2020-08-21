package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ImagePost struct {
	Author    string    `bson:"author" json:"author"`
	GuildID   string    `bson:"guild_id" json:"guild_id"`
	ChannelID string    `bson:"channel_id" json:"channel_id"`
	MessageID string    `bson:"message_id" json:"message_id"`
	Content   string    `bson:"content" json:"content"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

func NewImagePost(author, guildID, channelID, messageID, data string) *ImagePost {
	return &ImagePost{
		Author:    author,
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
		Content:   data,
		CreatedAt: time.Now(),
	}
}

func (d *Database) InsertOnePost(post *ImagePost) error {
	_, err := d.posts.InsertOne(context.Background(), post)
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) InsertManyPosts(posts []interface{}) error {
	_, err := d.posts.InsertMany(context.Background(), posts)
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) IsRepost(channelID, content string) (*ImagePost, error) {
	res := d.posts.FindOne(context.Background(), bson.D{{"channel_id", channelID}, {"content", content}})

	post := &ImagePost{}
	err := res.Decode(post)
	if err != nil && err != mongo.ErrNoDocuments {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return post, nil
}

//NewRepostDetection caches post info per channel.
func (d *Database) NewRepostDetection(author, guildID, channelID, messageID, post string) error {
	err := d.InsertOnePost(NewImagePost(author, guildID, channelID, messageID, post))
	if err != nil {
		return errRepostDetection(err)
	}
	return nil
}

func errRepostDetection(err error) error {
	return fmt.Errorf("Repost detection has failed. Please report this error to a dev and disable repost detection if problem remains.\n%v", err)
}
