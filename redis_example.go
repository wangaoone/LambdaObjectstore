package main

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/tidwall/redbench"
)

func main() {
	redbench.Bench("PING", "127.0.0.1:6379", nil, nil, func(buf []byte) []byte {
		return redbench.AppendCommand(buf, "PING")
	})
	redbench.Bench("SET", "127.0.0.1:6379", nil, nil, func(buf []byte) []byte {
		return redbench.AppendCommand(buf, "SET", "key:string", "val")
	})
	redbench.Bench("GET", "127.0.0.1:6379", nil, nil, func(buf []byte) []byte {
		return redbench.AppendCommand(buf, "GET", "key:string")
	})
	rand.Seed(time.Now().UnixNano())
	redbench.Bench("GEOADD", "127.0.0.1:6379", nil, nil, func(buf []byte) []byte {
		return redbench.AppendCommand(buf, "GEOADD", "key:geo",
			strconv.FormatFloat(rand.Float64()*360-180, 'f', 7, 64),
			strconv.FormatFloat(rand.Float64()*170-85, 'f', 7, 64),
			strconv.Itoa(rand.Int()))
	})
	redbench.Bench("GEORADIUS", "127.0.0.1:6379", nil, nil, func(buf []byte) []byte {
		return redbench.AppendCommand(buf, "GEORADIUS", "key:geo",
			strconv.FormatFloat(rand.Float64()*360-180, 'f', 7, 64),
			strconv.FormatFloat(rand.Float64()*170-85, 'f', 7, 64),
			"10", "km")
	})
}
