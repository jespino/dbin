# dbin - Database Learning Environment

dbin (Database Interactive) is a command-line tool designed to help developers explore and learn different database systems and paradigms quickly and easily. It provides a hassle-free way to spin up various database environments using Docker containers.

## Features

- Quick setup of popular databases
- No installation required (except Docker)
- Consistent interface across different databases
- Ephemeral or persistent data storage
- Interactive CLI clients
- Web interfaces where available
- Debug mode for troubleshooting

## Supported Databases

- **Relational (SQL)**
  - [PostgreSQL](https://www.postgresql.org/)
  - [MySQL](https://www.mysql.com/)
  - [MariaDB](https://mariadb.org/)
  - [ClickHouse](https://clickhouse.com/)
  
- **Document (NoSQL)**
  - [MongoDB](https://www.mongodb.com/)
  - [CouchDB](https://couchdb.apache.org/)
  
- **Graph**
  - [Neo4j](https://neo4j.com/)
  - [Dgraph](https://dgraph.io/)
  
- **Search Engine**
  - [OpenSearch](https://opensearch.org/)
  - [Elasticsearch](https://www.elastic.co/elasticsearch/)
  
- **Wide Column**
  - [Apache HBase](https://hbase.apache.org/)
  - [Cassandra](https://cassandra.apache.org/)
  
- **Time Series**
  - [QuestDB](https://questdb.io/)
  - [Prometheus](https://prometheus.io/)
  
- **Key-Value**
  - [Redis](https://redis.io/)
  - [ValKey](https://valkey.io/)
  
- **Multi-Model**
  - [ArangoDB](https://www.arangodb.com/)
  - [OrientDB](https://orientdb.org/)
  - [SurrealDB](https://surrealdb.com/)
  - [RethinkDB](https://rethinkdb.com/)

## Prerequisites

- Docker installed and running
- Go 1.22 or later

## Installation

```bash
go install github.com/yourusername/dbin@latest
```

## Usage

### List available databases
```bash
dbin list
```

### Start a database
```bash
dbin postgres     # Start PostgreSQL
dbin mongodb      # Start MongoDB
dbin neo4j        # Start Neo4j
# etc...
```

### Options
- `--data-dir`: Specify a directory for persistent data storage
- `--debug`: Enable debug output for troubleshooting
```bash
dbin postgres --data-dir ./mydata --debug
```

### Cleanup
Remove all containers and networks created by dbin:
```bash
dbin cleanup
```

## Learning Path Suggestions

1. **Start with Relational Databases**
   - Try PostgreSQL or MariaDB
   - Practice SQL queries
   - Understand ACID properties

2. **Explore Document Stores**
   - Use MongoDB or CouchDB
   - Learn about schema-less design
   - Practice with JSON documents

3. **Graph Databases**
   - Experiment with Neo4j
   - Learn Cypher query language
   - Model connected data

4. **Time Series Data**
   - Use QuestDB
   - Understand time-based queries
   - Work with metrics and events

5. **Search Capabilities**
   - Try OpenSearch
   - Learn about full-text search
   - Understand indexing and scoring

## Contributing

Contributions are welcome! Here are some ways you can contribute:

- Add support for new databases
- Improve documentation
- Report bugs
- Suggest features
- Submit pull requests

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Docker for containerization
- All the amazing database projects
- The Go community

## Disclaimer

This tool is for learning and development purposes. For production environments, please refer to the official documentation of each database system.
