package database

import (
	"context"
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

func InsertOnePost(post *ImagePost) error {
	collection := DB.Collection("image_posts")
	_, err := collection.InsertOne(context.Background(), post)
	if err != nil {
		return err
	}

	return nil
}

func InsertManyPosts(posts []interface{}) error {
	collection := DB.Collection("image_posts")
	_, err := collection.InsertMany(context.Background(), posts)
	if err != nil {
		return err
	}

	return nil
}

func IsRepost(channelID, content string) (*ImagePost, error) {
	collection := DB.Collection("image_posts")
	res := collection.FindOne(context.Background(), bson.D{{"channel_id", channelID}, {"content", content}})

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
