import sys

BASE_COMPOSE_FILE_PATH = 'compose-generator/docker-compose-base.yaml'

def main(output_file, num_clients):
    try:
        with open(BASE_COMPOSE_FILE_PATH, 'r') as f:
            lines = f.readlines()
    except IOError as e:
        print("Error: {}".format(e))
        sys.exit(e.errno)
    
    services_index = lines.index('services:\n')
    for i in range(1, num_clients + 1):
        lines.insert(services_index + 1, f'  client{i}:\n')
        lines.insert(services_index + 2, f'    container_name: client{i}\n')
        lines.insert(services_index + 3, '    image: client:latest\n')
        lines.insert(services_index + 4, '    entrypoint: /client\n')
        lines.insert(services_index + 5, f'    environment: [CLI_ID={i}, CLI_LOG_LEVEL=DEBUG]\n')
        lines.insert(services_index + 6, '    networks: [testing_net]\n')
        lines.insert(services_index + 7, '    depends_on: [server]\n')

    try:
        with open(output_file, 'w') as f:
            f.writelines(lines)
    except IOError as e:
        print(f"Error: {e}")
        sys.exit(e.errno)

if __name__ == '__main__':
    output_file = sys.argv[1]
    if not output_file.endswith('.yaml'):
        print("Error: Output file must end with .yaml")
        sys.exit(1)
    num_clients = int(sys.argv[2])
    main(output_file, num_clients)