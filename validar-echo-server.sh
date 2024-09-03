#!/bin/bash

message="hello world"
SERVER_PORT=$(grep SERVER_PORT server/config.ini | cut -d ' ' -f 3)
SERVER_IP=$(grep SERVER_IP server/config.ini | cut -d ' ' -f 3)

response=$(docker run --rm --network tp0_testing_net busybox:latest sh -c "echo '$message' | nc $SERVER_IP $SERVER_PORT")

if [ "$response" = "$message" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
fi