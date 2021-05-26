#!/bin/bash

BASE=`pwd`/`dirname $0`
PREFIX="CacheNode"
KEY="lambda"
start=0
cluster=100
mem=1024

S3="infinistore-storage-ben"
EMPH="\033[1;33m"
RESET="\033[0m"

PROPOSED_PREFIX=
if [ "$2" == "" ] ; then
    CODE=""
elif [ "$2" == "-code" ] ; then
    CODE="$2"
    PROPOSED_PREFIX="$3"
else
    CODE="$3"
    PROPOSED_PREFIX="$2"
fi

if [ -z $PROPOSED_PREFIX ]; then
    echo "No prefix argument supplied. Using default prefix \"$PREFIX\" instead."
else
    PREFIX=$PROPOSED_PREFIX
fi 

if [ "$CODE" == "-code" ] ; then
    echo -e "Updating "$EMPH"code and configuration"$RESET" of Lambda deployments ${PREFIX}${start} to ${PREFIX}$((start+cluster-1)) to $mem MB, $1s timeout..."
    #read -p "Press any key to confirm, or ctrl-C to stop."

    cd $BASE/../lambda
    echo "Compiling lambda code..."
    GOOS=linux go build
    echo "Compressing file..."
    zip $KEY $KEY
    echo "Putting code zip to s3"
    aws s3api put-object --bucket ${S3} --key $KEY.zip --body $KEY.zip
else 
    echo -e "Updating "$EMPH"configuration"$RESET" of Lambda deployments ${PREFIX}${start} to ${PREFIX}$((start+cluster)) to $mem MB, $1s timeout..."
    #read -p "Press any key to confirm, or ctrl-C to stop."
fi

echo "Updating Lambda deployments..."
go run $BASE/deploy_function.go -S3 ${S3} $CODE -config -prefix=$PREFIX -vpc -key=$KEY -from=$start -to=$((start+cluster)) -mem=$mem -timeout=$1

if [ "$CODE" == "-code" ] ; then
  rm $KEY*
fi