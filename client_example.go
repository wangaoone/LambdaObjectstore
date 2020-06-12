package main

import (
	"github.com/mason-leap-lab/infinicache/client"
	//"math/rand"
	//"io/ioutil"
	"strings"
	"fmt"
	"encoding/json"
	//"io/ioutil"
	//"bytes"
	//"os"
)

var addrList = "127.0.0.1:6378"

func main() {
	// initial object with random value
	//var val []byte
	//val = make([]byte, 1024)
	//rand.Read(val)

	//fmt.Println(val)

	marshalled_result, err := json.Marshal(5)
	if err != nil {
		panic(err)
	}

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	cli.EcSet("foo", marshalled_result)
	_, reader, ok := cli.EcGet("foo", 0)

	if ok == false {
		panic("Internal error!")
	}
	
	buf, err := reader.ReadAll()
	fmt.Println("buf:", buf)

	if err != nil {
		panic(err)
	}

	fmt.Println("Unmarshalling now...")
	var v int64 
	json.Unmarshal([]byte(buf), &v)
	fmt.Println(v)
}
