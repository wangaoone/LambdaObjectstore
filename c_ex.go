package main

import (
	"github.com/mason-leap-lab/infinicache/client"
	//"math/rand"
	"strings"
	"fmt"
	"encoding/json"
	//"bytes"
	//"io/ioutil"
)

var addrList = "127.0.0.1:6378"

func main() {
	// initial object with random value
	//var val []byte
	//val = make([]byte, 1024)
	//rand.Read(val)

	val, _err := json.Marshal(5)

	if _err != nil {
		panic(_err)
	}

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	cli.EcSet("foo", val)
	//cli.EcGet("foo", len(val))
	rc, int_err := cli.Get("foo")

	if int_err == false {
		fmt.Println("Internal error!")
	}
	
	//var buf []byte
	//var err error
	//buf := new(bytes.Buffer)
	buf := make([]byte, 32)
	//b, err := ioutil.ReadAll(rc)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println(b)
	//if _, err := rc.Read(buf); err != nil {
	//	panic(err)
	//}
	rc.Read(buf)

	fmt.Println(buf)

	var x int
	json.Unmarshal(buf, &x)
	fmt.Println(x)
}
