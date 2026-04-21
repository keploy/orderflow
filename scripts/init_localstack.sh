#!/bin/bash
awslocal s3 mb s3://order-receipts
awslocal s3api put-bucket-acl --bucket order-receipts --acl public-read
echo "S3 bucket 'order-receipts' created successfully"
