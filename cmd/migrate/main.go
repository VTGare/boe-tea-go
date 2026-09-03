package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	pgstore "github.com/VTGare/boe-tea-go/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type report struct {
	Guilds     int64 `json:"guilds"`
	Users      int64 `json:"users"`
	Artworks   int64 `json:"artworks"`
	Bookmarks  int64 `json:"bookmarks"`
	DestGuilds int64 `json:"dest_guilds,omitempty"`

	DestUsers     int64 `json:"dest_users,omitempty"`
	DestArtworks  int64 `json:"dest_artworks,omitempty"`
	DestBookmarks int64 `json:"dest_bookmarks,omitempty"`
	FavDiff       int64 `json:"favourites_diff,omitempty"`

	Sequence int64 `json:"sequence,omitempty"`
}

func mongoFromConfig(path string) (string, string, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var cfg struct {
		Mongo struct {
			URI      string `json:"uri"`
			Database string `json:"default_db"`
		} `json:"mongo"`
	}

	if err := json.Unmarshal(file, &cfg); err != nil {
		return "", "", err
	}

	if cfg.Mongo.URI == "" {
		return "", "", fmt.Errorf("no mongo.uri in %s", path)
	}

	return cfg.Mongo.URI, cfg.Mongo.Database, nil
}

func main() {
	mongoURI := flag.String("mongo-uri", os.Getenv("MONGO_URI"), "mongo uri")
	mongoDB := flag.String("mongo-db", os.Getenv("MONGO_DB"), "mongo database")
	mongoCfg := flag.String("mongo-config", "", "bot config.json path to read mongo.uri/default_db from")
	pgDSN := flag.String("postgres-dsn", os.Getenv("POSTGRES_DSN"), "postgres dsn")
	dryRun := flag.Bool("dry-run", false, "report only, no writes")
	asJSON := flag.Bool("json", false, "json report output")
	batch := flag.Int("batch", 1000, "batch size")

	flag.Parse()

	if *mongoCfg != "" {
		uri, db, err := mongoFromConfig(*mongoCfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)

			os.Exit(2)
		}

		if *mongoURI == "" {
			*mongoURI = uri
		}

		if *mongoDB == "" {
			*mongoDB = db
		}
	}

	if *mongoURI == "" || *mongoDB == "" || *pgDSN == "" {
		fmt.Fprintln(os.Stderr, "mongo-uri, mongo-db and postgres-dsn are required")

		os.Exit(2)
	}

	ctx := context.Background()

	if err := run(ctx, *mongoURI, *mongoDB, *pgDSN, *dryRun, *asJSON, *batch); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)

		os.Exit(1)
	}
}

func run(ctx context.Context, mongoURI, mongoDB, pgDSN string, dryRun, asJSON bool, batch int) error {
	pgInit, err := pgstore.New(ctx, pgDSN)
	if err != nil {
		return err
	}

	if err := pgInit.Init(ctx); err != nil {
		return err
	}

	if err := pgInit.Close(ctx); err != nil {
		return err
	}

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	defer func() { _ = mongoClient.Disconnect(ctx) }()

	db := mongoClient.Database(mongoDB)

	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	rep, err := buildReport(ctx, db, pool)
	if err != nil {
		return err
	}

	if dryRun {
		return printReport(rep, asJSON, 1)
	}

	if err := loadGuilds(ctx, db, pool, batch); err != nil {
		return err
	}

	if err := loadUsers(ctx, db, pool, batch); err != nil {
		return err
	}

	if err := loadArtworks(ctx, db, pool, batch); err != nil {
		return err
	}

	if err := loadBookmarks(ctx, db, pool, batch); err != nil {
		return err
	}

	if err := recomputeFavourites(ctx, pool); err != nil {
		return err
	}

	final, err := buildReport(ctx, db, pool)
	if err != nil {
		return err
	}

	if final.FavDiff != 0 {
		_ = printReport(final, asJSON, 0)

		return fmt.Errorf("favourites diff = %d, refusing to pass", final.FavDiff)
	}

	if final.Guilds != final.DestGuilds || final.Users != final.DestUsers ||
		final.Artworks != final.DestArtworks || final.Bookmarks != final.DestBookmarks {
		_ = printReport(final, asJSON, 0)

		return fmt.Errorf("count mismatch after load")
	}

	return printReport(final, asJSON, 0)
}

