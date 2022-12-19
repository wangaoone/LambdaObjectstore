package main

import (
	"os"
	"time"
)

const (
	LIFESPAN = 5 * time.Minute
)

var (
	// Bucket to store experiment data. No date will be stored if InputEvent.Prefix is not set.
	S3_COLLECTOR_BUCKET string = "jzhang33.default"
	// Bucket to store persistent data. Keep "%s" at the end of the bucket name.
	S3_BACKUP_BUCKET string = "jzhang33.infinicache%s"

	DRY_RUN = true
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
