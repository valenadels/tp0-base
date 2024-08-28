#!/bin/bash
outputFile=$1
clients=$2

if [ -z $outputFile ] || [ -z $clients ]; then
    echo "Error: Please provide the output file name and the number of clients"
    exit 1
fi

if ! [[ "$clients" =~ ^[0-9]+$ ]]; then
    echo "Error: clients should be an integer"
    exit 1
fi

echo "Output file: $1"
echo "N° clients: $2"

python3 compose-generator/docker-compose-generator.py $1 $2

if [ $? -ne 0 ]; then
    echo "Error: Python script execution failed"
    exit 1
fi

echo "Docker-compose file $1 generated successfully"
