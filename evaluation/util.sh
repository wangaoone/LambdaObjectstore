#!/bin/bash

PWD=`dirname $0`
REDBENCH=$GOPATH/src/github.com/wangaoone/redbench

echo $PWD

function update_lambda_timeout() {
    NAME=$1
    TIME=$2
    echo "updating lambda store timeout"
    for i in {0..63}
    do
        aws lambda update-function-configuration --function-name $NAME$i --timeout $TIME
    done
}

function update_lambda_mem() {
    NAME=$1
    MEM=$2
    echo "updating lambda store mem"
    for i in {0..63}
    do
            aws lambda update-function-configuration --function-name $NAME$i --memory-size $MEM
    done
}


function start_proxy() {
    echo "running proxy server"
    PREFIX=$1
    GOMAXPROCS=36 go run $PWD/../src/lambdaproxy.go -isPrint=true -prefix=$PREFIX
}

function bench() {
    N=$1
    C=$2
    KEYMIN=$3
    KEYMAX=$4
    SZ=$5
    D=$6
    P=$7
    OP=$8
    FILE=$9
    go run $REDBENCH/bench.go -addrlist localhost:6378 -n $N -c $C -keymin $KEYMIN -keymax $KEYMAX \
    -sz $SZ -d $D -p $P -op $OP -file $FILE -dec -i 1000
}

function updateTimeOut() {
    NAME=$1
    TIME=$2
    echo "updating lambda store timeout"
    go run $PWD/../../sbin/deploy_function.go -config=true -prefix=$NAME -timeout=$1

}

function updateMem() {
    NAME=$1
    MEM=$2
    echo "updating lambda store mem"
    go run $PWD/../../sbin/deploy_function.go -config=true -prefix=$NAME -mem=$MEM
}