func buildReport(ctx context.Context, db *mongo.Database, pool *pgxpool.Pool) (*report, error) {
	rep := &report{}

	counts := map[string]*int64{
		"guilds": &rep.Guilds, "users": &rep.Users, "artworks": &rep.Artworks, "bookmarks": &rep.Bookmarks,
	}

	for col, ptr := range counts {
		n, err := db.Collection(col).CountDocuments(ctx, bson.M{})
		if err != nil {
			return nil, err
		}

		*ptr = n
	}

	queries := map[string]*int64{
		"SELECT count(*) FROM guilds": &rep.DestGuilds, "SELECT count(*) FROM users": &rep.DestUsers,
		"SELECT count(*) FROM artworks": &rep.DestArtworks, "SELECT count(*) FROM bookmarks": &rep.DestBookmarks,
	}

	for q, ptr := range queries {
		if err := pool.QueryRow(ctx, q).Scan(ptr); err != nil {
			return nil, err
		}
	}

	if err := pool.QueryRow(ctx, `SELECT count(*) FROM artworks a WHERE a.favourites != (
		SELECT count(*) FROM bookmarks b WHERE b.artwork_id = a.id
	)`).Scan(&rep.FavDiff); err != nil {
		return nil, err
	}

	_ = pool.QueryRow(ctx, `SELECT last_value FROM artwork_id_seq`).Scan(&rep.Sequence)

	return rep, nil
}

func printReport(rep *report, asJSON bool, _ int) error {
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(rep)
	}

	fmt.Printf("mongo: guilds=%d users=%d artworks=%d bookmarks=%d\n", rep.Guilds, rep.Users, rep.Artworks, rep.Bookmarks)
	fmt.Printf("pg:    guilds=%d users=%d artworks=%d bookmarks=%d favDiff=%d seq=%d\n",
		rep.DestGuilds, rep.DestUsers, rep.DestArtworks, rep.DestBookmarks, rep.FavDiff, rep.Sequence)

	return nil
}

