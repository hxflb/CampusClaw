package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"campusclaw/internal/config"
	"campusclaw/internal/embedder"
	"campusclaw/internal/indexx"
	"campusclaw/internal/qdrant"
	"campusclaw/internal/server"
	"campusclaw/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := store.Init(context.Background(), db, cfg); err != nil {
		log.Fatal(err)
	}
	qd := qdrant.New(cfg.QdrantURL, cfg.VectorSize)
	var last error
	for i := 0; i < 40; i++ {
		if last = qd.Ready(); last == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if last != nil {
		log.Fatal("qdrant: ", last)
	}
	rebuilt, err := qd.Ensure()
	if err != nil {
		log.Fatal(err)
	}
	emb := embedder.New(cfg)
	idx := indexx.New(db, cfg, emb, qd)
	if rebuilt {
		log.Println("vector size changed, rewriting embeddings")
		if err := idx.Reembed(context.Background()); err != nil {
			log.Println("reembed:", err)
		}
	}
	if err := idx.Backfill(context.Background()); err != nil {
		log.Println("backfill:", err)
	}
	srv := server.New(cfg, db, idx)
	log.Printf("listen %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Routes()); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
