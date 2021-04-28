package models

import (
	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/VTGare/boe-tea-go/pkg/models/users"
	"go.uber.org/zap"
)

type Models struct {
	Artworks artworks.Service
	Guilds   guilds.Service
	Users    users.Service
}

func New(db *mongodb.Mongo, logger *zap.SugaredLogger) *Models {
	return &Models{
		Artworks: artworks.NewService(db, logger),
		Guilds:   guilds.NewService(db, logger),
		Users:    users.NewService(db, logger),
	}
}
