package models

import (
	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/models/artworks"
	"github.com/VTGare/boe-tea-go/models/guilds"
	"github.com/VTGare/boe-tea-go/models/users"
	"go.uber.org/zap"
)

type Models struct {
	DB       *mongodb.Mongo
	Artworks artworks.Service
	Guilds   guilds.Service
	Users    users.Service
}

func New(db *mongodb.Mongo, logger *zap.SugaredLogger) *Models {
	return &Models{
		DB:       db,
		Artworks: artworks.NewService(db, logger),
		Guilds:   guilds.NewService(db, logger),
		Users:    users.NewService(db, logger),
	}
}
