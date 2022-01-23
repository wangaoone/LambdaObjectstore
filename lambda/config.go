package main

import (
	"os"
	"time"
)

const (
	LIFESPAN = 5 * time.Minute
)

var (
<<<<<<< HEAD
	S3_COLLECTOR_BUCKET string = "mason-leap-lab.infinistore.collector"
	S3_BACKUP_BUCKET    string = "mason-leap-lab.infinistore.backup%s"
=======
	S3_COLLECTOR_BUCKET string = "tianium.default"
	S3_BACKUP_BUCKET    string = "tianium.infinicache%s"
>>>>>>> 5ff2a31d554049bca753b6bfdc96fce123104cce

	DRY_RUN = false
)

func init() {
	// Set required
	S3_COLLECTOR_BUCKET = GetenvIf(os.Getenv("S3_COLLECTOR_BUCKET"), S3_COLLECTOR_BUCKET)

	// Set required
	S3_BACKUP_BUCKET = GetenvIf(os.Getenv("S3_BACKUP_BUCKET"), S3_BACKUP_BUCKET)
}

func GetenvIf(env string, def string) string {
	if len(env) > 0 {
		return env
	} else {
		return def
	}
}
