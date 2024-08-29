#!/bin/bash

message="Hello world!"
server_port=$(grep SERVER_PORT server/config.ini | cut -d ' ' -f 3)
echo "server_port: $server_port"

response=$(docker run --rm --network tp0_testing_net busybox:latest /bin/sh -c "echo $message | nc server $server_port")

if [ "$response" == "$message" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
fi