func loadGuilds(ctx context.Context, db *mongo.Database, pool *pgxpool.Pool, batch int) error {
	cur, err := db.Collection("guilds").Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer func() { _ = cur.Close(ctx) }()

	buf := make([]*store.Guild, 0, batch)

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}

		for _, guild := range buf {
			if guild.ArtChannels == nil {
				guild.ArtChannels = make([]string, 0)
			}

			_, err := pool.Exec(ctx, `INSERT INTO guilds (id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
				crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
				ON CONFLICT (id) DO UPDATE SET prefix=EXCLUDED.prefix, pixiv=EXCLUDED.pixiv, twitter=EXCLUDED.twitter,
				deviant=EXCLUDED.deviant, bluesky=EXCLUDED.bluesky, tags=EXCLUDED.tags, flavour_text=EXCLUDED.flavour_text,
				crosspost=EXCLUDED.crosspost, reactions=EXCLUDED.reactions, skip_first=EXCLUDED.skip_first, "limit"=EXCLUDED."limit",
				repost=EXCLUDED.repost, repost_expiration=EXCLUDED.repost_expiration, art_channels=EXCLUDED.art_channels,
				nsfw=EXCLUDED.nsfw, updated_at=EXCLUDED.updated_at`,
				guild.ID, guild.Prefix, guild.Pixiv, guild.Twitter, guild.Deviant, guild.Bluesky, guild.Tags, guild.FlavorText,
				guild.Crosspost, guild.Reactions, guild.SkipFirst, guild.Limit, string(guild.Repost),
				int64(guild.RepostExpiration), guild.ArtChannels, guild.NSFW, guild.CreatedAt, guild.UpdatedAt,
			)
			if err != nil {
				return err
			}
		}

		buf = buf[:0]

		return nil
	}

	for cur.Next(ctx) {
		var guild store.Guild
		if err := cur.Decode(&guild); err != nil {
			return err
		}

		buf = append(buf, &guild)

		if len(buf) >= batch {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := cur.Err(); err != nil {
		return err
	}

	return flush()
}

func loadUsers(ctx context.Context, db *mongo.Database, pool *pgxpool.Pool, batch int) error {
	cur, err := db.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer func() { _ = cur.Close(ctx) }()

	count := 0

	for cur.Next(ctx) {
		var user store.User
		if err := cur.Decode(&user); err != nil {
			return err
		}

		_, err = pool.Exec(ctx, `INSERT INTO users (id, dm, crosspost, ignore, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO UPDATE SET dm=EXCLUDED.dm, crosspost=EXCLUDED.crosspost, ignore=EXCLUDED.ignore, updated_at=EXCLUDED.updated_at`,
			user.ID, user.DM, user.Crosspost, user.Ignore, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return err
		}

		if _, err := pool.Exec(ctx, `DELETE FROM user_groups WHERE user_id=$1`, user.ID); err != nil {
			return err
		}

		for _, grp := range user.Groups {
			children := grp.Children
			if children == nil {
				children = make([]string, 0)
			}

			if _, err := pool.Exec(ctx, `INSERT INTO user_groups (user_id, name, parent, is_pair, children)
				VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id, name) DO UPDATE SET parent=EXCLUDED.parent,
				is_pair=EXCLUDED.is_pair, children=EXCLUDED.children`,
				user.ID, grp.Name, grp.Parent, grp.IsPair, children,
			); err != nil {
				return err
			}
		}

		count++

		if count%batch == 0 {
			fmt.Fprintf(os.Stderr, "users: %d\r", count)
		}
	}

	return cur.Err()
}

func loadArtworks(ctx context.Context, db *mongo.Database, pool *pgxpool.Pool, batch int) error {
	opts := options.Find().SetSort(bson.M{"artwork_id": 1})
	cur, err := db.Collection("artworks").Find(ctx, bson.M{}, opts)
	if err != nil {
		return err
	}
	defer func() { _ = cur.Close(ctx) }()

	var maxID int

	count := 0

	for cur.Next(ctx) {
		var art store.Artwork
		if err := cur.Decode(&art); err != nil {
			return err
		}

		images := art.Images
		if images == nil {
			images = make([]string, 0)
		}

		_, err = pool.Exec(ctx, `INSERT INTO artworks (id, title, author, url, images, favourites, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, author=EXCLUDED.author, url=EXCLUDED.url,
			images=EXCLUDED.images, favourites=EXCLUDED.favourites, updated_at=EXCLUDED.updated_at`,
			art.ID, art.Title, art.Author, art.URL, images, art.Favorites, art.CreatedAt, art.UpdatedAt,
		)
		if err != nil {
			return err
		}

		if art.ID > maxID {
			maxID = art.ID
		}

		count++

		if count%batch == 0 {
			fmt.Fprintf(os.Stderr, "artworks: %d (maxID %d)\r", count, maxID)
		}
	}

	if err := cur.Err(); err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `SELECT setval('artwork_id_seq', $1, true)`, maxID+1)

	return err
}

func loadBookmarks(ctx context.Context, db *mongo.Database, pool *pgxpool.Pool, batch int) error {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":        bson.M{"user_id": "$user_id", "artwork_id": "$artwork_id"},
			"created_at": bson.M{"$min": "$created_at"},
			"nsfw":       bson.M{"$first": "$nsfw"},
		}}},
		{{Key: "$sort", Value: bson.M{"_id.user_id": 1, "_id.artwork_id": 1}}},
	}

	cur, err := db.Collection("bookmarks").Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer func() { _ = cur.Close(ctx) }()

	type deduped struct {
		ID struct {
			UserID    string `bson:"user_id"`
			ArtworkID int    `bson:"artwork_id"`
		} `bson:"_id"`
		CreatedAt time.Time `bson:"created_at"`
		NSFW      bool      `bson:"nsfw"`
	}

	count := 0

	for cur.Next(ctx) {
		var d deduped
		if err := cur.Decode(&d); err != nil {
			return err
		}

		_, err = pool.Exec(ctx, `INSERT INTO bookmarks (user_id, artwork_id, nsfw, created_at)
			VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, artwork_id) DO NOTHING`,
			d.ID.UserID, d.ID.ArtworkID, d.NSFW, d.CreatedAt,
		)
		if err != nil {
			return err
		}

		count++

		if count%batch == 0 {
			fmt.Fprintf(os.Stderr, "bookmarks: %d\r", count)
		}
	}

	return cur.Err()
}

func recomputeFavourites(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `UPDATE artworks a SET favourites = COALESCE((
		SELECT count(*) FROM bookmarks b WHERE b.artwork_id = a.id
	), 0)`)

	return err
}
