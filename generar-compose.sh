#!/bin/bash

function validate_input() {
    if [ -z $output_file ] || [ -z $clients ]; then
        echo "Error: Please provide the output file name and the number of clients"
        exit 1
    fi

    if ! [[ "$clients" =~ ^[0-9]+$ ]]; then
        echo "Error: clients should be an integer"
        exit 1
    fi

    if [[ $output_file != *yaml ]]; then
        echo "Error: Output file must end with .yaml"
        exit 1
    fi
}

function write_server_config() {
    echo "name: tp0" > $output_file
    echo "services:" >> $output_file
    echo "  server:" >> $output_file
    echo "    container_name: server" >> $output_file
    echo "    image: server:latest" >> $output_file
    echo "    entrypoint: python3 /main.py" >> $output_file
    echo "    environment:" >> $output_file
    echo "      - PYTHONUNBUFFERED=1" >> $output_file
    echo "    networks:" >> $output_file
    echo "      - testing_net" >> $output_file
    echo "    volumes:" >> $output_file
    echo "      - ./server/config.ini:/server/config.ini" >> $output_file
}

function write_client_config() {
    for ((i=1; i<$clients+1; i++))
    do
        echo "  client$i:" >> $output_file
        echo "    container_name: client$i" >> $output_file
        echo "    image: client:latest" >> $output_file
        echo "    entrypoint: /client" >> $output_file
        echo "    environment:" >> $output_file
        echo "      - CLI_ID=$i" >> $output_file
        echo "    networks:" >> $output_file
        echo "      - testing_net" >> $output_file
        echo "    volumes:" >> $output_file
        echo "      - ./client/config.yaml:/config.yaml" >> $output_file
        echo "    depends_on:" >> $output_file
        echo "      - server" >> $output_file
    done
}

function write_network_config() {
    echo "networks:" >> $output_file
    echo "  testing_net:" >> $output_file
    echo "    ipam:" >> $output_file
    echo "      driver: default" >> $output_file
    echo "      config:" >> $output_file
    echo "        - subnet: 172.25.125.0/24" >> $output_file
}

output_file=$1
clients=$2

validate_input
write_server_config
write_client_config
write_network_config

echo "Docker-compose file $1 generated successfully"