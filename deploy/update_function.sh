#!/bin/bash

BASE=`pwd`/`dirname $0`
PREFIX="CacheNode"
KEY="lambda"
cluster=100
mem=2048

if ![ -z "$2" ]; then 
    PREFIX="$2"
else 
    echo "No prefix argument supplied. Using default prefix 'CacheNode' instead."
fi 

S3="infinistore-storage-ben"

cd $BASE/../lambda
echo "Compiling lambda code..."
GOOS=linux go build
echo "Compressing file..."
zip $KEY $KEY
echo "Putting code zip to s3"
aws s3api put-object --bucket ${S3} --key $KEY.zip --body $KEY.zip

echo "Updating lambda functions.."
go run $BASE/deploy_function.go -S3 ${S3} -config -prefix=$PREFIX -vpc -key=$KEY -to=$cluster -mem=$mem -timeout=$1
rm $KEY*
