deploy:
	aws configure
	aws s3api put-bucket-policy --bucket ao.cloudwatch --policy file://$(GOPATH)/src/github.com/wangaoone/LambdaObjectstore/evaluation/cloudwatch/policy.json

microbench:
	./slap.sh 1>./log 2>&1 &

build-simulator:
	go build $(GOPATH)/src/github.com/wangaoone/redbench/simulator/playback/
	mv playback $(GOPATH)/src/github.com/wangaoone/redbench/simulator/playback/

dryrun: build-simulator
	./playback.sh ~/project/src/github.com/wangaoone/LambdaObjectstore/evaluation/dal09_10mb_50h.csv 400 10 2 0.1 -compact dryrun

simulate: build-simulator
	./playback.sh ~/project/src/github.com/wangaoone/LambdaObjectstore/evaluation/dal09_10mb_50h.csv 400 10 2 0.1 -compact 1>./log 2>&1 &

playback: build-simulator
	./playback.sh ~/project/src/github.com/wangaoone/LambdaObjectstore/evaluation/dal09_10mb_50h.csv 400 10 2 0.1 1>./log 2>&1 &

start-server:
	./server.sh 1>./log 2>&1 &

stop-server:
	kill -2 $(shell cat /tmp/lambdaproxy.pid)

benchmark:
	go run $(GOPATH)/src/github.com/wangaoone/redbench/bench.go -n 50 -c 1 -keymin 1 -keymax 50 -sz 2097152 -d 1 -p 1 -op 0 -i 10000
#	go run $(GOPATH)/src/github.com/wangaoone/redbench/bench.go -n 10 -c 1 -keymin 1 -keymax 10 -sz 2097152 -d 1 -p 1 -op 1 -i 2000
